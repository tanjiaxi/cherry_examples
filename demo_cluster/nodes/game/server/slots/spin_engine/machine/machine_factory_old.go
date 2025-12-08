/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-01 14:33:32
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 14:12:35
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/machine/machine_factory.go
 * @Description: Machine 使用简单的switch语句来创建不同的Machine实例，这里只是一个简单的示例，实际项目中可能需要更复杂的逻辑来决定使用哪个Machine实例。
 */
/*
 * @Description: Machine 工厂实现
 */
package machine

import (
	"fmt"

	cproto "github.com/cherry-game/cherry/net/proto"
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

// DefaultMachineFactoryOld 默认机器工厂
type DefaultMachineFactoryOld struct {
	// 可以添加配置映射
	roomMachineMap map[int32]string // roomId -> machineType 的映射
}

// NewMachineFactoryOld 创建工厂实例
func NewMachineFactoryOld() *DefaultMachineFactoryOld {
	return &DefaultMachineFactoryOld{
		roomMachineMap: make(map[int32]string),
	}
}

// CreateMachine 根据 roomId 创建对应的 Machine 实例
func (f *DefaultMachineFactoryOld) CreateMachine(
	roomId int32,
	session *cproto.Session,
	roomDataInfo *slotsModel.RoomDataInfo,
) (IMachine, error) {
	// 从配置中获取房间信息，判断使用哪个 Machine
	roomConfig, err := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	if err != nil {
		return nil, fmt.Errorf("failed to get room config: %w", err)
	}

	// 根据房间配置的字段来决定使用哪个 Machine
	// 这里使用 Version 字段来区分（你可以根据实际需求修改）
	version := roomConfig.Version

	var machine IMachine
	baseMachine := *NewBaseMachine(roomId, session, roomDataInfo, nil, roomId%1000)
	// 注意：这里 userInfo 传 nil，如果需要可以从 session 或其他地方获取
	switch version {
	case 1:
		// 创建 MachineInfo1
		m := &MachineInfo1{
			BaseMachine: baseMachine,
		}
		machine = m
	case 2:
		// 创建 MachineInfo2
		m := &MachineInfo2{
			BaseMachine: baseMachine,
		}
		machine = m
	default:
		// 默认使用 MachineInfo1
		m := &MachineInfo1{
			BaseMachine: baseMachine,
		}
		machine = m
	}

	// 初始化机器数据
	if err := machine.InitData(); err != nil {
		return nil, fmt.Errorf("failed to init machine data: %w", err)
	}

	return machine, nil
}

// CreateMachineByRoomId 便捷方法：直接通过 roomId 创建
func CreateMachineByRoomIdOld(
	roomId int32,
	session *cproto.Session,
	roomDataInfo *slotsModel.RoomDataInfo,
) (IMachine, error) {
	factory := NewMachineFactoryOld()
	return factory.CreateMachine(roomId, session, roomDataInfo)
}

// RegisterMachineType 注册自定义的 roomId 到 machineType 的映射
func (f *DefaultMachineFactoryOld) RegisterMachineType(roomId int32, machineType string) {
	f.roomMachineMap[roomId] = machineType
}
