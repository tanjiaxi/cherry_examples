/*
 * @Author: Kiro
 * @Date: 2025-12-12
 * @Description: Spin 集成测试 - 直接连接数据库测试完整流程
 *
 * 运行方式:
 *   go test -v ./demo_cluster/test/... -run TestSpinIntegration
 *
 * 注意: 需要确保数据库连接配置正确
 */
package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/cherry-game/examples/demo_cluster/internal/component/db"
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_engine/result"
	spinManager "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_manager"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var testInitialized = false

// setupTest 初始化测试环境
func setupTest(t *testing.T) {
	if testInitialized {
		return
	}

	// 数据库连接配置 - 根据你的实际配置修改
	dsn := "host=localhost user=postgres password=123456 dbname=classic_slots port=5432 sslmode=disable"

	gormDB, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent), // 静默模式，减少日志
	})
	if err != nil {
		t.Skipf("跳过测试: 无法连接数据库 - %v", err)
		return
	}

	// 设置数据库连接
	db.SetTestDB(gormDB)

	// 初始化配置缓存
	dataCenter := configCacheSlots.GetInstance()
	loader := configCacheSlots.NewDataLoader()
	dataCenter.SetLoader(loader)

	if err := dataCenter.Reload(); err != nil {
		t.Skipf("跳过测试: 无法加载配置 - %v", err)
		return
	}

	testInitialized = true
	t.Log("测试环境初始化成功")
}

// TestSpinIntegration_Room86 测试 86 关卡的 spin
func TestSpinIntegration_Room86(t *testing.T) {
	setupTest(t)
	result.RegisteAllGenReslt()
	roomId := int32(86001)
	ruleId := roomId / 1000
	curBet := 10000

	// 获取房间配置
	roomConfig, err := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	if err != nil {
		t.Fatalf("获取房间配置失败: %v", err)
	}

	// 创建房间数据
	roomDataInfo := &slotsModel.RoomDataInfo{
		RoomId:    roomId,
		UserId:    24,
		Stage:     slotsModel.NORMAL,
		StageType: 0,
		SpinNum:   0,
		Version:   1,
	}

	// 执行 spin
	collectAddMoney, reelJsonObj, redBlackFluctuationVal, err := spinManager.SpinBefore(ruleId)
	if err != nil {
		t.Fatalf("SpinBefore 失败: %v", err)
	}

	t.Logf("=== Room 86 Spin 测试 ===")
	t.Logf("RoomId: %d, RuleId: %d, Bet: %d", roomId, ruleId, curBet)

	result, err := spinManager.StarSPin(
		redBlackFluctuationVal,
		roomId,
		ruleId,
		false,
		curBet,
		collectAddMoney,
		roomConfig,
		reelJsonObj,
		roomDataInfo,
	)

	if err != nil {
		t.Fatalf("StarSPin 失败: %v", err)
	}

	t.Logf("Spin 完成, Result: %+v", result)
}

// TestSpinIntegration_MultipleSpins 测试多次 spin 统计
func TestSpinIntegration_MultipleSpins(t *testing.T) {
	setupTest(t)
	result.RegisteAllGenReslt()
	roomId := int32(86001)
	ruleId := roomId / 1000
	curBet := 10000
	rounds := 100

	roomConfig, err := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	if err != nil {
		t.Fatalf("获取房间配置失败: %v", err)
	}

	roomDataInfo := &slotsModel.RoomDataInfo{
		RoomId:    roomId,
		UserId:    24,
		Stage:     slotsModel.NORMAL,
		StageType: 0,
		SpinNum:   0,
		Version:   1,
	}

	t.Logf("=== 多轮 Spin 统计测试 (%d 轮) ===", rounds)

	winCount := 0
	totalCost := 0
	totalWin := 0

	for i := 0; i < rounds; i++ {
		collectAddMoney, reelJsonObj, redBlackFluctuationVal, err := spinManager.SpinBefore(ruleId)
		if err != nil {
			t.Fatalf("第 %d 轮 SpinBefore 失败: %v", i+1, err)
		}

		_, err = spinManager.StarSPin(
			redBlackFluctuationVal,
			roomId,
			ruleId,
			false,
			curBet,
			collectAddMoney,
			roomConfig,
			reelJsonObj,
			roomDataInfo,
		)

		if err != nil {
			t.Fatalf("第 %d 轮 StarSPin 失败: %v", i+1, err)
		}

		totalCost += curBet
		roomDataInfo.SpinNum++

		// 这里可以根据实际返回结果统计中奖
		// if result != nil && result.TotalWin > 0 {
		//     winCount++
		//     totalWin += result.TotalWin
		// }
	}

	t.Logf("统计结果:")
	t.Logf("  总轮数: %d", rounds)
	t.Logf("  中奖次数: %d (%.2f%%)", winCount, float64(winCount)/float64(rounds)*100)
	t.Logf("  总投入: %d", totalCost)
	t.Logf("  总中奖: %d", totalWin)
	if totalCost > 0 {
		t.Logf("  RTP: %.2f%%", float64(totalWin)/float64(totalCost)*100)
	}
}

// TestSpinIntegration_PrintRealData 打印真实数据结构（用于调试）
func TestSpinIntegration_PrintRealData(t *testing.T) {
	setupTest(t)

	ruleId := int32(86)

	// 获取符号配置
	symbols, err := configCacheSlots.GetInstance().GetCardConfig(ruleId)
	if err != nil {
		t.Fatalf("获取符号配置失败: %v", err)
	}

	fmt.Println("========== 符号配置 (ruleId=86) ==========")
	for id, sym := range symbols {
		fmt.Printf("\n符号 %d (CardIndex=%d):\n", id, sym.N2CfgCard.Cardindex)
		fmt.Printf("  Replacefunction: %d (是否Wild)\n", sym.N2CfgCard.Replacefunction)
		fmt.Printf("  Isreplacable: %d (是否可被Wild替换)\n", sym.N2CfgCard.Isreplacable)
		fmt.Printf("  Ifonbetline: %d (是否在线上)\n", sym.N2CfgCard.Ifonbetline)
		fmt.Printf("  Ifcontinue: %d (是否连续)\n", sym.N2CfgCard.Ifcontinue)
		fmt.Printf("  MixedGroup: %d\n", sym.MixedGroup)
		fmt.Printf("  MixedGroupAll: %v\n", sym.MixedGroupAll)
		fmt.Printf("  Conditionoddsseq: %v\n", sym.Conditionoddsseq)
		if len(sym.MixedOdds) > 0 {
			fmt.Printf("  MixedOdds: %v\n", sym.MixedOdds)
		}
	}

	// 获取线路配置
	lineIds, err := configCacheSlots.GetInstance().GetFromatLineIds(3, 3)
	if err != nil {
		t.Fatalf("获取线路ID配置失败: %v", err)
	}

	fmt.Println("\n========== 线路配置 (3x3) ==========")
	fmt.Printf("线路ID列表: %v\n", lineIds.Ids)

	for i := 0; i < 5 && i < len(lineIds.Ids); i++ {
		lineData, err := configCacheSlots.GetInstance().GetFromatLines(3, 3, lineIds.Ids[i])
		if err != nil {
			continue
		}
		fmt.Printf("线路 %d: ", lineIds.Ids[i])
		for _, pos := range lineData.LinesArr {
			fmt.Printf("(%d,%d) ", pos.X, pos.Y)
		}
		fmt.Println()
	}
}

// 如果直接运行此文件
func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
