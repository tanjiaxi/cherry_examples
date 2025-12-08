/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 23:46:24
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 14:59:20
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/component/spin_manager.go
 * @Description: 这是进入spin，前，后的数据获取和处理。 （玩家赔率的控制，产生的数据，处理，管理关卡的数据转换提供给关卡逻辑）
 */
package spinmanage

import (
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

// 组件层（全局单例）
type SpinManager struct {
}

// 为spin做准备
func ReadySPin(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig *gameModel.N2CfgRoomlist, reelJsonObj *gameModel.N2CfgReelRoom, roomDataInfo *slotsModel.RoomDataInfo) {
	SpinBefore()
	StarSPin(roomId, ruleId, isInit, bet, collectAddMoney, roomCongfig, reelJsonObj, roomDataInfo)
	SpinAfter()
}
func SpinBefore() {}
func SpinAfter()  {}
func SpinEnd()    {}
