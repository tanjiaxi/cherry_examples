/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-01 14:33:17
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-01 14:39:08
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/machine/machine_interface.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
/*
 * @Description: Machine 接口定义
 */
package machine

import (
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	spinManager "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_manager"
)

// IMachine 定义所有 Machine 必须实现的接口
type IMachine interface {
	// InitData 初始化机器数据
	InitData() error

	// GetBase 获取基础信息
	GetBase() (*pb.BaseInfo, error)

	// ConvertStage 转换游戏阶段
	ConvertStage() (*pb.GameStage, error)

	// GetInitSpinResult 获取初始 Spin 结果
	GetInitSpinResult() (*pb.SpinResponse, error)

	// GetSpinResult 获取 Spin 结果
	GetSpinResult(bet int64) (*pb.SpinResponse, error)

	// GetReelsInfo 获取卷轴信息
	GetReelsInfo() error

	// GetPayTable 获取赔付表
	GetPayTable() error

	// GetFeature 获取特性信息
	GetFeature() error

	// GetJackpot 获取 Jackpot 信息
	GetJackpot() error
}

// MachineFactory 机器工厂接口
type MachineFactory interface {
	CreateMachine(roomId int32, session *cproto.Session, roomDataInfo *spinManager.RoomDataInfo) (IMachine, error)
}
