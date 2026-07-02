package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// StatefulService 有状态服务
type StatefulService struct {
	walManager    *WALSegmentManager
	stateManager  *PlayerStateManager
	dbSyncer      *DBSyncer
	dataGenerator *DataGenerator
	sequenceGen   *SequenceGenerator
	config        ServiceConfig
	isRunning     atomic.Bool
}

// ServiceConfig 服务配置
type ServiceConfig struct {
	WALDir         string
	SnapshotDir    string
	PlayerCount    int
	InitialBalance int64
	SpinInterval   time.Duration
	MaxSegmentSize int64
	MaxSegmentTime time.Duration
	SyncInterval   time.Duration
	SyncDelay      time.Duration
}

// DefaultServiceConfig 默认配置
func DefaultServiceConfig() ServiceConfig {
	return ServiceConfig{
		WALDir:         "./wal_data",
		SnapshotDir:    "./snapshots",
		PlayerCount:    10,
		InitialBalance: 100000,
		SpinInterval:   100 * time.Millisecond,
		MaxSegmentSize: 1024 * 1024, // 1MB
		MaxSegmentTime: 10 * time.Second,
		SyncInterval:   2 * time.Second,
		SyncDelay:      50 * time.Millisecond,
	}
}

// NewStatefulService 创建有状态服务
func NewStatefulService(config ServiceConfig) (*StatefulService, error) {
	// 确保目录存在
	if err := os.MkdirAll(config.SnapshotDir, 0755); err != nil {
		return nil, err
	}

	// 创建组件
	walManager, err := NewWALSegmentManager(config.WALDir, config.MaxSegmentSize, config.MaxSegmentTime)
	if err != nil {
		return nil, fmt.Errorf("创建WAL管理器失败: %w", err)
	}

	stateManager := NewPlayerStateManager()
	dbSyncer := NewDBSyncer(walManager, stateManager, config.SyncInterval, config.SyncDelay)
	sequenceGen := NewSequenceGenerator()

	service := &StatefulService{
		walManager:   walManager,
		stateManager: stateManager,
		dbSyncer:     dbSyncer,
		sequenceGen:  sequenceGen,
		config:       config,
	}

	return service, nil
}

// Start 启动服务
func (s *StatefulService) Start() error {
	if s.isRunning.Load() {
		return fmt.Errorf("服务已在运行")
	}

	fmt.Println("=== 启动有状态服务 ===")

	// 1. 检查并恢复状态
	if err := s.recoverState(); err != nil {
		return fmt.Errorf("恢复状态失败: %w", err)
	}

	// 2. 启动数据库同步器
	s.dbSyncer.Start()

	// 3. 初始化玩家（如果是首次启动）
	s.initializePlayers()

	// 4. 启动数据生成器
	s.dataGenerator = NewDataGenerator(
		s.stateManager,
		s.walManager,
		s.sequenceGen,
		s.config.PlayerCount,
		s.config.SpinInterval,
	)
	s.dataGenerator.Start()

	s.isRunning.Store(true)
	fmt.Println("=== 服务启动完成 ===")

	return nil
}

// recoverState 恢复状态
func (s *StatefulService) recoverState() error {
	fmt.Println("[Recovery] 检查是否需要恢复...")

	// 1. 尝试从快照恢复
	snapshotPath := s.config.SnapshotDir + "/player_snapshot.json"
	if err := s.stateManager.ImportSnapshot(snapshotPath); err != nil {
		fmt.Printf("[Recovery] 未找到快照文件，将从零开始: %v\n", err)
	} else {
		fmt.Printf("[Recovery] 成功从快照恢复玩家状态\n")
	}

	// 2. 检查WAL元数据
	metadata := s.walManager.GetMetadata()
	if len(metadata.Segments) > 0 {
		fmt.Printf("[Recovery] 发现 %d 个WAL分片\n", len(metadata.Segments))

		// 3. 从最后同步点之后的WAL重放记录
		lastSyncedSeq := metadata.SyncProgress.LastSyncedSeq
		fmt.Printf("[Recovery] 上次同步到序列号: %d\n", lastSyncedSeq)

		// 重放未同步的记录
		replayCount := 0
		for _, segment := range metadata.Segments {
			if segment.StartSeq > lastSyncedSeq {
				records, err := s.walManager.ReadSegmentRecords(segment)
				if err != nil {
					return fmt.Errorf("读取分片 %d 失败: %w", segment.ID, err)
				}

				for _, record := range records {
					if record.Sequence > lastSyncedSeq {
						s.stateManager.UpdatePlayerState(record)
						replayCount++
					}
				}
			}
		}

		if replayCount > 0 {
			fmt.Printf("[Recovery] 重放了 %d 条WAL记录\n", replayCount)
		}

		// 4. 更新序列号生成器
		maxSeq := int64(0)

		// 先从所有分片找最大序列号
		for _, segment := range metadata.Segments {
			if segment.EndSeq > maxSeq {
				maxSeq = segment.EndSeq
			}
		}

		// 再从玩家状态找最大序列号
		for _, player := range s.stateManager.GetAllPlayers() {
			if player.LastSeq > maxSeq {
				maxSeq = player.LastSeq
			}
		}

		if maxSeq > 0 {
			s.sequenceGen.SetInitialSequence(maxSeq)
			fmt.Printf("[Recovery] 序列号生成器从 %d 开始\n", maxSeq+1)
		}
	} else {
		fmt.Println("[Recovery] 未找到WAL数据，首次启动")
	}

	return nil
}

// initializePlayers 初始化玩家
func (s *StatefulService) initializePlayers() {
	players := s.stateManager.GetAllPlayers()
	if len(players) > 0 {
		fmt.Printf("[Init] 已有 %d 个玩家，跳过初始化\n", len(players))
		return
	}

	fmt.Printf("[Init] 初始化 %d 个玩家...\n", s.config.PlayerCount)
	for i := 1; i <= s.config.PlayerCount; i++ {
		userID := int64(1000 + i)
		s.stateManager.GetOrCreatePlayer(userID, s.config.InitialBalance)
	}
	fmt.Printf("[Init] 玩家初始化完成\n")
}

// Stop 停止服务
func (s *StatefulService) Stop() error {
	if !s.isRunning.Load() {
		return nil
	}

	fmt.Println("\n=== 开始优雅关闭服务 ===")

	// 1. 停止数据生成
	if s.dataGenerator != nil {
		fmt.Println("[Shutdown] 停止数据生成...")
		s.dataGenerator.Stop()
	}

	// 2. 等待WAL刷盘
	fmt.Println("[Shutdown] 等待WAL刷盘...")
	time.Sleep(100 * time.Millisecond)

	// 3. 停止数据库同步器（会执行最后一次同步）
	fmt.Println("[Shutdown] 停止数据库同步器...")
	s.dbSyncer.Stop()

	// 4. 保存状态快照
	fmt.Println("[Shutdown] 保存状态快照...")
	snapshotPath := s.config.SnapshotDir + "/player_snapshot.json"
	if err := s.stateManager.ExportSnapshot(snapshotPath); err != nil {
		fmt.Printf("[Shutdown] 保存快照失败: %v\n", err)
	} else {
		fmt.Println("[Shutdown] 快照保存成功")
	}

	// 5. 关闭WAL管理器
	fmt.Println("[Shutdown] 关闭WAL管理器...")
	if err := s.walManager.Close(); err != nil {
		return fmt.Errorf("关闭WAL管理器失败: %w", err)
	}

	s.isRunning.Store(false)
	fmt.Println("=== 服务已安全关闭 ===")

	return nil
}

// PrintStats 打印统计信息
func (s *StatefulService) PrintStats() {
	fmt.Println("\n=== 服务统计信息 ===")

	// 玩家状态统计
	stats := s.stateManager.GetStats()
	fmt.Printf("玩家数量: %d\n", stats["player_count"])
	fmt.Printf("总余额: %d\n", stats["total_balance"])
	fmt.Printf("总下注: %d\n", stats["total_bet"])
	fmt.Printf("总赢取: %d\n", stats["total_win"])
	fmt.Printf("总旋转: %d\n", stats["total_spins"])

	// 同步统计
	syncStats := s.dbSyncer.GetSyncStats()
	fmt.Printf("\n分片总数: %d\n", syncStats["total_segments"])
	fmt.Printf("已同步分片: %d\n", syncStats["synced_segments"])
	fmt.Printf("未同步分片: %d\n", syncStats["unsynced_segments"])
	fmt.Printf("最后同步序列号: %d\n", syncStats["last_synced_seq"])
	fmt.Printf("当前分片ID: %d\n", syncStats["current_segment"])

	fmt.Println("==================")
}

// WaitForShutdownSignal 等待关闭信号
func (s *StatefulService) WaitForShutdownSignal() {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	fmt.Println("\n收到关闭信号...")
}

// SequenceGenerator 序列号生成器（全局唯一递增）
type SequenceGenerator struct {
	current int64
}

func NewSequenceGenerator() *SequenceGenerator {
	return &SequenceGenerator{current: 0}
}

func (g *SequenceGenerator) Next() int64 {
	return atomic.AddInt64(&g.current, 1)
}

func (g *SequenceGenerator) SetInitialSequence(seq int64) {
	atomic.StoreInt64(&g.current, seq)
}

// DataGenerator 数据生成器
type DataGenerator struct {
	stateManager *PlayerStateManager
	walManager   *WALSegmentManager
	sequenceGen  *SequenceGenerator
	playerCount  int
	spinInterval time.Duration
	stopChan     chan struct{}
	wg           sync.WaitGroup
}

func NewDataGenerator(
	stateManager *PlayerStateManager,
	walManager *WALSegmentManager,
	sequenceGen *SequenceGenerator,
	playerCount int,
	spinInterval time.Duration,
) *DataGenerator {
	return &DataGenerator{
		stateManager: stateManager,
		walManager:   walManager,
		sequenceGen:  sequenceGen,
		playerCount:  playerCount,
		spinInterval: spinInterval,
		stopChan:     make(chan struct{}),
	}
}

func (g *DataGenerator) Start() {
	fmt.Printf("[Data Generator] 启动 %d 个玩家模拟器...\n", g.playerCount)

	for i := 1; i <= g.playerCount; i++ {
		g.wg.Add(1)
		go g.playerSpinLoop(int64(1000 + i))
	}
}

func (g *DataGenerator) playerSpinLoop(userID int64) {
	defer g.wg.Done()

	ticker := time.NewTicker(g.spinInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// 生成Spin记录
			record := SpinRecord{
				UserID:    userID,
				BetAmount: 100,
				WinAmount: int64(time.Now().UnixNano() % 200), // 模拟随机赢取
				Sequence:  g.sequenceGen.Next(),
				Timestamp: time.Now().UnixNano(),
			}

			// 更新内存状态
			g.stateManager.UpdatePlayerState(record)

			// 写入WAL
			if err := g.walManager.WriteRecord(record); err != nil {
				fmt.Printf("[Data Generator] 写入WAL失败: %v\n", err)
			}

		case <-g.stopChan:
			return
		}
	}
}

func (g *DataGenerator) Stop() {
	close(g.stopChan)
	g.wg.Wait()
	fmt.Println("[Data Generator] 已停止")
}
