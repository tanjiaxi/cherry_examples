/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-01 14:24:25
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-15 17:35:32
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/machine/MachineInfo86.go
 * @Description: 关卡进关卡的逻辑
 */
package machine

import (
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

type MachineInfo86 struct {
	BaseMachine
}

// NewMachineInfo1 创建 MachineInfo86 实例
func NewMachineInfo86(base BaseMachine) *MachineInfo86 {
	return &MachineInfo86{
		BaseMachine: base,
	}
}

// GetSpinResult 重写 Spin 结果计算逻辑（Machine1 特有逻辑）
func (m *MachineInfo86) convertFeature() (*pb.Firebonus777Feature, error) {
	response := &pb.Firebonus777Feature{}
	return response, nil
}

// GetFeature 获取 Machine1 的特性
func (m *MachineInfo86) GetFeature() (*pb.FeatureInfo, error) {
	featureInfo, err := m.convertFeature()
	if err != nil {
		return nil, err
	}
	feature := &pb.FeatureInfo_Firebonus777Feature{Firebonus777Feature: featureInfo} // 使用 pb.FeatureInfo_Firebonus777Feature 类型的结构体作为值，这样可以确保类型安全。
	return &pb.FeatureInfo{Feature: feature}, nil
}
