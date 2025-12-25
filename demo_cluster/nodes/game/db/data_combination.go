/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-01 23:24:04
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 15:57:42
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/core/data_core.go
 * @Description:各种数据组合逻辑
 */
package db

import (
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	dbData "github.com/cherry-game/examples/demo_cluster/internal/db"                        //具体数据
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model" //具体数据
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

func FormatUserBetArr(levelConfig *dbData.FormatLevelConfig, roomId int32, costCoinsM float64, schema string) ([]int64, int, int, error) {
	n2CfgRoomlist, err := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	if err != nil {
		return nil, 0, 0, err
	}
	bets := getUserBaseBets(levelConfig, costCoinsM)
	betArr2 := make([]int64, 0, len(bets))
	for _, v := range bets {
		betArr2 = append(betArr2, v*int64(n2CfgRoomlist.GetBetStakegear()))
	}
	defaultBet := int64(float64(levelConfig.Recommendbet) * costCoinsM * float64(n2CfgRoomlist.GetBetStakegear()))
	baseAmount := n2CfgRoomlist.GetBetbaseamount()
	return betArr2, int(defaultBet), int(baseAmount), nil
}

// 获取基础倍率
func getUserBaseBets(levelConfig *dbData.FormatLevelConfig, costCoinsM float64) []int64 {
	ret := make([]int64, 0, 15)
	for _, v := range levelConfig.Stakegear {
		if v < int(levelConfig.Minbet) {
			continue
		}
		if v > int(levelConfig.Maxbet) {
			break
		}
		ret = append(ret, int64(float64(v)*costCoinsM))
	}
	return ret

}
func ToArrList(src *logicGameModel.SlotData) []*pb.Int32Array2 {
	var arrayThreeList []*pb.Int32Array2
	// A. 转换 SymbolsSequences (三维数组 -> repeated ReelSet)
	for _, one := range src.SymbolsSequences {
		arrayTwoList := &pb.Int32Array2{}
		for _, two := range one {
			arrayOneList := &pb.Int32Array1{}
			for _, v := range two {
				arrayOneList.Array = append(arrayOneList.Array, v)
			}
			arrayTwoList.Array = append(arrayTwoList.Array, arrayOneList)
		}
		arrayThreeList = append(arrayThreeList, arrayTwoList)
	}

	return arrayThreeList
}
