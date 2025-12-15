/*
 * @Author: Kiro
 * @Date: 2025-12-11
 * @Description: Slots 中奖计算数学逻辑 - 重构版本
 */
package result

import (
	"fmt"

	dbData "github.com/cherry-game/examples/demo_cluster/internal/db"
)

// ==================== 类型定义 ====================

// DealMapResultOptions 处理地图结果的配置选项
type DealMapResultOptions struct {
	MultiBet      [][]int // 倍数下注矩阵
	NoMultiSymbol []int   // 不参与倍数计算的符号
	GetHoldWin    bool    // 是否获取 hold win
	WildNeedMulti bool    // wild 是否需要倍数
	MultiAdd      bool    // 倍数是否累加（false 为累乘）
}

// SymbolWinInfo 符号中奖信息
type SymbolWinInfo struct {
	MinCount int // 最小中奖数量
	MaxCount int // 最大中奖数量
}

// DealMapResultResponse 处理地图结果的返回值
type DealMapResultResponse struct {
	SymbolWinMap      map[int]*SymbolWinInfo // 符号中奖映射
	CoinsPerLines     []int                  // 每线中奖金币
	WinCountPerLines  []int                  // 每线中奖数量
	WinSymbolPerLines []int                  // 每线中奖符号
	HoldWinData       [][]int                // hold win 数据
}

// GetResultOptions getResult 方法的配置选项
type GetResultOptions struct {
	IsInit bool // 是否初始化
	IsRun  bool // 是否运行模式
	NoFake bool // 是否禁用假棋盘
}

type GetResultInfo struct {
	WinSymbolPerLines []int                  // 每线中奖符号
	WinCountPerLines  []int                  // 每线中奖数量
	CoinsPerLines     []int                  // 每线中奖金币
	SymbolWinMap      map[int]*SymbolWinInfo // 符号中奖映射
	TotalWin          int                    // 总中奖
}

// lineContext 单条线路计算的上下文
type lineContext struct {
	winCnt               int                      // 正常中奖个数
	winSymbolCnt         int                      // 正常中奖棋子个数
	winSymbolId          int                      // 中奖符号ID
	wildSymbolCnt        int                      // wild中奖棋子个数
	wildCnt              int                      // wild数量
	winBet               int                      // 中奖倍数
	winSymbol            *dbData.FormatCardConfig // 中奖符号配置
	lastSymbol           *dbData.FormatCardConfig // 上一个符号配置
	onLine               bool                     // 是否在线上
	isContinue           bool                     // 是否连续
	inGroup              bool                     // 是否在组内
	groupCnt             map[int]int              // 组中奖数量
	groupSymbolCnt       map[int]int              // 组中奖符号数量
	winGroup             []int                    // 当前连续组
	singleCnt            map[int]int              // 单个符号数量
	singleWildCnt        map[int]int              // 单个wild数量
	singleGroupCnt       map[int]int              // 单个组数量
	singleGroupSymbolCnt map[int]int
	mirrorSymbolId       map[int]int // 镜像符号映射
	hasGroup             map[int]int // 是否有这个组
	lastGroupSymbol      map[int]int // 分组中上一个棋子
}

// symbolAnalysis 符号分析结果
type symbolAnalysis struct {
	wildSymbols    []int         // wild符号列表
	replaceSymbols []int         // 可替换符号列表
	groupSymbolIds map[int][]int // group -> symbolIds
	symbolWinMap   map[int]*SymbolWinInfo
}

// ==================== 主要方法 ====================

// GetResult 获取 spin 结果
// allMap: 棋盘数据 [x][y] 格式，例如 3x3 矩阵 allMap[0][0] 是第一行第一列
func (g *GenResultBase) GetResult(
	allMap [][]int,
	posInSequence []int,
	allLines *[][]dbData.Line,
	allSymbol map[int32]*dbData.FormatCardConfig,
	bet int,
	options GetResultOptions,
) (*GetResultInfo, error) {
	response := &GetResultInfo{
		WinSymbolPerLines: make([]int, 0),
		WinCountPerLines:  make([]int, 0),
		CoinsPerLines:     make([]int, 0),
		SymbolWinMap:      make(map[int]*SymbolWinInfo),
	}

	if !options.IsInit {
		DealMapResultOptions := DealMapResultOptions{
			MultiBet: [][]int{{3, 1, 3}, {1, 3, 1}, {3, 1, 1}},
		}
		result, err := g.SumMapResult(allSymbol, allLines, allMap, DealMapResultOptions)
		if err != nil {
			return nil, err
		}
		response.CoinsPerLines = result.CoinsPerLines
		response.WinCountPerLines = result.WinCountPerLines
		response.WinSymbolPerLines = result.WinSymbolPerLines
		response.SymbolWinMap = result.SymbolWinMap

		for _, coins := range response.CoinsPerLines {
			response.TotalWin += coins
		}
	}

	return response, nil
}

// SumMapResult 兼容中间有其他倍数变化
func (g *GenResultBase) SumMapResult(
	allSymbol map[int32]*dbData.FormatCardConfig,
	allLines *[][]dbData.Line,
	allMaps [][]int,
	options DealMapResultOptions,
) (*DealMapResultResponse, error) {
	if len(allMaps) == 0 {
		return nil, fmt.Errorf("allMaps is empty")
	}
	return g.DealMapResult(allSymbol, allLines, allMaps, options)
}

// DealMapResult 处理地图结果 - 核心中奖计算
func (g *GenResultBase) DealMapResult(
	allSymbol map[int32]*dbData.FormatCardConfig,
	allLines *[][]dbData.Line,
	balanceMap [][]int,
	options DealMapResultOptions,
) (*DealMapResultResponse, error) {
	// Step 1: 分析符号配置
	analysis := g.analyzeSymbols(allSymbol)

	// Step 2: 初始化响应
	response := &DealMapResultResponse{
		SymbolWinMap:      analysis.symbolWinMap,
		CoinsPerLines:     make([]int, len(*allLines)),
		WinCountPerLines:  make([]int, len(*allLines)),
		WinSymbolPerLines: make([]int, len(*allLines)),
		HoldWinData:       make([][]int, 0),
	}

	// Step 3: 遍历每条线路计算中奖
	for lineIdx, line := range *allLines {
		lineResult := g.calculateLineWin(line, balanceMap, allSymbol, analysis, options, lineIdx)

		response.CoinsPerLines[lineIdx] = lineResult.coins
		response.WinCountPerLines[lineIdx] = lineResult.count
		response.WinSymbolPerLines[lineIdx] = lineResult.symbolId

		if options.GetHoldWin {
			response.HoldWinData = append(response.HoldWinData, lineResult.holdWinData)
		}
	}

	return response, nil
}

// ==================== 符号分析 ====================

// analyzeSymbols 分析所有符号配置
func (g *GenResultBase) analyzeSymbols(allSymbol map[int32]*dbData.FormatCardConfig) *symbolAnalysis {
	analysis := &symbolAnalysis{
		wildSymbols:    make([]int, 0),
		replaceSymbols: make([]int, 0),
		groupSymbolIds: make(map[int][]int),
		symbolWinMap:   make(map[int]*SymbolWinInfo),
	}

	for symbolIdInt32, symbol := range allSymbol {
		symbolId := int(symbolIdInt32)

		// 收集 wild 符号
		if symbol.N2CfgCard.Replacefunction == 1 {
			analysis.wildSymbols = append(analysis.wildSymbols, symbolId)
		}

		// 收集可替换
		if symbol.N2CfgCard.Isreplacable == 1 {
			analysis.replaceSymbols = append(analysis.replaceSymbols, symbolId)
		}

		// 收集混合组符号
		for _, group := range symbol.MixedGroupAll {
			if analysis.groupSymbolIds[group] == nil {
				analysis.groupSymbolIds[group] = make([]int, 0)
			}
			analysis.groupSymbolIds[group] = append(analysis.groupSymbolIds[group], symbolId)
		}

		// 计算中奖数量范围
		minCount, maxCount := g.getWinCountRange(symbol)
		analysis.symbolWinMap[symbolId] = &SymbolWinInfo{
			MinCount: minCount,
			MaxCount: maxCount,
		}
	}

	return analysis
}

// getWinCountRange 获取符号的中奖数量范围
func (g *GenResultBase) getWinCountRange(symbol *dbData.FormatCardConfig) (minCount, maxCount int) {
	for k, v := range symbol.Conditionargumentseq {
		if v > 0 {
			if minCount == 0 || k < minCount {
				minCount = k
			}
			if k > maxCount {
				maxCount = k
			}
		}
	}
	return
}

// ==================== 线路中奖计算 ====================

// lineWinResult 线路中奖结果
type lineWinResult struct {
	coins       int
	count       int
	symbolId    int
	holdWinData []int
}

// calculateLineWin 计算单条线路的中奖
func (g *GenResultBase) calculateLineWin(
	line []dbData.Line,
	balanceMap [][]int,
	allSymbol map[int32]*dbData.FormatCardConfig,
	analysis *symbolAnalysis,
	options DealMapResultOptions,
	lineIdx int,
) *lineWinResult {
	// 初始化线路上下文
	ctx := g.newLineContext(options)

	// Step 1: 遍历线路上的每个位置，收集符号信息
	for i := 0; i < len(line); i++ {
		g.processLinePosition(i, line, balanceMap, allSymbol, ctx, options)
	}

	// Step 2: 计算最终中奖
	result := g.calculateFinalWin(ctx, allSymbol, analysis, options)

	return result
}

// newLineContext 创建新的线路上下文
func (g *GenResultBase) newLineContext(options DealMapResultOptions) *lineContext {
	winBet := 1
	if options.MultiAdd {
		winBet = 0
	}

	return &lineContext{
		winBet:               winBet,
		onLine:               true,
		isContinue:           true,
		groupCnt:             make(map[int]int),
		groupSymbolCnt:       make(map[int]int),
		winGroup:             make([]int, 0),
		singleCnt:            make(map[int]int),
		singleWildCnt:        make(map[int]int),
		singleGroupCnt:       make(map[int]int),
		singleGroupSymbolCnt: make(map[int]int),
		mirrorSymbolId:       make(map[int]int),
		hasGroup:             make(map[int]int),
		lastGroupSymbol:      make(map[int]int),
	}
}

// processLinePosition 处理线路上的单个位置
func (g *GenResultBase) processLinePosition(
	i int,
	line []dbData.Line,
	balanceMap [][]int,
	allSymbol map[int32]*dbData.FormatCardConfig,
	ctx *lineContext,
	options DealMapResultOptions,
) {
	x := line[i].X
	y := line[i].Y

	// 获取并处理符号ID
	thisSymbolId := g.resolveSymbolId(balanceMap[x][y], allSymbol, ctx)

	thisSymbol, exists := allSymbol[int32(thisSymbolId)]
	if !exists {
		return
	}

	countNum := g.getCountNum(thisSymbol)
	symbolIndex := int(thisSymbol.N2CfgCard.Cardindex)

	// 跳过特殊计数符号
	if thisSymbol.N2CfgCard.Isspcount == 1 {
		ctx.onLine = false
		return
	}

	// 处理不连续符号
	g.processNonContinuousSymbol(thisSymbol, symbolIndex, countNum, ctx)

	// 处理 wild 符号统计
	g.processWildSymbol(thisSymbol, symbolIndex, countNum, ctx)

	// 检查是否应该跳过
	if g.shouldSkipSymbol(thisSymbol) {
		ctx.onLine = false
		return
	}

	// 处理倍数
	g.processMultiplier(x, y, options, ctx)

	// 处理连续中奖逻辑
	if ctx.onLine {
		g.processContinuousWin(i, x, y, balanceMap, thisSymbol, symbolIndex, countNum, ctx)
	}
}

// resolveSymbolId 解析符号ID（处理镜像符号）
func (g *GenResultBase) resolveSymbolId(
	rawSymbolId int,
	allSymbol map[int32]*dbData.FormatCardConfig,
	ctx *lineContext,
) int {
	symbolId := g.GetRealSymbolId(rawSymbolId)

	// 检查镜像符号缓存
	if mid, exists := ctx.mirrorSymbolId[symbolId]; exists {
		return mid
	}

	// 处理镜像符号
	if sym, exists := allSymbol[int32(symbolId)]; exists && len(sym.MirrorImageIdList) > 0 {
		ctx.mirrorSymbolId[symbolId] = symbolId
		for _, mirrorId := range sym.MirrorImageIdList {
			ctx.mirrorSymbolId[mirrorId] = symbolId
		}
	}

	return symbolId
}

// getCountNum 获取符号的计数数量
func (g *GenResultBase) getCountNum(symbol *dbData.FormatCardConfig) int {
	countNum := int(symbol.N2CfgCard.CountNum)
	if countNum == 0 {
		countNum = 1
	}
	return countNum
}

// shouldSkipSymbol 判断是否应该跳过该符号（不参与连续中奖计算）
// 返回 true 表示该符号不参与连续中奖，但可能参与其他类型中奖
func (g *GenResultBase) shouldSkipSymbol(symbol *dbData.FormatCardConfig) bool {
	// Wild 符号永远不跳过
	if symbol.N2CfgCard.Replacefunction == 1 {
		return false
	}

	// 检查是否在线上 (ifonbetline)
	// 检查是否连续 (ifcontinue)
	// 只有当 ifonbetline=1 且 ifcontinue=1 时，才参与连续中奖
	// 如果 ifonbetline!=1 或 ifcontinue!=1，但不是 wild，则跳过连续中奖计算

	// 但是：如果两个字段都是0（未设置），默认参与连续中奖
	if symbol.N2CfgCard.Ifonbetline == 0 && symbol.N2CfgCard.Ifcontinue == 0 {
		return false
	}

	// 如果在线上 (ifonbetline=1)，无论是否连续都参与
	// 因为即使 ifcontinue=0，也可能参与混合组或其他类型中奖
	if symbol.N2CfgCard.Ifonbetline == 1 {
		return false
	}

	// 不在线上的符号跳过
	return true
}

// processNonContinuousSymbol 处理不连续但在线上的符号
func (g *GenResultBase) processNonContinuousSymbol(
	symbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	// 在线上但不连续的符号（Ifonbetline=1, Ifcontinue=0）
	if symbol.N2CfgCard.Ifonbetline == 1 && symbol.N2CfgCard.Ifcontinue == 0 {
		ctx.singleCnt[symbolIndex] += countNum
		g.updateSingleGroupCount(symbol, symbolIndex, countNum, ctx)
	}

	// 处理混合组符号（无论是否连续）
	if symbol.MixedGroup > 0 {
		for _, v := range symbol.MixedGroupAll {
			// 如果这个组还没有开始计数，跳过
			if ctx.singleGroupCnt[v] == 0 && ctx.groupCnt[v] == 0 {
				continue
			}
			if ctx.lastGroupSymbol[v] != 0 && ctx.lastGroupSymbol[v] != symbolIndex {
				ctx.hasGroup[v] = 1
			}
			if ctx.singleGroupCnt[v] > 0 {
				ctx.singleGroupCnt[v] += countNum
				ctx.singleGroupSymbolCnt[v]++
			}
		}
	}
}

// updateSingleGroupCount 更新单个组计数
func (g *GenResultBase) updateSingleGroupCount(
	symbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	if symbol.MixedGroup <= 0 {
		return
	}

	for _, v := range symbol.MixedGroupAll {
		if ctx.lastGroupSymbol[v] == 0 {
			ctx.lastGroupSymbol[v] = symbolIndex
		}
		if ctx.lastGroupSymbol[v] != symbolIndex {
			ctx.hasGroup[v] = 1
		}
		if ctx.singleGroupCnt[v] == 0 {
			ctx.singleGroupCnt[v] = ctx.groupCnt[v]
			ctx.singleGroupSymbolCnt[v] = ctx.groupSymbolCnt[v]
		}
		ctx.singleGroupCnt[v] += countNum
		ctx.singleGroupSymbolCnt[v]++
	}
}

// processWildSymbol 处理 wild 符号统计
func (g *GenResultBase) processWildSymbol(
	symbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	if symbol.N2CfgCard.Replacefunction == 1 {
		ctx.singleWildCnt[symbolIndex] += countNum
	}
}

// processMultiplier 处理倍数
func (g *GenResultBase) processMultiplier(x, y int, options DealMapResultOptions, ctx *lineContext) {
	if options.MultiBet == nil || x >= len(options.MultiBet) || y >= len(options.MultiBet[x]) {
		return
	}

	if options.MultiAdd {
		ctx.winBet += options.MultiBet[x][y]
	} else {
		ctx.winBet *= options.MultiBet[x][y]
	}
}

// ==================== 连续中奖处理 ====================

// processContinuousWin 处理连续中奖逻辑
func (g *GenResultBase) processContinuousWin(
	i, x, y int,
	balanceMap [][]int,
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	// 获取上一个符号的索引（用于比较）
	lastSymbolIndex := 0
	if ctx.lastSymbol != nil {
		lastSymbolIndex = int(ctx.lastSymbol.N2CfgCard.Cardindex)
	}

	// DEBUG: 打印处理过程
	// fmt.Printf("  [processContinuousWin] i=%d, symbolIndex=%d, lastSymbolIndex=%d, isContinue=%v, winCnt=%d\n",
	// 	i, symbolIndex, lastSymbolIndex, ctx.isContinue, ctx.winCnt)

	if ctx.isContinue {
		// 情况1: 第一个符号
		if ctx.lastSymbol == nil {
			g.handleSameSymbol(x, y, balanceMap, thisSymbol, symbolIndex, countNum, ctx)
		} else if lastSymbolIndex == symbolIndex {
			// 情况2: 与上一个符号相同
			g.handleSameSymbol(x, y, balanceMap, thisSymbol, symbolIndex, countNum, ctx)
		} else {
			// 情况3: 不同符号，检查 wild 替换或混合组
			g.handleDifferentSymbol(i, x, y, balanceMap, thisSymbol, symbolIndex, countNum, ctx)
		}
	}

	// DEBUG: 打印处理后状态
	// fmt.Printf("    -> isContinue=%v, winCnt=%d, winSymbolId=%d\n", ctx.isContinue, ctx.winCnt, ctx.winSymbolId)

	// 处理不连续但在组内的情况
	if !ctx.isContinue {
		g.handleNonContinuousInGroup(i, thisSymbol, symbolIndex, countNum, ctx)
	}

	if !ctx.isContinue && !ctx.inGroup {
		ctx.onLine = false
	}

	ctx.lastSymbol = thisSymbol
}

// handleSameSymbol 处理相同符号或第一个符号
func (g *GenResultBase) handleSameSymbol(
	x, y int,
	balanceMap [][]int,
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	// 第一个符号或相同符号，更新中奖信息
	if ctx.winSymbol == nil {
		// 第一个符号
		ctx.winSymbolId = balanceMap[x][y]
		ctx.winSymbol = thisSymbol
	}

	ctx.winCnt += countNum
	ctx.winSymbolCnt++

	// 统计 wild
	if thisSymbol.N2CfgCard.Replacefunction == 1 {
		ctx.wildSymbolCnt++
		ctx.wildCnt += countNum
	}

	// 处理混合组
	g.handleMixedGroup(thisSymbol, symbolIndex, countNum, ctx)
}

// handleMixedGroup 处理混合组
func (g *GenResultBase) handleMixedGroup(
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	// 检查是否有混合组
	if len(thisSymbol.MixedGroupAll) == 0 {
		return
	}

	if ctx.lastSymbol == nil {
		// 第一个符号，初始化组
		for _, v := range thisSymbol.MixedGroupAll {
			// 跳过无效的组ID
			if v <= 0 {
				continue
			}
			ctx.lastGroupSymbol[v] = symbolIndex
			ctx.winGroup = append(ctx.winGroup, v)
			ctx.groupCnt[v] = countNum
			ctx.groupSymbolCnt[v] = 1
		}
	} else {
		// 后续符号，更新组
		g.updateWinGroup(thisSymbol, symbolIndex, countNum, ctx)
	}
}

// updateWinGroup 更新中奖组
func (g *GenResultBase) updateWinGroup(
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	for j := len(ctx.winGroup) - 1; j >= 0; j-- {
		group := ctx.winGroup[j]
		if containsInt(thisSymbol.MixedGroupAll, group) {
			if ctx.lastGroupSymbol[group] == 0 {
				ctx.lastGroupSymbol[group] = symbolIndex
			}
			if ctx.lastGroupSymbol[group] != symbolIndex {
				ctx.hasGroup[group] = 1
			}
			ctx.groupCnt[group] += countNum
			ctx.groupSymbolCnt[group]++
		} else if ctx.winSymbol != nil && ctx.winSymbol.N2CfgCard.Ifcontinue == 1 {
			ctx.winGroup = append(ctx.winGroup[:j], ctx.winGroup[j+1:]...)
		}
	}
}

// handleDifferentSymbol 处理不同符号（wild替换和混合组逻辑）
func (g *GenResultBase) handleDifferentSymbol(
	i, x, y int,
	balanceMap [][]int,
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	// DEBUG
	// fmt.Printf("  [handleDifferentSymbol] i=%d, thisSymbol=%d, Replacefunction=%d, winSymbol=%v\n",
	// 	i, symbolIndex, thisSymbol.N2CfgCard.Replacefunction, ctx.winSymbolId)

	// 当前符号是 wild，检查是否可以替换之前的中奖符号
	if thisSymbol.N2CfgCard.Replacefunction == 1 {
		// 检查之前的中奖符号是否可被替换
		if ctx.winSymbol != nil && ctx.winSymbol.N2CfgCard.Isreplacable == 1 {
			// Wild 可以替换中奖符号，继续连续
			g.handleCurrentIsWild(thisSymbol, symbolIndex, countNum, ctx)
			return
		}
		// 之前的符号不可被替换，或者之前也是 Wild
		if ctx.winSymbol != nil && ctx.winSymbol.N2CfgCard.Replacefunction == 1 {
			// 两个 Wild 连续
			g.handleCurrentIsWild(thisSymbol, symbolIndex, countNum, ctx)
			return
		}
		// 之前的符号不可被 Wild 替换，中断连续
		ctx.isContinue = false
		return
	}

	// 当前符号不是 wild，检查之前的中奖符号是否是 wild
	if ctx.winSymbol != nil && ctx.winSymbol.N2CfgCard.Replacefunction == 1 {
		// 之前是 Wild，当前符号可被替换则替换 Wild 为当前符号
		if thisSymbol.N2CfgCard.Isreplacable == 1 {
			g.handlePreviousIsWild(i, x, y, balanceMap, thisSymbol, symbolIndex, countNum, ctx)
			return
		}
		// 当前符号不可被替换，中断连续
		ctx.isContinue = false
		return
	}

	// 检查是否在同一个混合组内
	if g.handleMixedGroupDifferentSymbol(thisSymbol, symbolIndex, countNum, ctx) {
		// 在混合组内，更新 winCnt（混合组也需要计数）
		ctx.winCnt += countNum
		ctx.winSymbolCnt++
		return
	}

	// 不能替换，中断连续
	ctx.isContinue = false
}

// handleMixedGroupDifferentSymbol 处理不同符号但在同一混合组的情况
func (g *GenResultBase) handleMixedGroupDifferentSymbol(
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) bool {
	// 检查当前符号是否有混合组
	if len(thisSymbol.MixedGroupAll) == 0 {
		return false
	}

	// 检查当前符号是否与之前的符号在同一个混合组
	foundInGroup := false

	// 如果之前的中奖符号也有混合组，检查是否有交集
	if ctx.winSymbol != nil && len(ctx.winSymbol.MixedGroupAll) > 0 {
		for _, group := range thisSymbol.MixedGroupAll {
			// 跳过无效的组ID
			if group <= 0 {
				continue
			}
			// 检查之前的符号是否也在这个组
			if containsInt(ctx.winSymbol.MixedGroupAll, group) {
				foundInGroup = true

				// 如果这个组还没有在 winGroup 中，添加它
				if !containsInt(ctx.winGroup, group) {
					ctx.winGroup = append(ctx.winGroup, group)
					// 初始化组计数（包括之前的符号）
					ctx.groupCnt[group] = ctx.winCnt
					ctx.groupSymbolCnt[group] = ctx.winSymbolCnt
				}

				// 标记为混合组（有不同符号）
				ctx.hasGroup[group] = 1
				ctx.lastGroupSymbol[group] = symbolIndex

				// 更新组计数
				ctx.groupCnt[group] += countNum
				ctx.groupSymbolCnt[group]++
			}
		}
	}

	return foundInGroup
}

// handleCurrentIsWild 处理当前符号是 wild 的情况
func (g *GenResultBase) handleCurrentIsWild(
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	// 使用中奖符号的 countNum（如果有的话）
	effectiveCountNum := countNum
	if ctx.winSymbol != nil {
		effectiveCountNum = int(ctx.winSymbol.N2CfgCard.CountNum)
		if effectiveCountNum == 0 {
			effectiveCountNum = 1
		}
	}

	ctx.wildSymbolCnt++
	ctx.wildCnt += effectiveCountNum

	// 关键：Wild 替换时也要更新 winCnt（如果中奖符号可被替换）
	if ctx.winSymbol != nil && ctx.winSymbol.N2CfgCard.Isreplacable == 1 {
		// Wild 可以替换中奖符号，更新连续计数
		ctx.winCnt += effectiveCountNum
		ctx.winSymbolCnt++

		// 更新组计数
		for _, group := range ctx.winGroup {
			if ctx.lastGroupSymbol[group] == 0 {
				ctx.lastGroupSymbol[group] = symbolIndex
			}
			if ctx.lastGroupSymbol[group] != symbolIndex {
				ctx.hasGroup[group] = 1
			}
			ctx.groupCnt[group] += effectiveCountNum
			ctx.groupSymbolCnt[group]++
		}
	} else if ctx.winSymbol != nil && ctx.winSymbol.N2CfgCard.Replacefunction == 1 {
		// 两个 wild 连续
		ctx.winCnt += effectiveCountNum
		ctx.winSymbolCnt++

		// 如果当前 wild 可被替换则替换中奖符号
		if thisSymbol.N2CfgCard.Isreplacable == 1 {
			ctx.winSymbol = thisSymbol
			ctx.winSymbolId = symbolIndex
		}
	}
}

// handlePreviousIsWild 处理之前中奖符号是 wild 的情况
func (g *GenResultBase) handlePreviousIsWild(
	i, x, y int,
	balanceMap [][]int,
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	ctx.winGroup = make([]int, 0)
	ctx.winSymbol = thisSymbol
	ctx.winSymbolId = balanceMap[x][y]
	ctx.winCnt = countNum
	ctx.winSymbolCnt = 1

	if thisSymbol.MixedGroup > 0 {
		for _, v := range thisSymbol.MixedGroupAll {
			if ctx.lastGroupSymbol[v] == 0 {
				ctx.lastGroupSymbol[v] = symbolIndex
			}
			if ctx.lastGroupSymbol[v] != symbolIndex {
				ctx.hasGroup[v] = 1
			}
			ctx.winGroup = append(ctx.winGroup, v)
			ctx.groupCnt[v] = (i + 1) * countNum
			ctx.groupSymbolCnt[v] = i + 1
		}
	}
}

// handleNonContinuousInGroup 处理不连续但在组内的情况
func (g *GenResultBase) handleNonContinuousInGroup(
	i int,
	thisSymbol *dbData.FormatCardConfig,
	symbolIndex, countNum int,
	ctx *lineContext,
) {
	if len(ctx.winGroup) == 0 {
		return
	}

	if thisSymbol.N2CfgCard.Replacefunction != 1 && thisSymbol.MixedGroup <= 0 {
		return
	}

	for j := len(ctx.winGroup) - 1; j >= 0; j-- {
		group := ctx.winGroup[j]
		inGroup := thisSymbol.N2CfgCard.Replacefunction == 1 || containsInt(thisSymbol.MixedGroupAll, group)

		if inGroup {
			if ctx.lastGroupSymbol[group] == 0 {
				ctx.lastGroupSymbol[group] = symbolIndex
			}
			if ctx.lastGroupSymbol[group] != symbolIndex {
				ctx.hasGroup[group] = 1
			}
			ctx.groupCnt[group] += countNum
			ctx.groupSymbolCnt[group]++
			ctx.inGroup = true
		} else if ctx.winSymbol != nil && ctx.winSymbol.N2CfgCard.Ifcontinue == 1 {
			ctx.winGroup = append(ctx.winGroup[:j], ctx.winGroup[j+1:]...)
		}
	}
}

// ==================== 最终中奖计算 ====================

// calculateFinalWin 计算最终中奖结果
func (g *GenResultBase) calculateFinalWin(
	ctx *lineContext,
	allSymbol map[int32]*dbData.FormatCardConfig,
	analysis *symbolAnalysis,
	options DealMapResultOptions,
) *lineWinResult {
	result := &lineWinResult{
		holdWinData: make([]int, g.y),
	}

	realWinBet := ctx.winBet
	if realWinBet < 1 {
		realWinBet = 1
	}

	// Step 1: 计算连续符号中奖
	g.calculateContinuousWin(ctx, allSymbol, options, realWinBet, result)

	// Step 2: 计算单个符号中奖（不连续但在线上）
	g.calculateSingleSymbolWin(ctx, allSymbol, options, realWinBet, result)

	// Step 3: 计算混合组中奖
	g.calculateGroupWin(ctx, allSymbol, analysis, options, realWinBet, result)

	// Step 4: 计算单个组中奖
	g.calculateSingleGroupWin(ctx, allSymbol, analysis, options, realWinBet, result)

	return result
}

// calculateContinuousWin 计算连续符号中奖
func (g *GenResultBase) calculateContinuousWin(
	ctx *lineContext,
	allSymbol map[int32]*dbData.FormatCardConfig,
	options DealMapResultOptions,
	realWinBet int,
	result *lineWinResult,
) {
	// 检查是否有中奖符号
	if ctx.winSymbol == nil || ctx.winCnt <= 0 {
		return
	}

	// 计算倍数
	winSymbolBet := realWinBet
	if ctx.winSymbol.N2CfgCard.Replacefunction == 1 && !options.WildNeedMulti {
		winSymbolBet = 1
	}
	if containsIntSlice(options.NoMultiSymbol, ctx.winSymbolId) {
		winSymbolBet = 1
	}

	// winCnt 已经在 processContinuousWin 中计算好了（包含了 Wild 替换）
	// 不需要再加 wildCnt
	winCnt := ctx.winCnt

	// DEBUG
	// fmt.Printf("  [calculateContinuousWin] winSymbolId=%d, winCnt=%d, Conditionoddsseq=%v\n",
	// 	ctx.winSymbolId, winCnt, ctx.winSymbol.Conditionoddsseq)

	// 计算中奖金额 - 检查 Conditionoddsseq 数组
	if winCnt > 0 && winCnt < len(ctx.winSymbol.Conditionoddsseq) {
		odds := ctx.winSymbol.Conditionoddsseq[winCnt]
		if odds > 0 {
			coins := odds * winSymbolBet
			if coins > result.coins {
				result.coins = coins
				result.count = winCnt
				result.symbolId = ctx.winSymbolId
			}
		}
	}
}

// calculateSingleSymbolWin 计算单个符号中奖
func (g *GenResultBase) calculateSingleSymbolWin(
	ctx *lineContext,
	allSymbol map[int32]*dbData.FormatCardConfig,
	options DealMapResultOptions,
	realWinBet int,
	result *lineWinResult,
) {
	for idInt, cnt := range ctx.singleCnt {
		symbol, exists := allSymbol[int32(idInt)]
		if !exists {
			continue
		}

		totalCnt := cnt
		// 加上 wild 数量
		if symbol.N2CfgCard.Isreplacable != 0 {
			for kid, wcnt := range ctx.singleWildCnt {
				if kid != idInt {
					totalCnt += wcnt
				}
			}
		}

		if totalCnt >= len(symbol.Conditionoddsseq) || symbol.Conditionoddsseq[totalCnt] <= 0 {
			continue
		}

		tempWinBet := realWinBet
		if (symbol.N2CfgCard.Replacefunction == 1 && !options.WildNeedMulti) || containsIntSlice(options.NoMultiSymbol, idInt) {
			tempWinBet = 1
		}

		coins := symbol.Conditionoddsseq[totalCnt] * tempWinBet
		if coins > result.coins {
			result.coins = coins
			result.count = totalCnt
			result.symbolId = idInt
		}
	}
}

// calculateGroupWin 计算混合组中奖
func (g *GenResultBase) calculateGroupWin(
	ctx *lineContext,
	allSymbol map[int32]*dbData.FormatCardConfig,
	analysis *symbolAnalysis,
	options DealMapResultOptions,
	realWinBet int,
	result *lineWinResult,
) {
	for gInt, cnt := range ctx.groupCnt {
		// 混合组中奖：需要有不同符号（hasGroup=1）才算混合组中奖
		// 如果只有相同符号，应该走普通中奖逻辑
		if ctx.hasGroup[gInt] != 1 {
			continue
		}

		coins, count := g.calculateMixedGroupOdds(gInt, cnt, allSymbol, analysis, options, realWinBet)
		if coins > result.coins {
			result.coins = coins
			result.count = count
			result.symbolId = -1 * gInt
		}
	}
}

// calculateSingleGroupWin 计算单个组中奖
func (g *GenResultBase) calculateSingleGroupWin(
	ctx *lineContext,
	allSymbol map[int32]*dbData.FormatCardConfig,
	analysis *symbolAnalysis,
	options DealMapResultOptions,
	realWinBet int,
	result *lineWinResult,
) {
	for gInt, cnt := range ctx.singleGroupCnt {
		if ctx.hasGroup[gInt] != 1 {
			continue
		}

		coins, count := g.calculateMixedGroupOdds(gInt, cnt, allSymbol, analysis, options, realWinBet)
		if coins > result.coins {
			result.coins = coins
			result.count = count
			result.symbolId = -1 * gInt
		}
	}
}

// calculateMixedGroupOdds 计算混合组赔率
func (g *GenResultBase) calculateMixedGroupOdds(
	groupId, cnt int,
	allSymbol map[int32]*dbData.FormatCardConfig,
	analysis *symbolAnalysis,
	options DealMapResultOptions,
	realWinBet int,
) (coins, count int) {
	symbolIds := analysis.groupSymbolIds[groupId]
	if len(symbolIds) == 0 {
		return 0, 0
	}

	sid := int32(symbolIds[0])
	symbol, exists := allSymbol[sid]
	if !exists {
		return 0, 0
	}

	// 检查是否需要倍数
	needMulti := true
	for _, id := range symbolIds {
		sym := allSymbol[int32(id)]
		if sym != nil {
			if (sym.N2CfgCard.Replacefunction == 1 && !options.WildNeedMulti) || containsIntSlice(options.NoMultiSymbol, id) {
				needMulti = false
				break
			}
		}
	}

	tempWinBet := realWinBet
	if !needMulti {
		tempWinBet = 1
	}

	// 获取赔率
	odds := 0
	if mixedOdds, exists := symbol.MixedOdds[groupId]; exists {
		if cnt > len(mixedOdds)-1 {
			odds = mixedOdds[len(mixedOdds)-1]
		} else if cnt >= 0 && cnt < len(mixedOdds) {
			odds = mixedOdds[cnt]
		}
	}

	return odds * tempWinBet, cnt
}

// ==================== 工具函数 ====================

// GetRealSymbolId 获取真实符号ID（子类可重写）
func (g *GenResultBase) GetRealSymbolId(id int) int {
	return id
}

// containsInt 检查切片是否包含指定整数
func containsInt(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// containsIntSlice 检查切片是否包含指定整数
func containsIntSlice(slice []int, val int) bool {
	for _, v := range slice {
		if v == val {
			return true
		}
	}
	return false
}

// ==================== 随机符号矩阵生成 ====================

// ReelConfig 卷轴配置
type ReelConfig struct {
	Symbols []int32
	Weights []int32
}

// ReelMatrixConfig 卷轴矩阵配置
type ReelMatrixConfig struct {
	Rows        int
	Reels       []ReelConfig
	UseWeighted bool
}

// GenerateReelBasedMatrix 根据卷轴配置生成随机符号矩阵
func (g *GenResultBase) GenerateReelBasedMatrix(config ReelMatrixConfig) ([][]int32, error) {
	if config.Rows <= 0 {
		return nil, fmt.Errorf("invalid rows: %d", config.Rows)
	}
	if len(config.Reels) == 0 {
		return nil, fmt.Errorf("reels configuration cannot be empty")
	}

	for i, reel := range config.Reels {
		if len(reel.Symbols) == 0 {
			return nil, fmt.Errorf("reel %d symbols cannot be empty", i)
		}
		if config.UseWeighted && len(reel.Weights) != len(reel.Symbols) {
			return nil, fmt.Errorf("reel %d weights length mismatch", i)
		}
	}

	cols := len(config.Reels)
	matrix := make([][]int32, config.Rows)
	for i := range matrix {
		matrix[i] = make([]int32, cols)
	}

	for col := 0; col < cols; col++ {
		reel := config.Reels[col]
		for row := 0; row < config.Rows; row++ {
			if config.UseWeighted {
				matrix[row][col] = g.selectSymbolWithWeight(reel.Symbols, reel.Weights)
			} else {
				matrix[row][col] = g.selectSymbolUniform(reel.Symbols)
			}
		}
	}

	return matrix, nil
}

// selectSymbolUniform 均匀随机选择符号
func (g *GenResultBase) selectSymbolUniform(symbols []int32) int32 {
	if len(symbols) == 0 {
		return 0
	}
	return symbols[g.RandomInt(0, len(symbols)-1)]
}

// selectSymbolWithWeight 使用权重随机选择符号
func (g *GenResultBase) selectSymbolWithWeight(symbols []int32, weights []int32) int32 {
	if len(symbols) == 0 {
		return 0
	}

	var totalWeight int32
	for _, w := range weights {
		totalWeight += w
	}

	randomValue := int32(g.RandomInt(0, int(totalWeight)-1))
	var cumulative int32
	for i, w := range weights {
		cumulative += w
		if randomValue < cumulative {
			return symbols[i]
		}
	}
	return symbols[len(symbols)-1]
}

// GenerateReelBasedMatrixSimple 简化版本
func (g *GenResultBase) GenerateReelBasedMatrixSimple(rows, cols int, symbols []int32) ([][]int32, error) {
	reels := make([]ReelConfig, cols)
	for i := 0; i < cols; i++ {
		reels[i] = ReelConfig{Symbols: symbols}
	}
	return g.GenerateReelBasedMatrix(ReelMatrixConfig{Rows: rows, Reels: reels})
}

// GenerateReelBasedMatrixFromReels 从卷轴序列生成矩阵
func (g *GenResultBase) GenerateReelBasedMatrixFromReels(reels [][]int32, rows int) ([][]int32, []int, error) {
	if len(reels) == 0 {
		return nil, nil, fmt.Errorf("reels cannot be empty")
	}

	cols := len(reels)
	matrix := make([][]int32, rows)
	for i := range matrix {
		matrix[i] = make([]int32, cols)
	}

	posInSequence := make([]int, cols)
	for col := 0; col < cols; col++ {
		reelLen := len(reels[col])
		if reelLen == 0 {
			return nil, nil, fmt.Errorf("reel %d is empty", col)
		}

		startPos := g.RandomInt(0, reelLen-1)
		posInSequence[col] = startPos

		for row := 0; row < rows; row++ {
			pos := (startPos + row) % reelLen
			matrix[row][col] = reels[col][pos]
		}
	}

	return matrix, posInSequence, nil
}
