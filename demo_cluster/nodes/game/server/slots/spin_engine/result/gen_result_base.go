/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-19 16:02:25
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 关卡基础类
 */
package result

import (
	"encoding/json"
	"slices"

	clog "github.com/cherry-game/cherry/logger"
	slotsConfigCache "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	dbData "github.com/cherry-game/examples/demo_cluster/internal/db" //具体数据
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
	"github.com/tidwall/gjson"
)

type lines struct {
	id        int     //那一条线
	symbol    int     //那个id中的
	win       int     //这条线赢钱
	positions [][]int //线的位置，[[1,2,3]],表示第一列的1，2，3位置
}
type winsInfo struct {
	win      int
	winType  int
	lines    []lines
	kindType int
}

// getResult 方法的返回值
type GetRetDataInfo struct {
	posInReels []int    // 停轴位子
	results    [][]int  // 棋盘数据
	holdWins   []int    // holdwin数据
	stopInfo   [][]int  // stop效果数据
	win        int      // 线上总赢钱
	winInfo    winsInfo // 中奖信息
}
type SingeReelsConfig struct {
	ReelsConfig      json.RawMessage `json:"reels_config"`
	SymbolsSequences [][][]int32     `json:"symbolsSequences"`
}
type ReelsConfigBase struct {
	SymbolsSequencesWeight      []int `json:"symbolsSequencesWeight"`
	FreespinSequencesStartIndex []int `json:"freespinSequencesStartIndex"`
	FreespinSequencesWeight     []int `json:"freespinSequencesWeight"`
}
type GenResultBase struct {
	roomDataInfo      *slotsModel.RoomDataInfo
	reelJsonObj       *SingeReelsConfig
	roomConfig        slotsConfigCache.IRoomListConfig
	roomId            int32
	ruleId            int32
	bet               int
	stage             int  //状态bonus，freeSpin，
	stageType         int  //是否是reSpin
	needSave          bool //是否需要保存种子
	x                 int
	y                 int
	remedyState       int
	randomNext        uint32
	reelsStart        int
	reelLevel         int
	reelsIdx          int
	winTotalMoney     int
	winRealTotalMoney int
}

func (g *GenResultBase) OnInit(roomId, ruleId int32, bet int, roomDataInfo *slotsModel.RoomDataInfo) {
	g.roomDataInfo = roomDataInfo
	g.roomId = roomId
	g.ruleId = ruleId
	g.bet = bet

}
func NewGenResultBase() *GenResultBase {
	return &GenResultBase{}
}
func (g *GenResultBase) GenResult(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig slotsConfigCache.IRoomListConfig) (*pb.SpinResult, error) {
	return nil, nil
}

// 获取棋盘
func (g *GenResultBase) GetGameMap(reelsConfigBase *ReelsConfigBase) (allBalanceMap [][]int, posInSequence []int, err error) {
	balanceMap := make([][]int, g.x)

	// 2. 遍历外层切片，为每一行分配长度为 y 的内层切片
	for i := range balanceMap {
		balanceMap[i] = make([]int, g.y)
	}
	// allBalanceMap = make([][][]int, 1, 1)
	// symbolsSequenceWeight := g.reelJsonObj.Reelsequences.Config.ReelsConfig.UnmarshalJSON() //  reelJsonObj['symbolsSequencesWeight'];
	sequencesStartIndex := []int{}
	symbolsSequenceWeight := reelsConfigBase.SymbolsSequencesWeight
	for i := 0; i < len(symbolsSequenceWeight); i++ {
		sequencesStartIndex = append(sequencesStartIndex, i+1)
	}
	if g.stage == slotsModel.FREE_SPIN {
		sequencesStartIndex = reelsConfigBase.FreespinSequencesStartIndex
		symbolsSequenceWeight = reelsConfigBase.FreespinSequencesWeight
	}
	g.reelsIdx, err = g.randomReel(symbolsSequenceWeight, sequencesStartIndex)
	reels := g.formatReel(reelsConfigBase, g.reelJsonObj.SymbolsSequences[g.reelsIdx])
	for {
		for i := 0; i < g.y; i++ {
			len := len(reels[i])
			tempPos := g.RandomInt(0, len)
			for j := 0; j < g.x; j++ {
				pos := int(tempPos+j) % len
				balanceMap[j][i] = int(reels[i][pos])
			}
			posInSequence = append(posInSequence, tempPos)
		}
		if !g.skipMap() {
			break
		}
	}
	allBalanceMap = balanceMap
	// allBalanceMap = append(allBalanceMap, balanceMap)
	return allBalanceMap, posInSequence, nil
}
func (g *GenResultBase) skipMap() bool {
	return false
}
func (g *GenResultBase) randomReel(symbolsSequenceWeight, sequencesStartIndex []int) (int, error) {
	reelsIdx := 0
	var weightTotal float64 = 0
	weightRange := make([]float64, len(symbolsSequenceWeight))
	for index, v := range symbolsSequenceWeight {
		step := v
		weightTotal += float64(step)
		weightRange[index] = weightTotal
	}
	rand := g.RandomFloat(0, 1) * weightTotal
	for i := 0; i < len(symbolsSequenceWeight); i++ {
		if rand < weightRange[i] {
			reelsIdx = sequencesStartIndex[i] - 1
			break
		}
	}
	return reelsIdx, nil
}
func (g *GenResultBase) formatReel(reelsConfigBase *ReelsConfigBase, reels [][]int32) [][]int32 {
	return reels
}
func (g *GenResultBase) GetWinType() string {
	return ""
}
func (g *GenResultBase) GetWinLines() error {
	return nil
}
func (g *GenResultBase) GetAllLinesData(lineNum int) (*[][]dbData.Line, error) {
	allLines, err := slotsConfigCache.GetInstance().GetFromatLineIds(g.x, g.y)
	if err != nil {
		return nil, err
	}
	lines := make([][]dbData.Line, 0, lineNum)
	for i := 0; i < lineNum; i++ {
		id := allLines.Ids[i]
		lineData, err := slotsConfigCache.GetInstance().GetFromatLines(g.x, g.y, id)
		if err != nil {
			return nil, err
		}
		lines = append(lines, lineData.LinesArr)
	}

	return &lines, err
}
func (g *GenResultBase) SetSeedSave() {
	if g.stage != slotsModel.NORMAL || g.stageType == slotsModel.RE_SPIN_NORMAL {
		g.needSave = false
	} else {
		g.needSave = true
	}
}
func (g *GenResultBase) GetRoomSymbolCfg() (map[int32]*dbData.FormatCardConfig, error) {
	symbols, err := slotsConfigCache.GetInstance().GetCardConfig(g.ruleId)
	if err != nil {
		return nil, err
	}
	return symbols, nil
}
func (g *GenResultBase) GetWinMoneyType(roomConfig slotsConfigCache.IRoomListConfig) []int {
	return []int{
		0, int(roomConfig.GetSfxMidwin()), int(roomConfig.GetSfxBigwin()), int(roomConfig.GetBigthreshold()), int(roomConfig.GetMegathreshold()), int(roomConfig.GetSuperthreshold()),
	}
}
func (g *GenResultBase) InitReelLevel(roomDataInfo *slotsModel.RoomDataInfo, redBlackFluctuationVal int) int {
	reelLevel := 3
	if roomDataInfo.StageType == slotsModel.RE_SPIN_ING || roomDataInfo.Stage == slotsModel.FREE_SPIN || roomDataInfo.FreeSpinNum > 0 {
		if roomDataInfo.LastReelLevel > 0 {
			reelLevel = roomDataInfo.LastReelLevel
		}
	} else if redBlackFluctuationVal > 0 {
		reelLevel = redBlackFluctuationVal
	}
	return reelLevel
}
func (g *GenResultBase) GetReelObj(reelRoonConfig *logicGameModel.N2CfgReelRoom, reelLevel int, ruleId int32) *SingeReelsConfig {
	sequenceIdx := 1
	reelsEnd := 0
	reelsStart := 0
	reelsequences := reelRoonConfig.Reelsequences
	reelsStartArr := reelsequences.Config.ReelsStart
	reelsWeight := reelsequences.Config.ReelsWeight[reelLevel-1]
	weightRange := make([]float64, len(reelsWeight), len(reelsWeight))
	var weightTotal float64 = 0
	for index, v := range reelsWeight {
		step := v * 100
		weightTotal += step
		weightRange[index] = weightTotal
	}
	rand := g.RandomFloat(0, 1) * weightTotal
	for i := 0; i < len(reelsWeight); i++ {
		if rand < weightRange[i] {
			sequenceIdx = i
			reelsStart = int(reelsStartArr[i]) - 1
			if i < 2 {
				reelsEnd = int(reelsStartArr[i+1] - 1)
			} else {
				reelsEnd = len(reelsequences.SymbolsSequences)
			}
			break
		}
	}
	g.reelsStart = reelsStart
	return genReelObj(&reelsequences, reelsStart, reelsEnd, sequenceIdx)
}
func genReelObj(reelJson *logicGameModel.SlotData, reelsStart, reelsEnd int, sequenceIdx int) *SingeReelsConfig {
	singeReelsConfig := SingeReelsConfig{
		ReelsConfig: reelJson.Config.ReelsConfig[sequenceIdx],
	}
	reelsAll := make([][][]int32, 0)
	for i := 0; i < reelsEnd-reelsStart; i++ {
		reelsAll = append(reelsAll, reelJson.SymbolsSequences[reelsStart+i])
	}
	singeReelsConfig.SymbolsSequences = reelsAll
	return &singeReelsConfig
}
func (g *GenResultBase) Random() uint32 {
	// 核心逻辑：
	// next = (previous * a + c) % m
	// 这里利用 uint32 的溢出特性，等价于 % 2^32
	// a = 1664525, c = 1013904223
	g.randomNext = g.randomNext*1664525 + 1013904223
	return g.randomNext
}
func (g *GenResultBase) RandomInt(min, max int) int {
	// 计算范围长度
	// 注意：这里假设 max >= min
	rangeLen := max - min + 1
	// 获取一个随机数
	rnd := g.Random()
	// 1. rnd 是 uint32，恒为正，不需要 Abs
	// 2. 将 rangeLen 转为 uint32 进行取模 (如果 rangeLen 很大，需注意类型匹配，通常游戏逻辑里int够用)
	// 3. 结果转回 int 加 min
	return int(rnd%uint32(rangeLen)) + min
}
func (g *GenResultBase) RandomFloat(min, max float64) float64 {
	// JS原版: this.random() / LCG_M ...
	// Go实现:
	// LCG_M 就是 2^32 = 4294967296
	const LCG_M_FLOAT = 4294967296.0

	// 1. 归一化到 [0, 1)
	r := float64(g.Random()) / LCG_M_FLOAT

	// 2. 缩放并平移
	result := (max-min)*r + min

	// 在 Go 里 uint32 转 float64 肯定是正数，除非 max < min，否则不需要 Abs
	return result
	// return math.Abs(result)
}
func (g *GenResultBase) BeforeSpin(redBlackFluctuationVal int, roomId, ruleId int32) error {
	n2CfgRoomlist, err := slotsConfigCache.GetInstance().GetRoomConfig(roomId)
	if err != nil || n2CfgRoomlist == nil {
		return err
	}
	allReelJsonObj, err := slotsConfigCache.GetInstance().GetN2CfgReelRoom(ruleId)
	if err != nil || allReelJsonObj == nil {
		return err
	}
	//获取轴
	reelLevel := g.InitReelLevel(g.roomDataInfo, redBlackFluctuationVal)
	//获取具体的轴配置
	reelJsonObj := g.GetReelObj(allReelJsonObj, reelLevel, ruleId)
	g.setResultBaseInfo(reelJsonObj, n2CfgRoomlist, reelLevel)
	return nil
}
func (g *GenResultBase) setResultBaseInfo(singeReelsConfig *SingeReelsConfig, n2CfgRoomlist slotsConfigCache.IRoomListConfig, reelLevel int) {
	g.reelJsonObj = singeReelsConfig
	g.roomConfig = n2CfgRoomlist
}

// 方案一：每关需要定义结构体
func (g *GenResultBase) UnmarshalReelJsonObj(reelsConfigInfo interface{}) error {
	return json.Unmarshal(g.reelJsonObj.ReelsConfig, reelsConfigInfo)
}

// 方案二：每关不需要定义结构体，使用 gjson库
func (g *GenResultBase) UnmarshalReelJsonObj2() {
	// type ReelsConfig86 struct {
	// 	SymbolsSequencesWeight      []int `json:"symbolsSequencesWeight"`
	// 	FreespinSequencesStartIndex []int `json:"freespinSequencesStartIndex"`
	// 	FreespinSequencesWeight     []int `json:"freespinSequencesWeight"`
	// 	SpLine                      []int `json:"spLine"`
	// 	SpReward                    []int `json:"spReward"`
	// }
	//类似 list := value.Array()[0].Array()
	SymbolsSequencesWeight := gjson.GetBytes(g.reelJsonObj.ReelsConfig, "SymbolsSequencesWeight").Array()
	clog.Info("SymbolsSequencesWeight:%v", SymbolsSequencesWeight)
}
func (g *GenResultBase) getResult() *SingeReelsConfig {
	return g.reelJsonObj
}
func (g *GenResultBase) covertWinType(winBet float64, winMoneyType []int) int {
	moneyType := 0
	if winBet > 0 {
		for i := len(winMoneyType) - 1; i >= 0; i-- {
			if winBet >= float64(winMoneyType[i]) {
				moneyType = i + 1
				break
			}
		}
	}
	return moneyType
}
func (g *GenResultBase) toPbResulut() {

}
func (g *GenResultBase) GetRetData(allMap [][]int, posInSequence []int, allLines *[][]dbData.Line, allSymbol map[int32]*dbData.FormatCardConfig, bet int, getResultInfo *GetResultInfo) (*GetRetDataInfo, error) {
	response := &GetRetDataInfo{
		posInReels: make([]int, 0),
		results:    make([][]int, 0),
		holdWins:   make([]int, 0),
		stopInfo:   make([][]int, 0),
		win:        0,
		winInfo:    winsInfo{},
	}
	response.posInReels = posInSequence
	response.results = allMap
	response.holdWins = g.getHoldWinData(allMap)
	response.stopInfo = g.getNormalStopInfo(allMap)
	totalWin, allWinLines := g.formatLineResult(allMap, allLines, allSymbol, bet, getResultInfo)
	response.win = totalWin
	betCost := int(g.roomConfig.GetBetbaseamount())
	winOdds := float64(totalWin / (bet * betCost))
	winType := g.covertWinType(winOdds, g.GetWinMoneyType(g.roomConfig))
	response.winInfo = winsInfo{
		win:      totalWin,
		lines:    allWinLines,
		kindType: winType,
	}
	return response, nil
}
func (g *GenResultBase) formatLineResult(allMap [][]int, allLines *[][]dbData.Line, allSymbol map[int32]*dbData.FormatCardConfig, bet int, getResultInfo *GetResultInfo) (totalWin int, allWinLines []lines) {
	for index, linesItem := range *allLines {
		if getResultInfo.CoinsPerLines[index] > 0 {

			// TODO 这里没有排出jp的
			lines := lines{
				id:        index,
				symbol:    getResultInfo.WinSymbolPerLines[index],
				positions: make([][]int, 0),
			}
			var winSymol = &dbData.FormatCardConfig{}
			winGroup := 0
			if getResultInfo.WinSymbolPerLines[index] > 0 {
				winSymol = allSymbol[int32(getResultInfo.WinSymbolPerLines[index])]
			} else {
				winGroup = -getResultInfo.WinCountPerLines[index]
			}
			positiionsAll := [][]int{}
			winM := getResultInfo.CoinsPerLines[index] * bet
			tempWinCountPerLines := make([]int, len(getResultInfo.WinCountPerLines))
			copy(getResultInfo.WinCountPerLines, tempWinCountPerLines)
			for k := 0; k < g.y; k++ {
				position := []int{}
				x := linesItem[k].X
				y := linesItem[k].Y
				thisSymbol := allSymbol[int32(allMap[x][y])]
				if tempWinCountPerLines[index] > 0 {
					if winGroup == 0 {
						if winSymol.Ifcontinue == 0 {
							if int(winSymol.ID) == allMap[x][y] || (thisSymbol.Replacefunction == 1 && winSymol.Isreplacable == 1) {
								tempWinCountPerLines[index]--
								position = append(position, x)
							}
						} else {
							tempWinCountPerLines[index]--
							position = append(position, x)
						}
					} else {
						thisSymbol = allSymbol[int32(allMap[x][y])]
						if slices.Contains(thisSymbol.MixedGroupAll, winGroup) || thisSymbol.Isreplacable == 1 {
							tempWinCountPerLines[index]--
							position = append(position, x)
						}
					}
				}
				positiionsAll = append(positiionsAll, position)
			}
			lines.win = winM
			lines.positions = positiionsAll
			allWinLines = append(allWinLines, lines)
			totalWin += winM
		}
	}
	return totalWin, allWinLines
}
func (g *GenResultBase) getHoldWinData(allMap [][]int) []int {
	return nil
}
func (g *GenResultBase) getNormalStopInfo(allMap [][]int) [][]int {
	return nil
}

// SpinResultData 用于收集多棋盘的spin结果数据
type SpinResultData struct {
	AllMaps       [][][]int  // 全部的棋盘
	PosInReelsAll [][]int    // 全部的停轴信息
	HoldWinsAll   [][]int    // 全部的holdwin信息
	StopInfoAll   [][][]int  // 全部的stop信息
	WinInfoAll    []winsInfo // 全部的中奖信息
	ReelsArrayId  []int32    // 使用的预设轴id
	FixedBet      int64      // 固定倍率
}

// ConvertToPbSpinResult 将spin结果数据转换为pb.SpinResult
func (g *GenResultBase) ConvertToPbSpinResult(data *SpinResultData) *pb.SpinResult {
	result := &pb.SpinResult{
		ReelsArrayId: data.ReelsArrayId,
		FixedBet:     data.FixedBet,
	}

	// 转换 PosInReels
	result.PosInReels = g.convertPosInReels(data.PosInReelsAll)

	// 转换 Results (棋盘数据)
	result.Results = g.convertResults(data.AllMaps)

	// 转换 HoldWins
	result.HoldWins = g.convertHoldWins(data.HoldWinsAll)

	// 转换 StopInfo
	result.StopInfo = g.convertStopInfo(data.StopInfoAll)

	// 转换 WinInfo
	result.WinInfo = g.convertWinInfo(data.WinInfoAll)

	return result
}

// convertPosInReels 转换停轴位置数据
// 输入: [][]int - 每个棋盘的停轴位置
// 输出: []*pb.Int32Array1 - 每个元素是一个棋盘的停轴位置数组
func (g *GenResultBase) convertPosInReels(posInReelsAll [][]int) []*pb.Int32Array1 {
	if len(posInReelsAll) == 0 {
		return nil
	}

	result := make([]*pb.Int32Array1, len(posInReelsAll))
	for i, positions := range posInReelsAll {
		arr := make([]int32, len(positions))
		for j, pos := range positions {
			arr[j] = int32(pos)
		}
		result[i] = &pb.Int32Array1{Array: arr}
	}
	return result
}

// convertResults 转换棋盘数据
// 输入: [][][]int - 多个棋盘，每个棋盘是二维数组
// 输出: []*pb.Int32Array2 - 每个元素是一个棋盘的二维数组
func (g *GenResultBase) convertResults(allMaps [][][]int) []*pb.Int32Array2 {
	if len(allMaps) == 0 {
		return nil
	}

	result := make([]*pb.Int32Array2, len(allMaps))
	for i, mapData := range allMaps {
		rows := make([]*pb.Int32Array1, len(mapData))
		for j, row := range mapData {
			arr := make([]int32, len(row))
			for k, val := range row {
				arr[k] = int32(val)
			}
			rows[j] = &pb.Int32Array1{Array: arr}
		}
		result[i] = &pb.Int32Array2{Array: rows}
	}
	return result
}

// convertHoldWins 转换holdwin数据
// 输入: [][]int - 每个棋盘的holdwin数据
// 输出: []*pb.Int32Array1 - 每个元素是一个棋盘的holdwin数组
func (g *GenResultBase) convertHoldWins(holdWinsAll [][]int) []*pb.Int32Array1 {
	if len(holdWinsAll) == 0 {
		return nil
	}

	result := make([]*pb.Int32Array1, len(holdWinsAll))
	for i, holdWins := range holdWinsAll {
		if holdWins == nil {
			result[i] = &pb.Int32Array1{Array: []int32{}}
			continue
		}
		arr := make([]int32, len(holdWins))
		for j, val := range holdWins {
			arr[j] = int32(val)
		}
		result[i] = &pb.Int32Array1{Array: arr}
	}
	return result
}

// convertStopInfo 转换stop信息
// 输入: [][][]int - 每个棋盘的stop信息(二维数组)
// 输出: []*pb.Int32Array2 - 每个元素是一个棋盘的stop信息
func (g *GenResultBase) convertStopInfo(stopInfoAll [][][]int) []*pb.Int32Array2 {
	if len(stopInfoAll) == 0 {
		return nil
	}

	result := make([]*pb.Int32Array2, len(stopInfoAll))
	for i, stopInfo := range stopInfoAll {
		if stopInfo == nil {
			result[i] = &pb.Int32Array2{Array: []*pb.Int32Array1{}}
			continue
		}
		rows := make([]*pb.Int32Array1, len(stopInfo))
		for j, row := range stopInfo {
			arr := make([]int32, len(row))
			for k, val := range row {
				arr[k] = int32(val)
			}
			rows[j] = &pb.Int32Array1{Array: arr}
		}
		result[i] = &pb.Int32Array2{Array: rows}
	}
	return result
}

// convertWinInfo 转换中奖信息
// 输入: []winsInfo - 每个棋盘的中奖信息
// 输出: []*pb.SpinResult_WinInfo - 每个元素是一个棋盘的中奖信息
func (g *GenResultBase) convertWinInfo(winInfoAll []winsInfo) []*pb.SpinResult_WinInfo {
	if len(winInfoAll) == 0 {
		return nil
	}

	result := make([]*pb.SpinResult_WinInfo, len(winInfoAll))
	for i, winInfo := range winInfoAll {
		pbWinInfo := &pb.SpinResult_WinInfo{
			Type:     g.convertWinType(winInfo.winType),
			Win:      int32(winInfo.win),
			KindType: int32(winInfo.kindType),
			Lines:    g.convertLineInfos(winInfo.lines),
		}
		result[i] = pbWinInfo
	}
	return result
}

// convertWinType 转换中奖类型
func (g *GenResultBase) convertWinType(winType int) pb.WinType {
	switch winType {
	case 1:
		return pb.WinType_WIN_MINOR
	case 2:
		return pb.WinType_WIN_MEDIUM
	case 3:
		return pb.WinType_WIN_BIG
	case 4:
		return pb.WinType_WIN_MEGA
	case 5:
		return pb.WinType_WIN_EPIC
	case 6:
		return pb.WinType_WIN_SUPER
	default:
		return pb.WinType_WIN_NONE
	}
}

// convertLineInfos 转换中奖线信息
func (g *GenResultBase) convertLineInfos(linesData []lines) []*pb.SpinResult_WinInfo_LineInfo {
	if len(linesData) == 0 {
		return nil
	}

	result := make([]*pb.SpinResult_WinInfo_LineInfo, len(linesData))
	for i, line := range linesData {
		lineInfo := &pb.SpinResult_WinInfo_LineInfo{
			Id:        int32(line.id),
			Win:       int64(line.win),
			Symbol:    int32(line.symbol),
			Positions: g.convertPositions(line.positions),
		}
		result[i] = lineInfo
	}
	return result
}

// convertPositions 转换位置信息
// 输入: [][]int - 每列的中奖位置
// 输出: []*pb.Int32Array1 - 每个元素是一列的位置数组
func (g *GenResultBase) convertPositions(positions [][]int) []*pb.Int32Array1 {
	if len(positions) == 0 {
		return nil
	}

	result := make([]*pb.Int32Array1, len(positions))
	for i, pos := range positions {
		arr := make([]int32, len(pos))
		for j, val := range pos {
			arr[j] = int32(val)
		}
		result[i] = &pb.Int32Array1{Array: arr}
	}
	return result
}
func (g *GenResultBase) formatReels() ([]int32, error) {
	reelsIds := []int32{int32(g.reelsIdx + g.reelsStart)}
	return reelsIds, nil
}

// 整理roomdata数据
func (g *GenResultBase) AfterSpin(roomDataInfo *slotsModel.RoomDataInfo) (*slotsModel.RoomDataInfo, error) {
	return nil, nil
}
