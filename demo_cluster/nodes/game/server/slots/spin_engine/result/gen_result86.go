/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-19 16:12:07
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package result

import (
	"fmt"
	"sync"

	slotsConfigCache "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	dbData "github.com/cherry-game/examples/demo_cluster/internal/db"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

type ReelsConfig86 struct {
	ReelsConfigBase
	SpLine   []int `json:"spLine"`
	SpReward []int `json:"spReward"`
}
type GenResult86 struct {
	GenResultBase
	ReelsConfig86
}

func NewGenResult86() *GenResult86 {
	genResultBase := NewGenResultBase()
	genResultBase.x = 3
	genResultBase.y = 3
	return &GenResult86{
		GenResultBase: *genResultBase,
		ReelsConfig86: ReelsConfig86{},
	}
	//这里如果测试对gc有影响，可是采用对象池的方式，现在测试出来影响不大
	// return AcquireGenResult86()
}

// allMap [][]int,
// posInSequence []int,
// allLines *[][]dbData.Line,
// allSymbol map[int32]*dbData.FormatCardConfig,
// bet int,
// options GetResultOptions,
func (g *GenResult86) GenResult(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig slotsConfigCache.IRoomListConfig) (*pb.SpinResult, error) {
	g.SetSeedSave()
	allLines, err := g.GetAllLinesData(int(roomCongfig.GetLineMax()))
	if err != nil {
		return nil, err
	}

	allSymbol, err := g.GetRoomSymbolCfg()
	if err != nil {
		return nil, err
	}
	if err := g.UnmarshalReelJsonObj(&g.ReelsConfig86); err != nil {
		return nil, err
	}
	resultInfo, err := g.genResultLogic(allLines, allSymbol, bet)
	if err != nil {
		return nil, err
	}
	return resultInfo, nil
}
func (g *GenResult86) genResultLogic(allLines *[][]dbData.Line, allSymbol map[int32]*dbData.FormatCardConfig, bet int) (*pb.SpinResult, error) {
	// 初始化SpinResultData用于收集多棋盘数据
	reelsArrayId, err := g.formatReels()
	if err != nil {
		return nil, fmt.Errorf("formatReels error: %v", err)
	}
	spinData := &SpinResultData{
		AllMaps:       make([][][]int, 0),
		PosInReelsAll: make([][]int, 0),
		HoldWinsAll:   make([][]int, 0),
		StopInfoAll:   make([][][]int, 0),
		WinInfoAll:    make([]winsInfo, 0),
		ReelsArrayId:  reelsArrayId,
	}

	balanceMap, posInSequence, err := g.GetGameMap(&g.ReelsConfigBase)
	if err != nil {
		return nil, err
	}
	// 测试 Wild 替换: 线路2 应该是 [4, 2, 2]，符号4可被Wild(2)替换
	// balanceMap = [][]int{
	// 	{2, 10, 2},
	// 	{10, 2, 10},
	// 	{2, 10, 1}, // 底部横线: 4, 2, 2 -> 应该中奖 (Wild替换)
	// }
	// 调试：打印真实数据
	// g.debugPrintData(balanceMap, allLines, allSymbol)

	options := GetResultOptions{}
	// 生成结果
	getResultInfo, err := g.GetResult(balanceMap, posInSequence, allLines, allSymbol, bet, options)
	if err != nil {
		return nil, err
	}
	retDataInfo, err := g.GetRetData(balanceMap, posInSequence, allLines, allSymbol, bet, getResultInfo)
	if err != nil {
		return nil, err
	}

	// 收集数据到SpinResultData
	spinData.AllMaps = append(spinData.AllMaps, balanceMap)
	spinData.PosInReelsAll = append(spinData.PosInReelsAll, posInSequence)
	spinData.HoldWinsAll = append(spinData.HoldWinsAll, retDataInfo.holdWins)
	spinData.StopInfoAll = append(spinData.StopInfoAll, retDataInfo.stopInfo)
	spinData.WinInfoAll = append(spinData.WinInfoAll, retDataInfo.winInfo)

	// 调试：打印结果
	// g.debugPrintResult(getResultInfo)

	// 使用通用方法转换为pb.SpinResult
	pbResult := g.ConvertToPbSpinResult(spinData)

	// 调试：打印转换后的pb.SpinResult
	// g.debugPrintPbSpinResult(pbResult)

	return pbResult, nil
}

// debugPrintData 打印调试数据
func (g *GenResult86) debugPrintData(balanceMap [][]int, allLines *[][]dbData.Line, allSymbol map[int32]*dbData.FormatCardConfig) {
	fmt.Println("========== DEBUG: 真实数据 ==========")
	fmt.Println("棋盘 (balanceMap):")
	for i, row := range balanceMap {
		fmt.Printf("  行%d: %v\n", i, row)
	}

	fmt.Printf("\n线路数量: %d\n", len(*allLines))
	fmt.Println("所有线路:")
	for i := 0; i < len(*allLines); i++ {
		line := (*allLines)[i]
		fmt.Printf("  线路%d: ", i)
		for _, pos := range line {
			fmt.Printf("(%d,%d) ", pos.X, pos.Y)
		}
		// 打印该线路上的符号
		fmt.Print(" -> 符号: ")
		for _, pos := range line {
			fmt.Printf("%d ", balanceMap[pos.X][pos.Y])
		}
		fmt.Println()
	}

	fmt.Printf("\n符号配置数量: %d\n", len(allSymbol))
	fmt.Println("符号详情:")
	for id, sym := range allSymbol {
		fmt.Printf("  符号%d: CardIndex=%d, Replacefunction=%d, Isreplacable=%d, MixedGroup=%d, MixedGroupAll=%v\n",
			id, sym.N2CfgCard.Cardindex, sym.N2CfgCard.Replacefunction, sym.N2CfgCard.Isreplacable,
			sym.MixedGroup, sym.MixedGroupAll)
		fmt.Printf("         Conditionoddsseq=%v\n", sym.Conditionoddsseq)
		if len(sym.MixedOdds) > 0 {
			fmt.Printf("         MixedOdds=%v\n", sym.MixedOdds)
		}
	}
	fmt.Println("=====================================")
}

// debugPrintResult 打印结果
func (g *GenResult86) debugPrintResult(result *GetResultInfo) {
	fmt.Println("========== DEBUG: 计算结果 ==========")
	fmt.Printf("CoinsPerLines: %v\n", result.CoinsPerLines)
	fmt.Printf("WinCountPerLines: %v\n", result.WinCountPerLines)
	fmt.Printf("WinSymbolPerLines: %v\n", result.WinSymbolPerLines)
	fmt.Printf("TotalWin: %d\n", result.TotalWin)
	fmt.Println("=====================================")
}

// debugPrintPbSpinResult 打印转换后的pb.SpinResult
func (g *GenResult86) debugPrintPbSpinResult(result *pb.SpinResult) {
	fmt.Println("========== DEBUG: pb.SpinResult ==========")

	// 打印 ReelsArrayId
	fmt.Printf("ReelsArrayId: %v\n", result.ReelsArrayId)

	// 打印 PosInReels
	fmt.Println("PosInReels:")
	for i, pos := range result.PosInReels {
		fmt.Printf("  棋盘%d: %v\n", i, pos.GetArray())
	}

	// 打印 Results (棋盘数据)
	fmt.Println("Results (棋盘数据):")
	for i, board := range result.Results {
		fmt.Printf("  棋盘%d:\n", i)
		for j, row := range board.GetArray() {
			fmt.Printf("    行%d: %v\n", j, row.GetArray())
		}
	}

	// 打印 HoldWins
	fmt.Println("HoldWins:")
	for i, hw := range result.HoldWins {
		fmt.Printf("  棋盘%d: %v\n", i, hw.GetArray())
	}

	// 打印 StopInfo
	fmt.Println("StopInfo:")
	for i, si := range result.StopInfo {
		fmt.Printf("  棋盘%d:\n", i)
		for j, row := range si.GetArray() {
			fmt.Printf("    行%d: %v\n", j, row.GetArray())
		}
	}

	// 打印 WinInfo
	fmt.Println("WinInfo:")
	for i, wi := range result.WinInfo {
		fmt.Printf("  棋盘%d: Type=%v, Win=%d, KindType=%d\n", i, wi.Type, wi.Win, wi.KindType)
		for j, line := range wi.Lines {
			fmt.Printf("    线路%d: Id=%d, Win=%d, Symbol=%d\n", j, line.Id, line.Win, line.Symbol)
			fmt.Printf("      Positions: ")
			for _, pos := range line.Positions {
				fmt.Printf("%v ", pos.GetArray())
			}
			fmt.Println()
		}
	}

	fmt.Println("==========================================")
}

// GenResult86 对象池
var genResult86Pool = sync.Pool{
	New: func() any {
		return &GenResult86{
			GenResultBase: GenResultBase{
				x: 3,
				y: 3,
			},
		}
	},
}

func AcquireGenResult86() *GenResult86 {
	return genResult86Pool.Get().(*GenResult86)
}
func ReleaseGenResult86(g *GenResult86) {
	// 重置所有状态
	g.roomDataInfo = nil
	g.reelJsonObj = nil
	g.roomConfig = nil
	g.roomId = 0
	g.ruleId = 0
	g.bet = 0
	g.stage = 0
	g.stageType = 0
	g.needSave = false
	g.randomNext = 0
	g.reelsStart = 0
	g.reelLevel = 0
	g.reelsIdx = 0
	g.winTotalMoney = 0
	g.winRealTotalMoney = 0
	// 重置 ReelsConfig86
	g.ReelsConfig86 = ReelsConfig86{}

	genResult86Pool.Put(g)
}
