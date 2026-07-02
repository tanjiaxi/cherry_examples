package main

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// PlayerState 玩家状态结构
type PlayerState struct {
	UserID    int64 `json:"user_id"`
	Balance   int64 `json:"balance"`      // 当前余额
	TotalBet  int64 `json:"total_bet"`    // 总下注
	TotalWin  int64 `json:"total_win"`    // 总赢取
	LastSeq   int64 `json:"last_seq"`     // 最后处理的序列号
	SpinCount int64 `json:"spin_count"`   // 总旋转次数
}

// PlayerStateManager 玩家状态管理器
type PlayerStateManager struct {
	players map[int64]*PlayerState // 内存中的玩家状态
	mu      sync.RWMutex           // 读写锁保护
}

// NewPlayerStateManager 创建状态管理器
func NewPlayerStateManager() *PlayerStateManager {
	return &PlayerStateManager{
		players: make(map[int64]*PlayerState),
	}
}

// GetOrCreatePlayer 获取或创建玩家状态
func (m *PlayerStateManager) GetOrCreatePlayer(userID int64, initialBalance int64) *PlayerState {
	m.mu.Lock()
	defer m.mu.Unlock()

	if player, exists := m.players[userID]; exists {
		return player
	}

	player := &PlayerState{
		UserID:    userID,
		Balance:   initialBalance,
		TotalBet:  0,
		TotalWin:  0,
		LastSeq:   0,
		SpinCount: 0,
	}
	m.players[userID] = player
	return player
}

// UpdatePlayerState 更新玩家状态（幂等操作）
func (m *PlayerStateManager) UpdatePlayerState(record SpinRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	player, exists := m.players[record.UserID]
	if !exists {
		return nil // 玩家不存在，跳过
	}

	// 幂等性检查：如果序列号已经处理过，跳过
	if record.Sequence <= player.LastSeq {
		return nil
	}

	// 更新状态
	player.Balance -= record.BetAmount
	player.Balance += record.WinAmount
	player.TotalBet += record.BetAmount
	player.TotalWin += record.WinAmount
	player.SpinCount++
	player.LastSeq = record.Sequence

	return nil
}

// GetPlayerState 获取玩家当前状态（只读）
func (m *PlayerStateManager) GetPlayerState(userID int64) (*PlayerState, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	player, exists := m.players[userID]
	if !exists {
		return nil, false
	}

	// 返回副本，避免外部修改
	stateCopy := *player
	return &stateCopy, true
}

// GetAllPlayers 获取所有玩家状态（只读）
func (m *PlayerStateManager) GetAllPlayers() []*PlayerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	players := make([]*PlayerState, 0, len(m.players))
	for _, player := range m.players {
		stateCopy := *player
		players = append(players, &stateCopy)
	}
	return players
}

// PlayerSnapshot 玩家状态快照
type PlayerSnapshot struct {
	Timestamp int64           `json:"timestamp"`
	Players   []*PlayerState  `json:"players"`
}

// ExportSnapshot 导出快照到JSON文件
func (m *PlayerStateManager) ExportSnapshot(filepath string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snapshot := PlayerSnapshot{
		Timestamp: time.Now().Unix(),
		Players:   make([]*PlayerState, 0, len(m.players)),
	}

	for _, player := range m.players {
		stateCopy := *player
		snapshot.Players = append(snapshot.Players, &stateCopy)
	}

	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filepath, data, 0644)
}

// ImportSnapshot 从JSON文件导入快照
func (m *PlayerStateManager) ImportSnapshot(filepath string) error {
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，跳过恢复
		}
		return err
	}

	var snapshot PlayerSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// 恢复玩家状态
	for _, player := range snapshot.Players {
		playerCopy := *player
		m.players[player.UserID] = &playerCopy
	}

	return nil
}

// GetStats 获取统计信息
func (m *PlayerStateManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalBalance := int64(0)
	totalBet := int64(0)
	totalWin := int64(0)
	totalSpins := int64(0)

	for _, player := range m.players {
		totalBalance += player.Balance
		totalBet += player.TotalBet
		totalWin += player.TotalWin
		totalSpins += player.SpinCount
	}

	return map[string]interface{}{
		"player_count":  len(m.players),
		"total_balance": totalBalance,
		"total_bet":     totalBet,
		"total_win":     totalWin,
		"total_spins":   totalSpins,
	}
}
