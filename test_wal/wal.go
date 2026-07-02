package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"sync"
	"time"
)

// SpinRecord 代表单次老虎机转动产生的资产变动事件
type SpinRecord struct {
	UserID    int64
	BetAmount int64 // 消耗的金币（下注）
	WinAmount int64 // 赢取的金币
	Sequence  int64 // 严格递增的版本号/序号
	Timestamp int64
}

// WALWriter WAL文件写入器
type WALWriter struct {
	file    *os.File
	mu      sync.Mutex
	bufSize int
	buf     []byte
	bufPos  int
}

// NewWALWriter 创建WAL写入器
func NewWALWriter(filepath string, bufferSize int) (*WALWriter, error) {
	file, err := os.OpenFile(filepath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, err
	}

	return &WALWriter{
		file:    file,
		bufSize: bufferSize,
		buf:     make([]byte, bufferSize),
		bufPos:  0,
	}, nil
}

// WriteRecord 写入单条记录（顺序写入）
func (w *WALWriter) WriteRecord(record SpinRecord) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 序列化记录为二进制格式 (8字节 * 5个字段 = 40字节)
	data := make([]byte, 40)
	binary.LittleEndian.PutUint64(data[0:8], uint64(record.UserID))
	binary.LittleEndian.PutUint64(data[8:16], uint64(record.BetAmount))
	binary.LittleEndian.PutUint64(data[16:24], uint64(record.WinAmount))
	binary.LittleEndian.PutUint64(data[24:32], uint64(record.Sequence))
	binary.LittleEndian.PutUint64(data[32:40], uint64(record.Timestamp))

	// 如果缓冲区满了，先刷盘
	if w.bufPos+len(data) > w.bufSize {
		if err := w.flushBuffer(); err != nil {
			return err
		}
	}

	// 写入缓冲区
	copy(w.buf[w.bufPos:], data)
	w.bufPos += len(data)

	return nil
}

// flushBuffer 刷新缓冲区到磁盘
func (w *WALWriter) flushBuffer() error {
	if w.bufPos == 0 {
		return nil
	}

	_, err := w.file.Write(w.buf[:w.bufPos])
	if err != nil {
		return err
	}

	// 强制刷盘（fsync）
	if err := w.file.Sync(); err != nil {
		return err
	}

	w.bufPos = 0
	return nil
}

// Close 关闭WAL文件
func (w *WALWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// 刷新剩余数据
	if err := w.flushBuffer(); err != nil {
		return err
	}

	return w.file.Close()
}

type SlotsWalletSync struct {
	recordChan chan SpinRecord // 用于暂存高频变动的本地缓冲区
	batchSize  int             // 触发批量提交的数量阈值
	flushTimer *time.Ticker    // 触发批量提交的时间阈值
	stopChan   chan struct{}
	walWriter  *WALWriter     // WAL写入器
	wg         sync.WaitGroup // 追踪后台 worker 状态
}

func NewSlotsWalletSync(bufferSize int, batchSize int, flushInterval time.Duration, walPath string) (*SlotsWalletSync, error) {
	var walWriter *WALWriter
	var err error

	if walPath != "" {
		walWriter, err = NewWALWriter(walPath, 4096) // 4KB缓冲区
		if err != nil {
			return nil, err
		}
	}

	return &SlotsWalletSync{
		recordChan: make(chan SpinRecord, bufferSize),
		batchSize:  batchSize,
		flushTimer: time.NewTicker(flushInterval),
		stopChan:   make(chan struct{}),
		walWriter:  walWriter,
	}, nil
}

// SubmitSpinRecord 核心对局主线程调用此方法，完全是非阻塞的，性能极高
func (s *SlotsWalletSync) SubmitSpinRecord(record SpinRecord) error {
	// 1. 先同步写入本地 WAL 文件（顺序写入，性能极高）
	if s.walWriter != nil {
		if err := s.walWriter.WriteRecord(record); err != nil {
			return err
		}
	}

	// 2. 然后丢入内存管道，交给后台异步合并刷盘
	s.recordChan <- record
	return nil
}

// StartWorker 启动后台刷盘消费者
func (s *SlotsWalletSync) StartWorker() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		// 临时存放当前批次的切片，预分配内存
		batch := make([]SpinRecord, 0, s.batchSize)

		for {
			select {
			case record, ok := <-s.recordChan:
				if !ok {
					// channel 已关闭，处理剩余数据后退出
					if len(batch) > 0 {
						s.flush(batch)
					}
					return
				}
				batch = append(batch, record)
				// 达到数量阈值，立刻刷盘
				if len(batch) >= s.batchSize {
					s.flush(batch)
					batch = make([]SpinRecord, 0, s.batchSize) // 重新分配
				}

			case <-s.flushTimer.C:
				// 即使数量没满，到了定时器时间（如每隔 1 秒），也必须强制刷盘
				if len(batch) > 0 {
					s.flush(batch)
					batch = make([]SpinRecord, 0, s.batchSize)
				}

			case <-s.stopChan:
				// 优雅关闭：处理完剩下的最后一批数据
				if len(batch) > 0 {
					s.flush(batch)
				}
				// 把管道里残留的所有数据清空刷盘
				for record := range s.recordChan {
					batch = append(batch, record)
					if len(batch) >= s.batchSize {
						s.flush(batch)
						batch = make([]SpinRecord, 0, s.batchSize)
					}
				}
				if len(batch) > 0 {
					s.flush(batch)
				}
				return
			}
		}
	}()
}

// flush 真正执行批量网络 RPC 或者打包写入 Redis
func (s *SlotsWalletSync) flush(batch []SpinRecord) {
	// 在这里，原本 10,000 次的 Spin 独立变动，被聚合成了一条批量请求发送给资产微服务
	fmt.Printf("[WAL Sync] 正在批量提交 %d 条流水到资产微服务/中心数据库...\n", len(batch))
	// err := rpcClient.BatchUpdateWallet(batch)
	// if err == nil {
	//     // 提交成功后，可以异步地在本地 WAL 中标记这些 Sequence 已经安全落库
	//     markWALAsSynced(batch)
	// }
}

func (s *SlotsWalletSync) Close() error {
	// 1. 停止 timer
	s.flushTimer.Stop()

	// 2. 发送停止信号
	close(s.stopChan)

	// 3. 关闭 recordChan，让 worker 能退出 range 循环
	close(s.recordChan)

	// 4. 等待 worker 完全退出
	s.wg.Wait()

	// 5. 关闭 WAL 文件
	if s.walWriter != nil {
		return s.walWriter.Close()
	}
	return nil
}
