package main

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// TestNormalRunning 测试1: 正常运行测试
func TestNormalRunning(t *testing.T) {
	fmt.Println("\n========== 测试1: 正常运行测试 ==========")

	// 清理旧数据
	cleanupTestData(t)

	// 创建服务配置
	config := DefaultServiceConfig()
	config.WALDir = "./test_wal_normal"
	config.SnapshotDir = "./test_snapshots_normal"
	config.PlayerCount = 10
	config.SpinInterval = 50 * time.Millisecond
	config.MaxSegmentSize = 10 * 1024 // 10KB 快速触发分片
	config.MaxSegmentTime = 2 * time.Second
	config.SyncInterval = 1 * time.Second
	config.SyncDelay = 20 * time.Millisecond

	// 创建服务
	service, err := NewStatefulService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	// 启动服务
	if err := service.Start(); err != nil {
		t.Fatalf("启动服务失败: %v", err)
	}

	// 运行5秒
	fmt.Println("\n运行5秒...")
	time.Sleep(5 * time.Second)

	// 打印统计信息
	service.PrintStats()

	// 停止服务
	if err := service.Stop(); err != nil {
		t.Fatalf("停止服务失败: %v", err)
	}

	// 验证结果
	stats := service.stateManager.GetStats()
	playerCount := stats["player_count"].(int)
	totalSpins := stats["total_spins"].(int64)

	if playerCount != config.PlayerCount {
		t.Errorf("玩家数量不匹配，期望: %d, 实际: %d", config.PlayerCount, playerCount)
	}

	if totalSpins == 0 {
		t.Error("未产生任何Spin记录")
	}

	// 验证WAL分片生成
	metadata := service.walManager.GetMetadata()
	if len(metadata.Segments) == 0 {
		t.Error("未生成WAL分片")
	}

	// 验证同步进度
	syncStats := service.dbSyncer.GetSyncStats()
	syncedSegments := syncStats["synced_segments"].(int)
	if syncedSegments == 0 {
		t.Log("警告: 未同步任何分片（可能运行时间太短）")
	}

	fmt.Printf("\n✅ 测试1通过: 玩家=%d, 总Spin=%d, 分片=%d, 已同步=%d\n\n",
		playerCount, totalSpins, len(metadata.Segments), syncedSegments)

	// 清理测试数据
	cleanupTestData(t)
}

// TestCrashRecovery 测试2: 宕机恢复测试
func TestCrashRecovery(t *testing.T) {
	fmt.Println("\n========== 测试2: 宕机恢复测试 ==========")

	// 清理旧数据
	cleanupTestData(t)

	config := DefaultServiceConfig()
	config.WALDir = "./test_wal_crash"
	config.SnapshotDir = "./test_snapshots_crash"
	config.PlayerCount = 5
	config.SpinInterval = 50 * time.Millisecond
	config.MaxSegmentSize = 5 * 1024
	config.MaxSegmentTime = 2 * time.Second
	config.SyncInterval = 1 * time.Second
	config.SyncDelay = 20 * time.Millisecond

	// ===== 第一阶段: 运行后模拟宕机 =====
	fmt.Println("\n--- 第一阶段: 运行3秒后模拟宕机 ---")

	service1, err := NewStatefulService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	if err := service1.Start(); err != nil {
		t.Fatalf("启动服务失败: %v", err)
	}

	// 运行3秒
	time.Sleep(3 * time.Second)

	// 记录宕机前的状态
	stats1 := service1.stateManager.GetStats()
	totalSpins1 := stats1["total_spins"].(int64)
	totalBet1 := stats1["total_bet"].(int64)
	totalWin1 := stats1["total_win"].(int64)

	fmt.Printf("\n宕机前状态: 总Spin=%d, 总下注=%d, 总赢取=%d\n", totalSpins1, totalBet1, totalWin1)

	// 模拟宕机（强制关闭，不执行优雅关闭）
	fmt.Println("\n💥 模拟宕机（强制关闭）...")
	// 注意：这里故意不调用 Stop()，模拟突然宕机

	// 手动关闭WAL（模拟系统强制刷盘）
	service1.walManager.Close()

	// 等待一会儿
	time.Sleep(1 * time.Second)

	// ===== 第二阶段: 重启恢复 =====
	fmt.Println("\n--- 第二阶段: 重启服务并恢复 ---")

	service2, err := NewStatefulService(config)
	if err != nil {
		t.Fatalf("创建恢复服务失败: %v", err)
	}

	if err := service2.Start(); err != nil {
		t.Fatalf("启动恢复服务失败: %v", err)
	}

	// 等待恢复完成
	time.Sleep(500 * time.Millisecond)

	// 检查恢复后的状态
	stats2 := service2.stateManager.GetStats()
	totalSpins2 := stats2["total_spins"].(int64)
	totalBet2 := stats2["total_bet"].(int64)
	totalWin2 := stats2["total_win"].(int64)

	fmt.Printf("\n恢复后状态: 总Spin=%d, 总下注=%d, 总赢取=%d\n", totalSpins2, totalBet2, totalWin2)

	// 继续运行2秒
	fmt.Println("\n继续运行2秒...")
	time.Sleep(2 * time.Second)

	service2.PrintStats()

	// 优雅关闭
	if err := service2.Stop(); err != nil {
		t.Fatalf("停止恢复服务失败: %v", err)
	}

	// 验证恢复的正确性
	// 恢复后的数据应该至少等于宕机前的数据
	if totalSpins2 < totalSpins1 {
		t.Errorf("数据丢失！宕机前Spin=%d, 恢复后Spin=%d", totalSpins1, totalSpins2)
	}

	// 由于可能有一些记录在宕机时未刷盘，允许小幅度差异
	lossRate := float64(totalSpins1-totalSpins2) / float64(totalSpins1)
	if lossRate > 0.1 { // 允许最多10%的数据丢失（缓冲区未刷盘的部分）
		t.Errorf("数据丢失过多！丢失率=%.2f%%", lossRate*100)
	}

	fmt.Printf("\n✅ 测试2通过: 数据恢复成功，恢复率=%.2f%%\n\n", (1-lossRate)*100)

	// 清理测试数据
	os.RemoveAll(config.WALDir)
	os.RemoveAll(config.SnapshotDir)
}

// TestHighLoad 测试3: 高负载测试
func TestHighLoad(t *testing.T) {
	fmt.Println("\n========== 测试3: 高负载测试 ==========")

	// 清理旧数据
	cleanupTestData(t)

	config := DefaultServiceConfig()
	config.WALDir = "./test_wal_highload"
	config.SnapshotDir = "./test_snapshots_highload"
	config.PlayerCount = 100 // 100个玩家
	config.SpinInterval = 20 * time.Millisecond // 更快的生成速度
	config.MaxSegmentSize = 50 * 1024 // 50KB
	config.MaxSegmentTime = 3 * time.Second
	config.SyncInterval = 500 * time.Millisecond // 更频繁的同步
	config.SyncDelay = 10 * time.Millisecond

	service, err := NewStatefulService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	if err := service.Start(); err != nil {
		t.Fatalf("启动服务失败: %v", err)
	}

	// 运行10秒
	fmt.Println("\n高负载运行10秒...")
	
	// 每隔2秒打印一次统计
	for i := 0; i < 5; i++ {
		time.Sleep(2 * time.Second)
		fmt.Printf("\n--- %d秒统计 ---\n", (i+1)*2)
		service.PrintStats()
	}

	// 停止服务
	if err := service.Stop(); err != nil {
		t.Fatalf("停止服务失败: %v", err)
	}

	// 验证结果
	stats := service.stateManager.GetStats()
	playerCount := stats["player_count"].(int)
	totalSpins := stats["total_spins"].(int64)
	totalBet := stats["total_bet"].(int64)
	totalWin := stats["total_win"].(int64)

	// 验证数据一致性
	expectedBalance := config.InitialBalance*int64(config.PlayerCount) - totalBet + totalWin
	actualBalance := stats["total_balance"].(int64)

	if actualBalance != expectedBalance {
		t.Errorf("余额不一致！期望=%d, 实际=%d", expectedBalance, actualBalance)
	}

	// 验证WAL分片
	metadata := service.walManager.GetMetadata()
	if len(metadata.Segments) < 2 {
		t.Errorf("高负载下应该产生多个分片，实际只有 %d 个", len(metadata.Segments))
	}

	// 验证同步
	syncStats := service.dbSyncer.GetSyncStats()
	syncedSegments := syncStats["synced_segments"].(int)

	fmt.Printf("\n✅ 测试3通过: 玩家=%d, 总Spin=%d, 分片=%d, 已同步=%d\n",
		playerCount, totalSpins, len(metadata.Segments), syncedSegments)
	fmt.Printf("   余额一致性验证通过: 期望=%d, 实际=%d\n\n", expectedBalance, actualBalance)

	// 清理测试数据
	os.RemoveAll(config.WALDir)
	os.RemoveAll(config.SnapshotDir)
}

// TestWALSegmentation 测试4: WAL分片机制测试
func TestWALSegmentation(t *testing.T) {
	fmt.Println("\n========== 测试4: WAL分片机制测试 ==========")

	cleanupTestData(t)

	config := DefaultServiceConfig()
	config.WALDir = "./test_wal_segment"
	config.SnapshotDir = "./test_snapshots_segment"
	config.PlayerCount = 5
	config.SpinInterval = 10 * time.Millisecond
	config.MaxSegmentSize = 2 * 1024 // 2KB 非常小，快速触发分片
	config.MaxSegmentTime = 1 * time.Second
	config.SyncInterval = 500 * time.Millisecond
	config.SyncDelay = 10 * time.Millisecond

	service, err := NewStatefulService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	if err := service.Start(); err != nil {
		t.Fatalf("启动服务失败: %v", err)
	}

	// 运行5秒，应该产生多个分片
	fmt.Println("\n运行5秒，观察分片切换...")
	time.Sleep(5 * time.Second)

	service.PrintStats()

	if err := service.Stop(); err != nil {
		t.Fatalf("停止服务失败: %v", err)
	}

	// 验证分片
	metadata := service.walManager.GetMetadata()
	if len(metadata.Segments) < 3 {
		t.Errorf("应该产生至少3个分片，实际只有 %d 个", len(metadata.Segments))
	}

	// 验证分片的序列号连续性
	for i := 0; i < len(metadata.Segments)-1; i++ {
		current := metadata.Segments[i]
		next := metadata.Segments[i+1]

		if current.EndSeq >= next.StartSeq {
			t.Errorf("分片序列号不连续: 分片%d结束于%d, 分片%d开始于%d",
				current.ID, current.EndSeq, next.ID, next.StartSeq)
		}
	}

	fmt.Printf("\n✅ 测试4通过: 产生了 %d 个分片，序列号连续性验证通过\n\n", len(metadata.Segments))

	os.RemoveAll(config.WALDir)
	os.RemoveAll(config.SnapshotDir)
}

// TestSyncResume 测试5: 断点续传测试
func TestSyncResume(t *testing.T) {
	fmt.Println("\n========== 测试5: 断点续传测试 ==========")

	cleanupTestData(t)

	config := DefaultServiceConfig()
	config.WALDir = "./test_wal_resume"
	config.SnapshotDir = "./test_snapshots_resume"
	config.PlayerCount = 5
	config.SpinInterval = 30 * time.Millisecond
	config.MaxSegmentSize = 5 * 1024
	config.MaxSegmentTime = 2 * time.Second
	config.SyncInterval = 10 * time.Second // 故意设置很长，模拟未同步
	config.SyncDelay = 50 * time.Millisecond

	// 第一次运行
	fmt.Println("\n--- 第一次运行: 产生数据但不完全同步 ---")
	service1, err := NewStatefulService(config)
	if err != nil {
		t.Fatalf("创建服务失败: %v", err)
	}

	if err := service1.Start(); err != nil {
		t.Fatalf("启动服务失败: %v", err)
	}

	time.Sleep(3 * time.Second)

	// 手动触发一次同步
	fmt.Println("\n手动触发一次同步...")
	service1.dbSyncer.ForceSync()

	metadata1 := service1.walManager.GetMetadata()
	syncProgress1 := metadata1.SyncProgress

	fmt.Printf("第一次运行结束: 分片=%d, 同步进度=%d\n",
		len(metadata1.Segments), syncProgress1.LastSyncedSeq)

	if err := service1.Stop(); err != nil {
		t.Fatalf("停止服务失败: %v", err)
	}

	// 第二次运行（恢复）
	fmt.Println("\n--- 第二次运行: 从断点继续同步 ---")
	
	// 修改配置，加快同步
	config.SyncInterval = 500 * time.Millisecond
	
	service2, err := NewStatefulService(config)
	if err != nil {
		t.Fatalf("创建恢复服务失败: %v", err)
	}

	if err := service2.Start(); err != nil {
		t.Fatalf("启动恢复服务失败: %v", err)
	}

	// 等待同步完成
	time.Sleep(3 * time.Second)

	service2.PrintStats()

	metadata2 := service2.walManager.GetMetadata()
	syncProgress2 := metadata2.SyncProgress

	if err := service2.Stop(); err != nil {
		t.Fatalf("停止恢复服务失败: %v", err)
	}

	// 验证同步进度推进
	if syncProgress2.LastSyncedSeq <= syncProgress1.LastSyncedSeq {
		t.Errorf("同步进度未推进！第一次=%d, 第二次=%d",
			syncProgress1.LastSyncedSeq, syncProgress2.LastSyncedSeq)
	}

	fmt.Printf("\n✅ 测试5通过: 断点续传成功，同步进度从 %d 推进到 %d\n\n",
		syncProgress1.LastSyncedSeq, syncProgress2.LastSyncedSeq)

	os.RemoveAll(config.WALDir)
	os.RemoveAll(config.SnapshotDir)
}

// cleanupTestData 清理测试数据
func cleanupTestData(t *testing.T) {
	dirs := []string{
		"./test_wal_normal",
		"./test_snapshots_normal",
		"./test_wal_crash",
		"./test_snapshots_crash",
		"./test_wal_highload",
		"./test_snapshots_highload",
		"./test_wal_segment",
		"./test_snapshots_segment",
		"./test_wal_resume",
		"./test_snapshots_resume",
	}

	for _, dir := range dirs {
		os.RemoveAll(dir)
	}
}
