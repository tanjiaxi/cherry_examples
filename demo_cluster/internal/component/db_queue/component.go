package dbqueue

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
)

// OpType 数据库操作类型定义
type OpType int

const (
	OpInsert OpType = 1
	OpUpdate OpType = 2
	OpDelete OpType = 3
)

// 通用队列数据任务 DbWriteTask
type DbWriteTask struct {
	Table      string      // 目标表名
	ExtraKeyId string      // 额外的key 比如roomId
	PlayerID   int32       // 玩家Id
	OpType     OpType      // 操作类型
	Data       interface{} // 玩家数据深拷贝的
}

// BatchSaver 具体的数据库批量写入执行者（解耦 GORM/MongoDB 等）
type BatchSaver interface {
	SaveBatch(table string, tasks []*DbWriteTask) error
}
type TableConfig struct {
	QueueCount    int           // 该表开启的队列(Worker)数量
	QueueSize     int           // 单个队列的 Channel 缓冲大小
	BulkSize      int           // 单次批量写入的最大条数（去重后）
	FlushInterval time.Duration // 强制刷入数据库的时间间隔
}

func DefaultTableConfig() *TableConfig {
	return &TableConfig{
		QueueCount:    1,
		QueueSize:     1024,
		BulkSize:      100,
		FlushInterval: 2 * time.Second,
	}
}

type DBWriteQueueComponent struct {
	cfacade.Component
	name       string
	config     map[string]TableConfig // 表配置
	saver      PersistenceBackend     // 具体的底层数据库保存实现
	workers    map[string][]*worker   // 运行中的 workers: table -> []*worker
	wg         sync.WaitGroup
	ctx        context.Context
	cancelFunc context.CancelFunc
	running    bool
	mu         sync.RWMutex
}

func NewDBWriteQueueComponent(config map[string]TableConfig, saver PersistenceBackend) *DBWriteQueueComponent {
	ctx, cancel := context.WithCancel(context.Background())
	if saver == nil {
		clog.Panicf(fmt.Sprintf("[%s] Init failed: BatchSaver is nil", "db_write_queue"))
	}
	return &DBWriteQueueComponent{
		config:     config,
		workers:    make(map[string][]*worker),
		ctx:        ctx,
		cancelFunc: cancel,
	}
}

func (d *DBWriteQueueComponent) Name() string {
	return "db_write_queue"
}

func (d *DBWriteQueueComponent) Init() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return
	}

	d.running = true
	for table, tCfg := range d.config {
		for i := 0; i < tCfg.QueueCount; i++ {
			w := &worker{
				queue:         make(chan *DbWriteTask, tCfg.QueueSize),
				table:         table,
				bulkSize:      tCfg.BulkSize,
				flushInterval: tCfg.FlushInterval,
				batchMap:      make(map[string]*DbWriteTask),
			}
			d.workers[table] = append(d.workers[table], w)
			d.wg.Add(1)
			go func(w *worker) {
				defer d.wg.Done()
				w.run(d.ctx, d.saver)
			}(w)
		}
		clog.Infof("[%s] Init table [%s] with %d queues successfully", d.Name(), table, tCfg.QueueCount)
	}
	clog.Infof("[%s] component init successfully", d.Name())
}

func (d *DBWriteQueueComponent) OnAfterInit() {
}

func (d *DBWriteQueueComponent) OnBeforeStop() {
}

func (d *DBWriteQueueComponent) OnStop() {
	d.mu.Lock()
	d.running = false
	defer d.mu.Unlock()
	clog.Infof("[%s] Stopping, closing channels and flushing tasks...", d.Name())
	// 1. 停止接受新任务并关闭所有 worker 的 channel
	d.mu.RLock()
	for _, workers := range d.workers {
		for _, w := range workers {
			close(w.queue) // 关闭通道，通知 worker 消费完剩余积压
		}
	}
	d.mu.RUnlock()

	// 2. 通知 context 结束（如果是阻塞等待的 timer 也会被快速释放）
	d.cancelFunc()
	d.wg.Wait() // 等待所有 worker 处理完积压的任务
	clog.Infof("[%s] Stopped, all tasks flushed to DB safely.", d.Name())
}

func (d *DBWriteQueueComponent) SubmitTask(task *DbWriteTask) bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if !d.running {
		clog.Warnf("[%s] SubmitTask failed: component is stopped. task: %+v", d.Name(), task)
		return false
	}
	if _, ok := d.workers[task.Table]; !ok {
		clog.Errorf("[%s] SubmitTask failed: table [%s] queue not registered", d.Name(), task.Table)
		return false
	}
	workerIndex := task.PlayerID % int32(len(d.workers[task.Table]))
	select {
	case d.workers[task.Table][workerIndex].queue <- task:
		return true
	default:
		// 当队列爆满时的降级机制（生产环境可根据业务需求选择：阻塞/丢弃/落日志告警）,或者重启扩大队列
		clog.Errorf("[%s] Queue is full for table [%s], player_id: %d", d.Name(), task.Table, task.PlayerID)
		return false
	}
}

type worker struct {
	queue         chan *DbWriteTask
	table         string
	bulkSize      int
	flushInterval time.Duration
	batchMap      map[string]*DbWriteTask // 消费端用来合并去重的内存 Map
}

func (w *worker) run(ctx context.Context, saver PersistenceBackend) {
	timer := time.NewTimer(w.flushInterval)
	defer timer.Stop()

	for {
		select {
		case task, ok := <-w.queue:
			if !ok {
				w.flush(ctx, saver)
				clog.Infof("dbqueue channel closed")
				return
			}
			key := w.getHashKey(w.table, strconv.FormatInt(int64(task.PlayerID), 10), task.ExtraKeyId)
			w.batchMap[key] = task

			if len(w.batchMap) >= w.bulkSize {
				w.flush(ctx, saver)
			}
		case <-ctx.Done():
			// 强制刷入剩余数据
			w.flush(ctx, saver)
			clog.Infof("dbqueue worker stoped")
			return
		case <-timer.C:
			// 定时刷入数据
			w.flush(ctx, saver)
			timer.Reset(w.flushInterval)
		}
	}
}

func (w *worker) flush(ctx context.Context, saver PersistenceBackend) {
	if len(w.batchMap) == 0 {
		return
	}
	tasks := make([]*DbWriteTask, 0, len(w.batchMap))
	for _, task := range w.batchMap {
		tasks = append(tasks, task)
	}

	if len(tasks) > 0 {
		err := saver.BatchSave(ctx, tasks)
		if err != nil {
			clog.Errorf("save batch failed. err = %v", err)
		}
	}

	// 清空map
	for k := range w.batchMap {
		delete(w.batchMap, k)
	}
}

func (w *worker) getHashKey(tableName string, key ...string) string {
	return strings.Join(append([]string{tableName}, key...), ":")
}
