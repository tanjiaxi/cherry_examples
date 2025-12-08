# IsDirty 标记时机指南

## 🎯 核心问题

**何时标记 `IsDirty = true`？**

1. **修改前标记**：简单，但可能标记了无效修改
2. **修改后标记**：安全，但需要确保所有修改都标记
3. **成功后标记**：最安全，但实现复杂

## 📊 三种方案对比

### 方案 1：修改前标记（不推荐）❌

```go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomDataInfo := r.roomDataManager.GetLevelSessionDataByRoomId(...)
    
    // ❌ 问题：还没修改就标记了
    roomDataInfo.IsDirty = true
    
    // 执行 spin 逻辑
    result, err := machine.GetSpinResult(req.Bet)
    if err != nil {
        // ❌ 问题：失败了，但已经标记为脏
        // 会同步错误的数据到数据库
        return
    }
    
    // 修改数据
    roomDataInfo.SpinNum++
    roomDataInfo.FreeSpinNum = result.FreeSpinNum
}
```

**问题**：
- ❌ 如果 spin 失败，数据没有修改，但已经标记为脏
- ❌ 会同步未修改的数据到数据库（浪费）
- ❌ 可能导致数据不一致

### 方案 2：修改后立即标记（推荐）⭐

```go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomDataInfo := r.roomDataManager.GetLevelSessionDataByRoomId(...)
    
    // 1. 执行 spin 逻辑
    result, err := machine.GetSpinResult(req.Bet)
    if err != nil {
        // 失败了，不修改数据，不标记
        clog.Errorf("Spin 失败: %v", err)
        r.ResponseCode(session, code.SpinFailed)
        return
    }
    
    // 2. 修改数据
    roomDataInfo.SpinNum++
    roomDataInfo.FreeSpinNum = result.FreeSpinNum
    roomDataInfo.CurBetNum = req.Bet
    roomDataInfo.Stage = result.NextStage
    
    // 3. ✅ 修改完成后立即标记
    roomDataInfo.IsDirty = true
    roomDataInfo.UpdatedAt = time.Now().Unix()
    
    // 4. 返回响应
    r.Response(session, result)
}
```

**优点**：
- ✅ 只有成功修改后才标记
- ✅ 逻辑清晰，易于理解
- ✅ 不会同步错误数据

**缺点**：
- ⚠️ 需要记得在每次修改后标记
- ⚠️ 如果有多处修改，可能遗漏

### 方案 3：使用包装方法（最推荐）⭐⭐⭐

```go
// room_data_manager.go
type RoomDataManager struct {
    levelSessionDataMgr map[int32]*RoomDataInfo
    mutex               sync.RWMutex
}

// UpdateRoomData 包装方法，自动标记脏数据
func (m *RoomDataManager) UpdateRoomData(
    roomId int32,
    updateFunc func(*RoomDataInfo) error,
) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    data, ok := m.levelSessionDataMgr[roomId]
    if !ok {
        return fmt.Errorf("room data not found")
    }
    
    // 1. 执行更新函数
    if err := updateFunc(data); err != nil {
        // ✅ 更新失败，不标记为脏
        return err
    }
    
    // 2. ✅ 更新成功后自动标记
    data.IsDirty = true
    data.UpdatedAt = time.Now().Unix()
    
    return nil
}
```

```go
// level_room.go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomId := req.Id
    
    // 使用包装方法，自动处理脏标记
    err := r.roomDataManager.UpdateRoomData(roomId, func(data *RoomDataInfo) error {
        // 1. 执行 spin 逻辑
        machine := spinEngine.CreateMachineByType(...)
        result, err := machine.GetSpinResult(req.Bet)
        if err != nil {
            // ✅ 返回错误，不会标记为脏
            return err
        }
        
        // 2. 修改数据
        data.SpinNum++
        data.FreeSpinNum = result.FreeSpinNum
        data.CurBetNum = req.Bet
        data.Stage = result.NextStage
        
        // 3. 返回响应
        r.Response(session, result)
        
        // 4. ✅ 返回 nil 表示成功，会自动标记为脏
        return nil
    })
    
    if err != nil {
        clog.Errorf("Spin 失败: %v", err)
        r.ResponseCode(session, code.SpinFailed)
    }
}
```

**优点**：
- ✅ 自动处理脏标记，不会遗漏
- ✅ 只有成功时才标记
- ✅ 统一的错误处理
- ✅ 支持事务回滚
- ✅ 代码更清晰

## 🔧 完整实现

### 1. 数据结构

```go
// level_data_types.go
type RoomDataInfo struct {
    // 基础信息
    UserId int32 `json:"user_id"`
    RoomId int32 `json:"room_id"`
    
    // 游戏数据
    SpinNum     int   `json:"spin_num"`
    FreeSpinNum int   `json:"free_spin_num"`
    CurBetNum   int64 `json:"cur_bet_num"`
    Stage       int   `json:"stage"`
    NextStage   int   `json:"next_stage"`
    
    // 元数据
    IsDirty   bool  `json:"-"`          // 脏数据标记（不序列化）
    UpdatedAt int64 `json:"updated_at"` // 最后更新时间
    Version   int   `json:"version"`    // 版本号（乐观锁）
}

// MarkDirty 标记为脏数据
func (r *RoomDataInfo) MarkDirty() {
    r.IsDirty = true
    r.UpdatedAt = time.Now().Unix()
}

// ClearDirty 清除脏标记
func (r *RoomDataInfo) ClearDirty() {
    r.IsDirty = false
}
```

### 2. 管理器实现

```go
// room_data_manager.go
package spinmanage

import (
    "fmt"
    "sync"
    "time"
)

type RoomDataManager struct {
    levelSessionDataMgr map[int32]*RoomDataInfo
    mutex               sync.RWMutex
}

func NewSessoinManager() *RoomDataManager {
    return &RoomDataManager{
        levelSessionDataMgr: make(map[int32]*RoomDataInfo),
    }
}

// GetLevelSessionDataByRoomId 获取房间数据（只读）
func (m *RoomDataManager) GetLevelSessionDataByRoomId(userID, roomId int32) *RoomDataInfo {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    
    if data, ok := m.levelSessionDataMgr[roomId]; ok {
        return data
    }
    
    // 创建新数据
    data := &RoomDataInfo{
        UserId: userID,
        RoomId: roomId,
    }
    m.levelSessionDataMgr[roomId] = data
    return data
}

// UpdateRoomData 更新房间数据（自动标记脏数据）
func (m *RoomDataManager) UpdateRoomData(
    roomId int32,
    updateFunc func(*RoomDataInfo) error,
) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    data, ok := m.levelSessionDataMgr[roomId]
    if !ok {
        return fmt.Errorf("room data not found: roomId=%d", roomId)
    }
    
    // 执行更新
    if err := updateFunc(data); err != nil {
        return err
    }
    
    // 自动标记为脏
    data.MarkDirty()
    
    return nil
}

// UpdateRoomDataWithSnapshot 带快照的更新（支持回滚）
func (m *RoomDataManager) UpdateRoomDataWithSnapshot(
    roomId int32,
    updateFunc func(*RoomDataInfo) error,
) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    data, ok := m.levelSessionDataMgr[roomId]
    if !ok {
        return fmt.Errorf("room data not found: roomId=%d", roomId)
    }
    
    // 创建快照
    snapshot := *data
    
    // 执行更新
    if err := updateFunc(data); err != nil {
        // 回滚
        *data = snapshot
        return err
    }
    
    // 自动标记为脏
    data.MarkDirty()
    
    return nil
}

// SyncDirtyData 同步脏数据到数据库
func (m *RoomDataManager) SyncDirtyData() error {
    m.mutex.RLock()
    dirtyData := make([]*RoomDataInfo, 0)
    for _, data := range m.levelSessionDataMgr {
        if data.IsDirty {
            dirtyData = append(dirtyData, data)
        }
    }
    m.mutex.RUnlock()
    
    // 批量保存
    for _, data := range dirtyData {
        if err := db.SaveRoomData(data); err != nil {
            clog.Errorf("保存房间数据失败: userId=%d, roomId=%d, err=%v",
                data.UserId, data.RoomId, err)
            continue
        }
        data.ClearDirty()
    }
    
    return nil
}

// StartAutoSync 启动定时同步
func (m *RoomDataManager) StartAutoSync(interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            if err := m.SyncDirtyData(); err != nil {
                clog.Errorf("定时同步失败: %v", err)
            }
        }
    }()
}
```

### 3. 使用示例

#### 示例 1：普通 Spin

```go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomId := req.Id
    
    // 使用包装方法
    err := r.roomDataManager.UpdateRoomData(roomId, func(data *RoomDataInfo) error {
        // 1. 获取 Machine
        machine := spinEngine.CreateMachineByType(...)
        if machine == nil {
            return fmt.Errorf("create machine failed")
        }
        
        // 2. 执行 Spin
        result, err := machine.GetSpinResult(req.Bet)
        if err != nil {
            return fmt.Errorf("spin failed: %w", err)
        }
        
        // 3. 更新数据
        data.SpinNum++
        data.CurBetNum = req.Bet
        data.FreeSpinNum = result.FreeSpinNum
        data.Stage = result.NextStage
        
        // 4. 返回响应
        r.Response(session, result)
        
        return nil  // ✅ 成功，会自动标记为脏
    })
    
    if err != nil {
        clog.Errorf("Spin 失败: %v", err)
        r.ResponseCode(session, code.SpinFailed)
    }
}
```

#### 示例 2：Bonus（需要回滚）

```go
func (r *ActorRoom) bonus(session *cproto.Session, req *pb.Bonus) {
    roomId := req.Id
    
    // 使用带快照的更新（支持回滚）
    err := r.roomDataManager.UpdateRoomDataWithSnapshot(roomId, func(data *RoomDataInfo) error {
        // 1. 验证状态
        if data.Stage != STAGE_BONUS {
            return fmt.Errorf("not in bonus stage")
        }
        
        // 2. 执行 Bonus
        bonusEngine := spinEngine.CreateBonusByType(...)
        result, err := bonusEngine.Execute(req)
        if err != nil {
            return fmt.Errorf("bonus failed: %w", err)
        }
        
        // 3. 更新数据
        data.Stage = result.NextStage
        data.FreeSpinNum += result.AddFreeSpins
        
        // 4. 返回响应
        r.Response(session, result)
        
        return nil  // ✅ 成功，会自动标记为脏
    })
    
    if err != nil {
        clog.Errorf("Bonus 失败: %v", err)
        r.ResponseCode(session, code.BonusFailed)
        // ✅ 数据已自动回滚
    }
}
```

#### 示例 3：关键操作立即同步

```go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomId := req.Id
    
    err := r.roomDataManager.UpdateRoomData(roomId, func(data *RoomDataInfo) error {
        // ... spin 逻辑
        
        result, err := machine.GetSpinResult(req.Bet)
        if err != nil {
            return err
        }
        
        // 更新数据
        data.SpinNum++
        data.CurBetNum = req.Bet
        
        // 返回响应
        r.Response(session, result)
        
        return nil
    })
    
    if err != nil {
        return
    }
    
    // ✅ 关键操作立即同步
    if isJackpot || isBigWin {
        r.roomDataManager.SyncDirtyData()
    }
}
```

## 📋 最佳实践

### 1. 标记时机总结

| 场景 | 标记时机 | 方法 |
|------|---------|------|
| 普通操作 | 修改成功后 | `UpdateRoomData` |
| 需要回滚 | 修改成功后 | `UpdateRoomDataWithSnapshot` |
| 关键操作 | 修改成功后 + 立即同步 | `UpdateRoomData` + `SyncDirtyData` |
| 批量操作 | 所有修改成功后 | `UpdateRoomData` |

### 2. 错误处理

```go
// ✅ 好的做法
err := r.roomDataManager.UpdateRoomData(roomId, func(data *RoomDataInfo) error {
    // 业务逻辑
    if someCondition {
        return fmt.Errorf("validation failed")  // 不会标记为脏
    }
    
    // 修改数据
    data.SpinNum++
    
    return nil  // 成功，会标记为脏
})

// ❌ 不好的做法
data := r.roomDataManager.GetLevelSessionDataByRoomId(...)
data.SpinNum++
data.IsDirty = true  // 手动标记，容易遗漏或错误
```

### 3. 日志记录

```go
func (m *RoomDataManager) UpdateRoomData(
    roomId int32,
    updateFunc func(*RoomDataInfo) error,
) error {
    // ... 前面的代码
    
    // 执行更新
    if err := updateFunc(data); err != nil {
        clog.Warnf("更新房间数据失败: roomId=%d, err=%v", roomId, err)
        return err
    }
    
    // 标记为脏
    data.MarkDirty()
    clog.Debugf("房间数据已更新: roomId=%d, userId=%d", roomId, data.UserId)
    
    return nil
}
```

### 4. 监控指标

```go
// 记录脏数据数量
func (m *RoomDataManager) GetDirtyCount() int {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    
    count := 0
    for _, data := range m.levelSessionDataMgr {
        if data.IsDirty {
            count++
        }
    }
    return count
}

// 定期输出监控日志
func (m *RoomDataManager) StartMonitoring(interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            dirtyCount := m.GetDirtyCount()
            totalCount := len(m.levelSessionDataMgr)
            clog.Infof("房间数据统计: total=%d, dirty=%d", totalCount, dirtyCount)
        }
    }()
}
```

## 🎯 总结

### 推荐做法

**使用方案 3（包装方法）**：

1. ✅ **自动标记**：不需要手动标记，不会遗漏
2. ✅ **只在成功时标记**：失败不会标记，保证数据一致性
3. ✅ **支持回滚**：使用快照版本可以自动回滚
4. ✅ **统一错误处理**：代码更清晰
5. ✅ **易于维护**：逻辑集中在一个地方

### 关键要点

- **标记时机**：修改成功后立即标记
- **错误处理**：失败时不标记，或自动回滚
- **使用包装方法**：避免手动标记，减少错误
- **关键操作立即同步**：Jackpot、大奖等
- **定时同步**：普通操作延迟同步

这样既保证了数据一致性，又提供了良好的性能！
