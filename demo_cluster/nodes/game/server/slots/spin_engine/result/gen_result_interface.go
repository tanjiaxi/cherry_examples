/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 15:09:23
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 关卡抽象对象
 */
package result

import (
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

type SpinResult struct {
}
type ResultInterface interface {
	OnInit(roomId, ruleId int32, bet int, roomDataInfo *slotsModel.RoomDataInfo, reelJsonObj *gameModel.N2CfgReelRoom)
	GenResult(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig *gameModel.N2CfgRoomlist, reelJsonObj *gameModel.N2CfgReelRoom, roomDataInfo *slotsModel.RoomDataInfo) (*pb.SpinResult, error)
	GetGameMap() float64
	GetWinType() string
	GetWinLines() error
	GetAllLinesData() error
}
