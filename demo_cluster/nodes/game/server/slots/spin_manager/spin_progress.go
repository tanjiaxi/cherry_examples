/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 21:27:47
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 15:09:32
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_manager/spin_progress.go
 * @Description: 这是spin 中的一些分解组合，跑种子等类
 */
package spinmanage

import (
	clog "github.com/cherry-game/cherry/logger"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
	spinResult "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_engine/result"
)

type SpinResult struct {
}

// 开始spin
func StarSPin(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig *gameModel.N2CfgRoomlist, reelJsonObj *gameModel.N2CfgReelRoom, roomDataInfo *slotsModel.RoomDataInfo) (*pb.SpinResult, error) {
	//获取spin
	resultLogic := spinResult.CreatGenResltByRoomId(ruleId)
	if resultLogic == nil {
		clog.Error("resultLogic is nil")
	}
	resultLogic.OnInit(roomId, ruleId, bet, roomDataInfo, reelJsonObj)
	return resultLogic.GenResult(roomId, ruleId, isInit, bet, collectAddMoney, roomCongfig, reelJsonObj, roomDataInfo)
}
