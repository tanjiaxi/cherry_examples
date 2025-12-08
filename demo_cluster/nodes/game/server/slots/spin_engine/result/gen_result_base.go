/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 15:36:03
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 关卡基础类
 */
package result

import (
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

type GenResultBase struct {
	roomDataInfo *slotsModel.RoomDataInfo
	reelJsonObj  *gameModel.N2CfgReelRoom
	roomId       int32
	ruleId       int32
	bet          int
	stage        int  //状态bonus，freeSpin，
	stageType    int  //是否是reSpin
	needSave     bool //是否需要保存种子
}

func (g *GenResultBase) OnInit(roomId, ruleId int32, bet int, roomDataInfo *slotsModel.RoomDataInfo, reelJsonObj *gameModel.N2CfgReelRoom) {
	g.roomDataInfo = roomDataInfo
	g.reelJsonObj = reelJsonObj
	g.roomId = roomId
	g.ruleId = ruleId
	g.bet = bet

}
func NewGenResultBase() *GenResultBase {
	return &GenResultBase{}
}
func (g *GenResultBase) GenResult(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig *gameModel.N2CfgRoomlist, reelJsonObj *gameModel.N2CfgReelRoom, roomDataInfo *slotsModel.RoomDataInfo) (*pb.SpinResult, error) {
	return nil, nil
}
func (g *GenResultBase) GetGameMap() float64 {
	return 0
}
func (g *GenResultBase) GetWinType() string {
	return ""
}
func (g *GenResultBase) GetWinLines() error {
	return nil
}
func (g *GenResultBase) GetAllLinesData() error {
	return nil
}
func (g *GenResultBase) SetSeedSave() {
	if g.stage != slotsModel.NORMAL || g.stageType == slotsModel.RE_SPIN_NORMAL {
		g.needSave = false
	} else {
		g.needSave = true
	}
}
