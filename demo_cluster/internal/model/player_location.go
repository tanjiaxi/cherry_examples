/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-21 18:37:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-21 18:47:39
 * @FilePath: /examples/demo_cluster/internal/model/player_location.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package model

import (
	"encoding/json"
	"time"
)

// PlayerLocation 玩家位置信息
// 记录玩家当前所在的Gate和Game节点
type PlayerLocation struct {
	UserId     int64  `json:"user_id" gorm:"primaryKey;column:user_id"`
	GateNodeId string `json:"gate_node_id" gorm:"column:gate_node_id;size:64;not null"`
	GameNodeId string `json:"game_node_id" gorm:"column:game_node_id;size:64;not null"`
	LoginTime  int64  `json:"login_time" gorm:"column:login_time;not null"`
	Status     int32  `json:"status" gorm:"column:status;default:1"` // 1=online, 0=offline
	CreatedAt  int64  `json:"created_at" gorm:"column:created_at;autoCreateTime"`
	UpdatedAt  int64  `json:"updated_at" gorm:"column:updated_at;autoUpdateTime"`
}

// TableName 指定表名
func (PlayerLocation) TableName() string {
	return "newsz_2024.player_location"
}

// PlayerLocationStatus 玩家位置状态
const (
	PlayerLocationStatusOffline = 0
	PlayerLocationStatusOnline  = 1
)

// NewPlayerLocation 创建新的玩家位置
func NewPlayerLocation(UserId int64, gateNodeId, gameNodeId string) *PlayerLocation {
	return &PlayerLocation{
		UserId:     UserId,
		GateNodeId: gateNodeId,
		GameNodeId: gameNodeId,
		LoginTime:  time.Now().Unix(),
		Status:     PlayerLocationStatusOnline,
	}
}

// IsOnline 检查玩家是否在线
func (p *PlayerLocation) IsOnline() bool {
	return p.Status == PlayerLocationStatusOnline
}

// SetOffline 设置玩家离线
func (p *PlayerLocation) SetOffline() {
	p.Status = PlayerLocationStatusOffline
	p.UpdatedAt = time.Now().Unix()
}

// SetOnline 设置玩家在线
func (p *PlayerLocation) SetOnline() {
	p.Status = PlayerLocationStatusOnline
	p.UpdatedAt = time.Now().Unix()
}

// UpdateGameNode 更新Game节点（用于故障迁移）
func (p *PlayerLocation) UpdateGameNode(newGameNodeId string) {
	p.GameNodeId = newGameNodeId
	p.UpdatedAt = time.Now().Unix()
}

// ToJSON 序列化为JSON
func (p *PlayerLocation) ToJSON() ([]byte, error) {
	return json.Marshal(p)
}

// FromJSON 从JSON反序列化
func (p *PlayerLocation) FromJSON(data []byte) error {
	return json.Unmarshal(data, p)
}

// Clone 克隆一个副本
func (p *PlayerLocation) Clone() *PlayerLocation {
	return &PlayerLocation{
		UserId:     p.UserId,
		GateNodeId: p.GateNodeId,
		GameNodeId: p.GameNodeId,
		LoginTime:  p.LoginTime,
		Status:     p.Status,
		CreatedAt:  p.CreatedAt,
		UpdatedAt:  p.UpdatedAt,
	}
}

// IsValid 检查位置信息是否有效
func (p *PlayerLocation) IsValid() bool {
	return p.UserId > 0 &&
		p.GateNodeId != "" &&
		p.GameNodeId != "" &&
		p.LoginTime > 0
}
