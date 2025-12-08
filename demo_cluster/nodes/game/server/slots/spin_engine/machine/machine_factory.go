/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-01 14:33:32
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 14:11:32
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/machine/machine_factory.go
 * @Description: 使用映射的方式实现，工厂模式
 */
/*
 * @Description: Machine 工厂实现
 */
package machine

import (
	clog "github.com/cherry-game/cherry/logger"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

/*
 * @Description: Machine 工厂实现
 * @param roomId  为真实的ruleId 就是1，2，3，不是1001
 * @param session
 * @param roomDataInfo
 * @param userInfo
 * @return IMachine
 */
type MachineFactoryFunc func(roomId int32, session *cproto.Session, roomDataInfo *slotsModel.RoomDataInfo, userInfo *pb.GetUserInfoResponse) IMachine

var machineRegistry = make(map[int32]MachineFactoryFunc)

/*
 * @Description: 注册工厂方法
 * @param machineType
 * @param factoryFunc
 */
func RegisterMachineFactory(ruleId int32, factoryFunc MachineFactoryFunc) {
	if factoryFunc, ok := machineRegistry[ruleId]; ok {
		if factoryFunc != nil { // 已经注册过了
			clog.Panic("ruleId %d is registered ", ruleId)
		}
	}
	machineRegistry[ruleId] = factoryFunc
}
func CreateMachineByType(ruleId int32, roomId int32, session *cproto.Session, roomDataInfo *slotsModel.RoomDataInfo, userInfo *pb.GetUserInfoResponse) IMachine {
	if factoryFunc, ok := machineRegistry[ruleId]; ok {
		return factoryFunc(roomId, session, roomDataInfo, userInfo)
	}
	return nil
}

// 每一个关卡需要在这里注册
func RegisterMachineAll() {
	//rule：1
	RegisterMachineFactory(1, func(roomId int32, session *cproto.Session, roomDataInfo *slotsModel.RoomDataInfo, userInfo *pb.GetUserInfoResponse) IMachine {
		baseMachine := NewBaseMachine(roomId, session, roomDataInfo, userInfo, roomId%1000)
		return NewMachineInfo1(*baseMachine)
	})
	//ruel:2
	RegisterMachineFactory(2, func(roomId int32, session *cproto.Session, roomDataInfo *slotsModel.RoomDataInfo, userInfo *pb.GetUserInfoResponse) IMachine {
		baseMachine := NewBaseMachine(roomId, session, roomDataInfo, userInfo, roomId%1000)
		return NewMachineInfo2(*baseMachine)
	})
}
