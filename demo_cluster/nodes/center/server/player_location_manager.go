package server

import (
	"fmt"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/model"
)

// PlayerLocationManager 玩家位置管理器
// 管理玩家与Gate/Game节点的绑定关系
type PlayerLocationManager struct {
	cache map[int64]*model.PlayerLocation // 内存缓存 userId -> location
	// mu    sync.RWMutex

	// 依赖的组件
	nodeCounter   *NodeOnlineCounter
	healthChecker *NodeHealthChecker
}

// NewPlayerLocationManager 创建玩家位置管理器
func NewPlayerLocationManager(counter *NodeOnlineCounter, checker *NodeHealthChecker) *PlayerLocationManager {
	return &PlayerLocationManager{
		cache:         make(map[int64]*model.PlayerLocation),
		nodeCounter:   counter,
		healthChecker: checker,
	}
}

// AllocateNodes 为玩家分配Gate和Game节点
// 如果玩家已有位置（断线重连），返回已有位置
// 否则使用负载均衡分配新节点
func (m *PlayerLocationManager) AllocateNodes(userId int64, gateNodeId string, gameNodes []string) (*model.PlayerLocation, error) {
	// m.mu.Lock()
	// defer m.mu.Unlock()

	// 检查是否已有位置（断线重连场景）
	if loc, exists := m.cache[userId]; exists && loc.IsOnline() {
		clog.Infof("[PlayerLocationManager] 玩家 %d 断线重连，返回已有位置: gate=%s, game=%s",
			userId, loc.GateNodeId, loc.GameNodeId)
		return loc, nil
	}

	// 使用负载均衡选择Game节点
	gameNodeId := m.nodeCounter.GetLeastLoadedNode("game", gameNodes)
	if gameNodeId == "" {
		return nil, fmt.Errorf("no available game node")
	}

	// 创建新的位置记录
	loc := model.NewPlayerLocation(userId, gateNodeId, gameNodeId)
	m.cache[userId] = loc

	// 增加节点在线人数
	m.nodeCounter.Increment(gameNodeId)
	m.nodeCounter.Increment(gateNodeId)

	clog.Infof("[PlayerLocationManager] 为玩家 %d 分配节点: gate=%s, game=%s",
		userId, gateNodeId, gameNodeId)

	return loc, nil
}

// GetLocation 获取玩家位置
func (m *PlayerLocationManager) GetLocation(userId int64) (*model.PlayerLocation, bool) {
	// m.mu.RLock()
	// defer m.mu.RUnlock()

	loc, exists := m.cache[userId]
	if !exists {
		return nil, false
	}
	return loc.Clone(), true
}

// UpdateLocation 更新玩家位置
func (m *PlayerLocationManager) UpdateLocation(loc *model.PlayerLocation) error {
	if loc == nil || loc.UserId <= 0 {
		return fmt.Errorf("invalid player location")
	}

	// m.mu.Lock()
	// defer m.mu.Unlock()

	m.cache[loc.UserId] = loc
	return nil
}

// RemoveLocation 移除玩家位置（玩家登出）
func (m *PlayerLocationManager) RemoveLocation(userId int64) error {
	// m.mu.Lock()
	// defer m.mu.Unlock()
	startTime := time.Now()
	loc, exists := m.cache[userId]
	if !exists {
		return nil
	}

	// 减少节点在线人数
	m.nodeCounter.Decrement(loc.GameNodeId)
	m.nodeCounter.Decrement(loc.GateNodeId)

	// 从缓存中删除
	delete(m.cache, userId)

	elapsed := time.Since(startTime)
	clog.Infof("[PlayerLocationManager] 移除玩家 %d 位置: gate=%s, game=%s time=%s",
		userId, loc.GateNodeId, loc.GameNodeId, elapsed)

	return nil
}

// SetOffline 设置玩家离线（但保留位置，用于断线重连）
func (m *PlayerLocationManager) SetOffline(userId int64) {
	// m.mu.Lock()
	// defer m.mu.Unlock()

	if loc, exists := m.cache[userId]; exists {
		loc.SetOffline()
	}
}

// GetPlayersByGameNode 获取指定Game节点上的所有玩家
func (m *PlayerLocationManager) GetPlayersByGameNode(gameNodeId string) []int64 {
	// m.mu.RLock()
	// defer m.mu.RUnlock()

	var players []int64
	for userId, loc := range m.cache {
		if loc.GameNodeId == gameNodeId {
			players = append(players, userId)
		}
	}
	return players
}

// MigratePlayersFromNode 从故障节点迁移玩家到其他节点
func (m *PlayerLocationManager) MigratePlayersFromNode(failedNodeId string, healthyNodes []string) (int, error) {
	if len(healthyNodes) == 0 {
		return 0, fmt.Errorf("no healthy nodes available for migration")
	}

	// m.mu.Lock()
	// defer m.mu.Unlock()

	migratedCount := 0
	for userId, loc := range m.cache {
		if loc.GameNodeId == failedNodeId {
			// 选择新的健康节点
			newNodeId := m.nodeCounter.GetLeastLoadedNode("game", healthyNodes)
			if newNodeId == "" {
				continue
			}

			// 更新位置
			oldNodeId := loc.GameNodeId
			loc.UpdateGameNode(newNodeId)

			// 更新在线人数统计
			m.nodeCounter.Decrement(oldNodeId)
			m.nodeCounter.Increment(newNodeId)

			migratedCount++
			clog.Infof("[PlayerLocationManager] 迁移玩家 %d: %s -> %s",
				userId, oldNodeId, newNodeId)
		}
	}

	return migratedCount, nil
}

// GetOnlineCount 获取在线玩家数量
func (m *PlayerLocationManager) GetOnlineCount() int {
	// m.mu.RLock()
	// defer m.mu.RUnlock()

	count := 0
	for _, loc := range m.cache {
		if loc.IsOnline() {
			count++
		}
	}
	return count
}

// CleanupExpiredLocations 清理过期的离线位置
func (m *PlayerLocationManager) CleanupExpiredLocations(expireSeconds int64) int {
	// m.mu.Lock()
	// defer m.mu.Unlock()

	now := time.Now().Unix()
	cleanedCount := 0

	for userId, loc := range m.cache {
		// 只清理离线状态且超过过期时间的记录
		if !loc.IsOnline() && (now-loc.UpdatedAt) > expireSeconds {
			delete(m.cache, userId)
			cleanedCount++
		}
	}

	if cleanedCount > 0 {
		clog.Infof("[PlayerLocationManager] 清理了 %d 个过期位置记录", cleanedCount)
	}

	return cleanedCount
}
