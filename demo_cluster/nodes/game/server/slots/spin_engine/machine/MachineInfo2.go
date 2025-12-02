/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-01 14:24:25
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 17:32:34
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/machine/MachineInfo2.go
 * @Description: 关卡进关卡的逻辑
 */
package machine

import (
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

type MachineInfo2 struct {
	BaseMachine
}

// NewMachineInfo1 创建 MachineInfo2 实例
func NewMachineInfo2(base BaseMachine) *MachineInfo2 {
	return &MachineInfo2{
		BaseMachine: base,
	}
}

// GetSpinResult 重写 Spin 结果计算逻辑（Machine1 特有逻辑）
func (m *MachineInfo2) convertFeature() (*pb.Firebonus777Feature, error) {
	response := &pb.Firebonus777Feature{}
	return response, nil
}

// GetFeature 获取 Machine1 的特性
func (m *MachineInfo2) GetFeature() (*pb.FeatureInfo, error) {
	featureInfo, err := m.convertFeature()
	if err != nil {
		return nil, err
	}
	feature := &pb.FeatureInfo_Firebonus777Feature{Firebonus777Feature: featureInfo} // 使用 pb.FeatureInfo_Firebonus777Feature 类型的结构体作为值，这样可以确保类型安全。
	return &pb.FeatureInfo{Feature: feature}, nil
}
