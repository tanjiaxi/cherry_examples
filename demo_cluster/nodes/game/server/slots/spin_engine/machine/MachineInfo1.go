/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-01 14:24:25
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 17:42:38
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/machine/MachineInfo1.go
 * @Description: 关卡进关卡的逻辑
 */
package machine

import (
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

type MachineInfo1 struct {
	BaseMachine
}

// NewMachineInfo1 创建 MachineInfo1 实例
func NewMachineInfo1(base BaseMachine) *MachineInfo1 {
	return &MachineInfo1{
		BaseMachine: base,
	}
}

// GetSpinResult 重写 Spin 结果计算逻辑（Machine1 特有逻辑）
func (m *MachineInfo1) convertFeature() (*pb.Amazing777Feature, error) {
	response := &pb.Amazing777Feature{
		BonusInfo: &pb.Amazing777Bonus{
			BaseInfo: &pb.BonusBaseInfo{
				WinType:  pb.WinType_WIN_MINOR,
				WinMoney: 10000,
			},
		},
	}
	return response, nil
}

// GetFeature 获取 Machine1 的特性
func (m *MachineInfo1) GetFeature() (*pb.FeatureInfo, error) {
	featureInfo, err := m.convertFeature()
	if err != nil {
		return nil, err
	}
	feature := &pb.FeatureInfo_Amazing777Feature{
		Amazing777Feature: featureInfo,
	}
	return &pb.FeatureInfo{
		Feature: feature,
	}, nil

}
