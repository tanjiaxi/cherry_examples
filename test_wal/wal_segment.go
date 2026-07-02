package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SegmentInfo WAL分片信息
type SegmentInfo struct {
	ID       int    `json:"id"`
	StartSeq int64  `json:"start_seq"`
	EndSeq   int64  `json:"end_seq"` // -1 表示当前活跃分片
	Synced   bool   `json:"synced"`
	FilePath string `json:"file_path"`
}

// SyncProgress 同步进度
type SyncProgress struct {
	LastSyncedSegment int   `json:"last_synced_segment"`
	LastSyncedSeq     int64 `json:"last_synced_sequence"`
}

// WALMetadata WAL元数据
type WALMetadata struct {
	CurrentSegmentID int           `json:"current_segment_id"`
	Segments         []SegmentInfo `json:"segments"`
	SyncProgress     SyncProgress  `json:"sync_progress"`
}

// WALSegmentManager WAL分片管理器
type WALSegmentManager struct {
	baseDir          string
	maxSegmentSize   int64         // 单个分片最大大小（字节）
	maxSegmentTime   time.Duration // 单个分片最大时间
	currentSegment   *WALWriter
	currentSegmentID int
	currentStartSeq  int64
	segmentStartTime time.Time
	metadata         *WALMetadata
	mu               sync.Mutex
}

// NewWALSegmentManager 创建WAL分片管理器
func NewWALSegmentManager(baseDir string, maxSegmentSize int64, maxSegmentTime time.Duration) (*WALSegmentManager, error) {
	// 确保目录存在
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, err
	}

	manager := &WALSegmentManager{
		baseDir:        baseDir,
		maxSegmentSize: maxSegmentSize,
		maxSegmentTime: maxSegmentTime,
		metadata: &WALMetadata{
			CurrentSegmentID: 0,
			Segments:         make([]SegmentInfo, 0),
			SyncProgress: SyncProgress{
				LastSyncedSegment: 0,
				LastSyncedSeq:     0,
			},
		},
	}

	// 尝试加载现有元数据
	if err := manager.loadMetadata(); err != nil {
		// 如果加载失败，创建新的第一个分片
		if err := manager.rotateSegment(1); err != nil {
			return nil, err
		}
	} else {
		// 恢复现有分片
		if err := manager.resumeCurrentSegment(); err != nil {
			return nil, err
		}
	}

	return manager, nil
}

// loadMetadata 加载元数据
func (m *WALSegmentManager) loadMetadata() error {
	metadataPath := filepath.Join(m.baseDir, "metadata.json")
	data, err := os.ReadFile(metadataPath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, m.metadata); err != nil {
		return err
	}

	return nil
}

// saveMetadata 保存元数据
func (m *WALSegmentManager) saveMetadata() error {
	metadataPath := filepath.Join(m.baseDir, "metadata.json")
	data, err := json.MarshalIndent(m.metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(metadataPath, data, 0644)
}

// resumeCurrentSegment 恢复当前分片
func (m *WALSegmentManager) resumeCurrentSegment() error {
	if len(m.metadata.Segments) == 0 {
		return m.rotateSegment(1)
	}

	// 找到最后一个分片
	lastSegment := &m.metadata.Segments[len(m.metadata.Segments)-1]

	// 如果最后一个分片已经关闭（EndSeq > 0），创建新分片
	if lastSegment.EndSeq > 0 {
		fmt.Printf("[WAL Segment] 恢复: 上个分片已关闭，创建新分片 (起始序列: %d)\n", lastSegment.EndSeq+1)
		return m.rotateSegment(lastSegment.EndSeq + 1)
	}

	// 否则，关闭旧分片，创建新分片继续（避免文件句柄冲突）
	// 读取旧文件获取最后序列号
	segmentPath := filepath.Join(m.baseDir, fmt.Sprintf("wal_%04d.log", lastSegment.ID))
	nextSeq := lastSegment.StartSeq

	if records, err := readSegmentRecordsFromPath(segmentPath); err == nil && len(records) > 0 {
		lastRecord := records[len(records)-1]
		nextSeq = lastRecord.Sequence + 1
		lastSegment.EndSeq = lastRecord.Sequence
	}

	fmt.Printf("[WAL Segment] 恢复: 关闭旧分片，创建新分片 (起始序列: %d)\n", nextSeq)
	return m.rotateSegment(nextSeq)
}

// readSegmentRecordsFromPath 从指定路径读取分片记录（辅助函数）
func readSegmentRecordsFromPath(filepath string) ([]SpinRecord, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []SpinRecord
	buffer := make([]byte, 40)

	for {
		n, err := file.Read(buffer)
		if err != nil || n == 0 {
			break
		}
		if n < 40 {
			break
		}

		record := SpinRecord{
			UserID:    int64(binary.LittleEndian.Uint64(buffer[0:8])),
			BetAmount: int64(binary.LittleEndian.Uint64(buffer[8:16])),
			WinAmount: int64(binary.LittleEndian.Uint64(buffer[16:24])),
			Sequence:  int64(binary.LittleEndian.Uint64(buffer[24:32])),
			Timestamp: int64(binary.LittleEndian.Uint64(buffer[32:40])),
		}
		records = append(records, record)
	}

	return records, nil
}

// rotateSegment 切换到新分片
func (m *WALSegmentManager) rotateSegment(startSeq int64) error {
	// 关闭当前分片
	if m.currentSegment != nil {
		if err := m.currentSegment.Close(); err != nil {
			return err
		}

		// 更新旧分片的结束序列号
		if len(m.metadata.Segments) > 0 {
			lastSegment := &m.metadata.Segments[len(m.metadata.Segments)-1]
			lastSegment.EndSeq = startSeq - 1
		}
	}

	// 创建新分片
	m.currentSegmentID++
	segmentPath := filepath.Join(m.baseDir, fmt.Sprintf("wal_%04d.log", m.currentSegmentID))

	writer, err := NewWALWriter(segmentPath, 4096)
	if err != nil {
		return err
	}

	m.currentSegment = writer
	m.currentStartSeq = startSeq
	m.segmentStartTime = time.Now()

	// 更新元数据
	m.metadata.CurrentSegmentID = m.currentSegmentID
	m.metadata.Segments = append(m.metadata.Segments, SegmentInfo{
		ID:       m.currentSegmentID,
		StartSeq: startSeq,
		EndSeq:   -1, // 活跃分片
		Synced:   false,
		FilePath: segmentPath,
	})

	if err := m.saveMetadata(); err != nil {
		return err
	}

	fmt.Printf("[WAL Segment] 创建新分片: %s (起始序列: %d)\n", segmentPath, startSeq)
	return nil
}

// WriteRecord 写入记录（自动分片）
func (m *WALSegmentManager) WriteRecord(record SpinRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否需要切换分片
	if m.shouldRotate() {
		if err := m.rotateSegment(record.Sequence); err != nil {
			return err
		}
	}

	// 写入当前分片
	return m.currentSegment.WriteRecord(record)
}

// shouldRotate 判断是否需要切换分片
func (m *WALSegmentManager) shouldRotate() bool {
	if m.currentSegment == nil {
		return true
	}

	// 检查文件大小
	fileInfo, err := os.Stat(m.currentSegment.file.Name())
	if err == nil && fileInfo.Size() >= m.maxSegmentSize {
		return true
	}

	// 检查时间
	if time.Since(m.segmentStartTime) >= m.maxSegmentTime {
		return true
	}

	return false
}

// GetUnsyncedSegments 获取未同步的分片列表
func (m *WALSegmentManager) GetUnsyncedSegments() []SegmentInfo {
	m.mu.Lock()
	defer m.mu.Unlock()

	unsynced := make([]SegmentInfo, 0)
	for _, segment := range m.metadata.Segments {
		if !segment.Synced && segment.EndSeq > 0 { // 只返回已关闭的分片
			unsynced = append(unsynced, segment)
		}
	}
	return unsynced
}

// MarkSegmentSynced 标记分片已同步
func (m *WALSegmentManager) MarkSegmentSynced(segmentID int, lastSeq int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.metadata.Segments {
		if m.metadata.Segments[i].ID == segmentID {
			m.metadata.Segments[i].Synced = true
			break
		}
	}

	// 更新同步进度
	m.metadata.SyncProgress.LastSyncedSegment = segmentID
	m.metadata.SyncProgress.LastSyncedSeq = lastSeq

	return m.saveMetadata()
}

// ReadSegmentRecords 读取指定分片的所有记录
func (m *WALSegmentManager) ReadSegmentRecords(segmentInfo SegmentInfo) ([]SpinRecord, error) {
	file, err := os.Open(segmentInfo.FilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var records []SpinRecord
	buffer := make([]byte, 40) // 每条记录40字节

	for {
		n, err := file.Read(buffer)
		if err != nil || n == 0 {
			break
		}
		if n < 40 {
			break // 不完整的记录
		}

		record := SpinRecord{
			UserID:    int64(binary.LittleEndian.Uint64(buffer[0:8])),
			BetAmount: int64(binary.LittleEndian.Uint64(buffer[8:16])),
			WinAmount: int64(binary.LittleEndian.Uint64(buffer[16:24])),
			Sequence:  int64(binary.LittleEndian.Uint64(buffer[24:32])),
			Timestamp: int64(binary.LittleEndian.Uint64(buffer[32:40])),
		}
		records = append(records, record)
	}

	return records, nil
}

// Close 关闭管理器
func (m *WALSegmentManager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.currentSegment != nil {
		if err := m.currentSegment.Close(); err != nil {
			return err
		}
	}

	return m.saveMetadata()
}

// GetMetadata 获取元数据（只读）
func (m *WALSegmentManager) GetMetadata() WALMetadata {
	m.mu.Lock()
	defer m.mu.Unlock()
	return *m.metadata
}
