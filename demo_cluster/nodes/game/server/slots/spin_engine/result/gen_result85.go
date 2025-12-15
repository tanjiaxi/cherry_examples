/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-12 11:16:07
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package result

import (
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

type ReelsConfig85 struct {
	ReelsConfigBase
	SpLine   []int `json:"spLine"`
	SpReward []int `json:"spReward"`
}
type GenResult85 struct {
	GenResultBase
	ReelsConfig85
}

func NewGenResult85(genResultBase GenResultBase) *GenResult85 {
	genResultBase.x = 3
	genResultBase.y = 3
	return &GenResult85{
		GenResultBase: genResultBase,
		ReelsConfig85: ReelsConfig85{},
	}
}

// allMap [][]int,
// posInSequence []int,
// allLines *[][]dbData.Line,
// allSymbol map[int32]*dbData.FormatCardConfig,
// bet int,
// options GetResultOptions,
func (g *GenResult85) GenResult(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig *gameModel.N2CfgRoomlist) (*pb.SpinResult, error) {
	// posInSequence := make([]int, g.y)
	g.SetSeedSave()
	allLines, err := g.GetAllLinesData(int(roomCongfig.LineMax))
	if err != nil {
		return nil, err
	}

	allSymbolg, err := g.GetRoomSymbolCfg()
	if err != nil {
		return nil, err
	}
	if err := g.UnmarshalReelJsonObj(&g.ReelsConfig85); err != nil {
		return nil, err
	}
	allBalanceMap, posInSequence, err := g.GetGameMap(&g.ReelsConfigBase)
	if err != nil {
		return nil, err
	}
	allBalanceMap = [][]int{
		{1, 10, 10},
		{4, 5, 5},
		{1, 10, 10},
	}
	options := GetResultOptions{}
	// 生成结果
	g.GetResult(allBalanceMap, posInSequence, allLines, allSymbolg, bet, options)
	return nil, nil
}
