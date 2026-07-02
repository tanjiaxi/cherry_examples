package main

import (
	"fmt"
	"sync"
	"time"
)

// DBSyncer 数据库同步器
type DBSyncer struct {
	walManager    *WALSegmentManager
	stateManager  *PlayerStateManager
	syncInterval  time.Duration
	syncDelay     time.Duration // 模拟网络延迟
	stopChan      chan struct{}
	wg            sync.WaitGroup
	isRunning     bool
	mu            sync.Mutex
}

// NewDBSyncer 创建数据库同步器
func NewDBSyncer(walManager *WALSegmentManager, stateManager *PlayerStateManager, syncInterval, syncDelay time.Duration) *DBSyncer {
	return &DBSyncer{
		walManager:   walManager,
		stateManager: stateManager,
		syncInterval: syncInterval,
		syncDelay:    syncDelay,
		stopChan:     make(chan struct{}),
		isRunning:    false,
	}
}

// Start 启动同步器
func (s *DBSyncer) Start() {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = true
	s.mu.Unlock()

	s.wg.Add(1)
	go s.syncLoop()
}

// syncLoop 同步循环
func (s *DBSyncer) syncLoop() {
	defer s.wg.Done()

	ticker := time.NewTicker(s.syncInterval)
	defer ticker.Stop()

	fmt.Println("[DB Syncer] 同步器已启动")

	for {
		select {
		case <-ticker.C:
			if err := s.syncOnce(); err != nil {
				fmt.Printf("[DB Syncer] 同步失败: %v\n", err)
			}
		case <-s.stopChan:
			fmt.Println("[DB Syncer] 收到停止信号，执行最后一次同步...")
			s.syncOnce() // 最后一次同步
			fmt.Println("[DB Syncer] 同步器已停止")
			return
		}
	}
}

// syncOnce 执行一次同步
func (s *DBSyncer) syncOnce() error {
	// 获取未同步的分片
	unsyncedSegments := s.walManager.GetUnsyncedSegments()
	if len(unsyncedSegments) == 0 {
		return nil
	}

	fmt.Printf("[DB Syncer] 发现 %d 个未同步分片，开始同步...\n", len(unsyncedSegments))

	for _, segment := range unsyncedSegments {
		if err := s.syncSegment(segment); err != nil {
			return fmt.Errorf("同步分片 %d 失败: %w", segment.ID, err)
		}
	}

	return nil
}

// syncSegment 同步单个分片
func (s *DBSyncer) syncSegment(segment SegmentInfo) error {
	startTime := time.Now()
	fmt.Printf("[DB Syncer] 开始同步分片 %d (seq: %d-%d)...\n", segment.ID, segment.StartSeq, segment.EndSeq)

	// 读取分片中的所有记录
	records, err := s.walManager.ReadSegmentRecords(segment)
	if err != nil {
		return err
	}

	if len(records) == 0 {
		fmt.Printf("[DB Syncer] 分片 %d 无数据，标记为已同步\n", segment.ID)
		return s.walManager.MarkSegmentSynced(segment.ID, segment.EndSeq)
	}

	// 模拟批量同步到数据库（故意放慢速度）
	batchSize := 50
	totalBatches := (len(records) + batchSize - 1) / batchSize

	for i := 0; i < len(records); i += batchSize {
		end := i + batchSize
		if end > len(records) {
			end = len(records)
		}

		batch := records[i:end]
		batchNum := i/batchSize + 1

		// 模拟网络延迟
		time.Sleep(s.syncDelay)

		// 模拟数据库批量写入
		if err := s.mockDatabaseBatchInsert(batch); err != nil {
			return err
		}

		fmt.Printf("[DB Syncer] 分片 %d: 批次 %d/%d 已同步 (%d条记录)\n", 
			segment.ID, batchNum, totalBatches, len(batch))
	}

	// 标记分片已同步
	lastSeq := records[len(records)-1].Sequence
	if err := s.walManager.MarkSegmentSynced(segment.ID, lastSeq); err != nil {
		return err
	}

	elapsed := time.Since(startTime)
	fmt.Printf("[DB Syncer] 分片 %d 同步完成，共 %d 条记录，耗时: %v\n", 
		segment.ID, len(records), elapsed)

	return nil
}

// mockDatabaseBatchInsert 模拟数据库批量插入
func (s *DBSyncer) mockDatabaseBatchInsert(records []SpinRecord) error {
	// 在实际应用中，这里会执行真实的数据库操作
	// 例如: INSERT INTO spin_records (user_id, bet_amount, win_amount, sequence, timestamp) VALUES ...
	
	// 这里只是打印日志模拟
	for _, record := range records {
		// 可以在这里添加数据验证逻辑
		if record.UserID <= 0 {
			return fmt.Errorf("无效的用户ID: %d", record.UserID)
		}
	}

	// 模拟数据库写入延迟
	// time.Sleep(10 * time.Millisecond)

	return nil
}

// Stop 停止同步器
func (s *DBSyncer) Stop() {
	s.mu.Lock()
	if !s.isRunning {
		s.mu.Unlock()
		return
	}
	s.isRunning = false
	s.mu.Unlock()

	close(s.stopChan)
	s.wg.Wait()
}

// GetSyncProgress 获取同步进度
func (s *DBSyncer) GetSyncProgress() SyncProgress {
	metadata := s.walManager.GetMetadata()
	return metadata.SyncProgress
}

// GetSyncStats 获取同步统计信息
func (s *DBSyncer) GetSyncStats() map[string]interface{} {
	metadata := s.walManager.GetMetadata()
	
	totalSegments := len(metadata.Segments)
	syncedSegments := 0
	unsyncedSegments := 0

	for _, segment := range metadata.Segments {
		if segment.Synced {
			syncedSegments++
		} else if segment.EndSeq > 0 { // 只统计已关闭的分片
			unsyncedSegments++
		}
	}

	return map[string]interface{}{
		"total_segments":   totalSegments,
		"synced_segments":  syncedSegments,
		"unsynced_segments": unsyncedSegments,
		"last_synced_seq":  metadata.SyncProgress.LastSyncedSeq,
		"current_segment":  metadata.CurrentSegmentID,
	}
}

// ForceSync 强制立即同步
func (s *DBSyncer) ForceSync() error {
	return s.syncOnce()
}
