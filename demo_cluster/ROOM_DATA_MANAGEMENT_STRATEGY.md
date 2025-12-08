# RoomDataInfo 数据管理策略

## 🎯 问题分析

### 当前情况

```go
// level_room.go
roomDataInfo := r.roomDataManager.GetLevelSessionDataByRoomId(int32(userInfo.UserId), roomId)

// roomDataInfo 是指针，指向 roomDataManager 中的数据
// 在 spin 过程中会修改 roomDataInfo 的字段
// 问题：如何保证数据一致性和持久化？
```

### 核心问题

1. **指针共享**：`roomDataInfo` 是指针，多个地方可能同时访问
2. **并发安全**：Actor 是单线程的，但需要考虑定时器等异步操作
3. **数据持久化**：何时同步到数据库？
4. **数据一致性**：如何保证内存和数据库的一致性？
5. **错误恢复**：如果 spin 失败，如何回滚数据？

## 📊 数据流分析

### 当前数据流

```
┌─────────────────────────────────────────────────────────┐
│                    ActorRoom                             │
│  ┌────────────────────────────────────────────┐         │
│  │         roomDataManager                    │         │
│  │  map[roomId]*RoomDataInfo                  │         │
│  │    ↓                                        │         │
│  │  roomDataInfo (指针)                        │         │
│  └────────────────────────────────────────────┘         │
│                    ↓                                     │
│              machineinfo()                               │
│                    ↓                                     │
│              spin() ← 修改 roomDataInfo                  │
│                    ↓                                     │
│              bonus() ← 修改 roomDataInfo                 │
│                    ↓                                     │
│              何时同步到数据库？                            │
└─────────────────────────────────────────────────────────┘
```

## 💡 推荐方案

### 方案 1：写时标记 + 定时同步（推荐）⭐

#### 核心思路

1. **IsDirty 标记**：修改数据时设置脏标记
2. **定时同步**：每隔一段时间同步脏数据到数据库
3. **关键节点同步**：在关键操作后立即同步

#### 实现细节

```go
// level_data_types.go
type RoomDataInfo struct {
    // ... 其他字段
    
    // 元数据
    IsDirty   bool  `json:"-"`          // 脏数据标记（不序列化）
    UpdatedAt int64 `json:"updated_at"` // 最后更新时间
    Version   int   `json:"version"`    // 版本号（乐观锁）
}

// 标记为脏数据
func (r *RoomDataInfo) MarkDirty() {
    r.IsDirty = true
    r.UpdatedAt = time.Now().Unix()
}

// 清除脏标记
func (r *RoomDataInfo) ClearDirty() {
    r.IsDirty = false
}
```

```go
// room_data_manager.go
type RoomDataManager struct {
    levelSessionDataMgr map[int32]*RoomDataInfo
    mutex               sync.RWMutex  // 读写锁
    syncTimer           *time.Timer   // 同步定时器
}

// 更新数据（自动标记为脏）
func (m *RoomDataManager) UpdateRoomData(roomId int32, updateFunc func(*RoomDataInfo)) {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    if data, ok := m.levelSessionDataMgr[roomId]; ok {
        updateFunc(data)
        data.MarkDirty()
    }
}

// 同步脏数据到数据库
func (m *RoomDataManager) SyncDirtyData() error {
    m.mutex.RLock()
    dirtyData := make([]*RoomDataInfo, 0)
    for _, data := range m.levelSessionDataMgr {
        if data.IsDirty {
            dirtyData = append(dirtyData, data)
        }
    }
    m.mutex.RUnlock()
    
    // 批量保存到数据库
    for _, data := range dirtyData {
        if err := db.SaveRoomData(data); err != nil {
            clog.Errorf("保存房间数据失败: %v", err)
            continue
        }
        data.ClearDirty()
    }
    
    return nil
}

// 启动定时同步
func (m *RoomDataManager) StartAutoSync(interval time.Duration) {
    ticker := time.NewTicker(interval)
    go func() {
        for range ticker.C {
            m.SyncDirtyData()
        }
    }()
}
```

```go
// level_room.go
func (r *ActorRoom) OnInit() {
    // ... 其他初始化
    
    // 启动定时同步（每 30 秒）
    r.roomDataManager.StartAutoSync(30 * time.Second)
}

func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomId := req.Id
    
    // 1. 获取房间数据
    roomDataInfo := r.roomDataManager.GetLevelSessionDataByRoomId(
        int32(session.Uid), roomId)
    
    // 2. 执行 spin 逻辑
    machine := spinEngine.CreateMachineByType(...)
    spinResult, err := machine.GetSpinResult(req.Bet)
    if err != nil {
        // 错误处理
        return
    }
    
    // 3. 更新房间数据（自动标记为脏）
    r.roomDataManager.UpdateRoomData(roomId, func(data *RoomDataInfo) {
        data.SpinNum++
        data.CurBetNum = req.Bet
        // 更新其他字段
    })
    
    // 4. 关键操作后立即同步（可选）
    if spinResult.IsJackpot {
        r.roomDataManager.SyncDirtyData()
    }
    
    // 5. 返回响应
    r.Response(session, spinResult)
}
```

#### 优点

- ✅ **性能好**：不是每次都写数据库
- ✅ **数据安全**：定时同步保证数据不丢失
- ✅ **灵活性高**：可以在关键节点立即同步
- ✅ **易于实现**：代码改动小

#### 缺点

- ⚠️ 可能丢失最近 30 秒的数据（如果服务器崩溃）
- ⚠️ 需要额外的定时器管理

### 方案 2：事务式更新（最安全）

#### 核心思路

1. **快照备份**：操作前备份数据
2. **原子操作**：操作成功后提交，失败则回滚
3. **立即持久化**：每次操作都同步到数据库

#### 实现细节

```go
// room_data_manager.go
type RoomDataManager struct {
    levelSessionDataMgr map[int32]*RoomDataInfo
    mutex               sync.RWMutex
}

// 事务式更新
func (m *RoomDataManager) TransactionalUpdate(
    roomId int32,
    updateFunc func(*RoomDataInfo) error,
) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    data, ok := m.levelSessionDataMgr[roomId]
    if !ok {
        return fmt.Errorf("room data not found")
    }
    
    // 1. 创建快照
    snapshot := *data
    
    // 2. 执行更新
    if err := updateFunc(data); err != nil {
        // 回滚
        *data = snapshot
        return err
    }
    
    // 3. 持久化到数据库
    if err := db.SaveRoomData(data); err != nil {
        // 回滚
        *data = snapshot
        return err
    }
    
    return nil
}
```

```go
// level_room.go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomId := req.Id
    
    // 事务式更新
    err := r.roomDataManager.TransactionalUpdate(roomId, func(data *RoomDataInfo) error {
        // 1. 执行 spin 逻辑
        machine := spinEngine.CreateMachineByType(...)
        spinResult, err := machine.GetSpinResult(req.Bet)
        if err != nil {
            return err
        }
        
        // 2. 更新数据
        data.SpinNum++
        data.CurBetNum = req.Bet
        data.FreeSpinNum = spinResult.FreeSpinNum
        
        // 3. 返回响应
        r.Response(session, spinResult)
        return nil
    })
    
    if err != nil {
        clog.Errorf("Spin 失败: %v", err)
        r.ResponseCode(session, code.SpinFailed)
    }
}
```

#### 优点

- ✅ **数据安全**：每次操作都持久化
- ✅ **一致性强**：支持回滚
- ✅ **易于调试**：数据变更清晰

#### 缺点

- ⚠️ **性能差**：每次都写数据库
- ⚠️ **延迟高**：影响用户体验

### 方案 3：混合模式（平衡方案）

#### 核心思路

1. **普通操作**：使用脏标记 + 定时同步
2. **关键操作**：立即同步到数据库
3. **批量操作**：使用事务

#### 实现细节

```go
// room_data_manager.go
type SyncStrategy int

const (
    SyncStrategyDeferred  SyncStrategy = 1 // 延迟同步
    SyncStrategyImmediate SyncStrategy = 2 // 立即同步
    SyncStrategyBatch     SyncStrategy = 3 // 批量同步
)

func (m *RoomDataManager) UpdateRoomData(
    roomId int32,
    strategy SyncStrategy,
    updateFunc func(*RoomDataInfo) error,
) error {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    data, ok := m.levelSessionDataMgr[roomId]
    if !ok {
        return fmt.Errorf("room data not found")
    }
    
    // 执行更新
    if err := updateFunc(data); err != nil {
        return err
    }
    
    // 根据策略同步
    switch strategy {
    case SyncStrategyDeferred:
        // 只标记为脏，等待定时同步
        data.MarkDirty()
        
    case SyncStrategyImmediate:
        // 立即同步到数据库
        if err := db.SaveRoomData(data); err != nil {
            return err
        }
        data.ClearDirty()
        
    case SyncStrategyBatch:
        // 标记为脏，但触发批量同步
        data.MarkDirty()
        go m.SyncDirtyData()
    }
    
    return nil
}
```

```go
// level_room.go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomId := req.Id
    
    // 普通 spin：延迟同步
    strategy := spinManager.SyncStrategyDeferred
    
    // 关键操作：立即同步
    if isJackpot || isBigWin {
        strategy = spinManager.SyncStrategyImmediate
    }
    
    err := r.roomDataManager.UpdateRoomData(roomId, strategy, func(data *RoomDataInfo) error {
        // 更新逻辑
        data.SpinNum++
        data.CurBetNum = req.Bet
        return nil
    })
    
    if err != nil {
        clog.Errorf("更新失败: %v", err)
    }
}
```

## 🔧 数据库设计

### 表结构

```sql
CREATE TABLE room_session_data (
    id BIGSERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    room_id INT NOT NULL,
    
    -- 游戏数据
    cur_bet_num BIGINT DEFAULT 0,
    spe_spin_bet BIGINT DEFAULT 0,
    stage INT DEFAULT 0,
    next_stage INT DEFAULT 0,
    free_spin_num INT DEFAULT 0,
    spin_num INT DEFAULT 0,
    
    -- 元数据
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INT DEFAULT 1,
    
    -- 索引
    UNIQUE(user_id, room_id),
    INDEX idx_user_id (user_id),
    INDEX idx_updated_at (updated_at)
);
```

### 数据访问层

```go
// db/room_session_table.go
package db

type RoomSessionTable struct {
    ID          int64     `gorm:"primaryKey"`
    UserID      int32     `gorm:"index"`
    RoomID      int32     `gorm:"index"`
    CurBetNum   int64
    SpeSpinBet  int64
    Stage       int
    NextStage   int
    FreeSpinNum int
    SpinNum     int
    CreatedAt   time.Time
    UpdatedAt   time.Time
    Version     int
}

// 保存房间数据
func SaveRoomData(data *spinManager.RoomDataInfo) error {
    table := &RoomSessionTable{
        UserID:      data.UserId,
        RoomID:      data.RoomId,
        CurBetNum:   data.CurBetNum,
        SpeSpinBet:  data.SpeSpinBet,
        Stage:       data.Stage,
        NextStage:   data.NextStage,
        FreeSpinNum: data.FreeSpinNum,
        SpinNum:     data.SpinNum,
        Version:     data.Version + 1,
    }
    
    // 使用乐观锁
    result := GetDB().Model(&RoomSessionTable{}).
        Where("user_id = ? AND room_id = ? AND version = ?", 
            data.UserId, data.RoomId, data.Version).
        Updates(table)
    
    if result.RowsAffected == 0 {
        return fmt.Errorf("version conflict")
    }
    
    data.Version++
    return nil
}

// 加载房间数据
func LoadRoomData(userId, roomId int32) (*spinManager.RoomDataInfo, error) {
    table := &RoomSessionTable{}
    err := GetDB().Where("user_id = ? AND room_id = ?", userId, roomId).
        First(table).Error
    
    if err != nil {
        if errors.Is(err, gorm.ErrRecordNotFound) {
            // 创建新数据
            return &spinManager.RoomDataInfo{
                UserId: userId,
                RoomId: roomId,
            }, nil
        }
        return nil, err
    }
    
    return &spinManager.RoomDataInfo{
        UserId:      table.UserID,
        RoomId:      table.RoomID,
        CurBetNum:   table.CurBetNum,
        SpeSpinBet:  table.SpeSpinBet,
        Stage:       table.Stage,
        NextStage:   table.NextStage,
        FreeSpinNum: table.FreeSpinNum,
        SpinNum:     table.SpinNum,
        Version:     table.Version,
    }, nil
}
```

## 📋 最佳实践

### 1. 数据加载时机

```go
func (r *ActorRoom) OnInit() {
    // Actor 初始化时不加载数据
    // 等到第一次使用时才加载（懒加载）
}

func (r *ActorRoom) machineinfo(session *cproto.Session, req *pb.MachineInfo) {
    // 第一次访问时从数据库加载
    roomDataInfo := r.roomDataManager.GetOrLoadRoomData(userId, roomId)
}
```

### 2. 数据同步时机

```go
// 定时同步（每 30 秒）
r.roomDataManager.StartAutoSync(30 * time.Second)

// 关键操作立即同步
if isJackpot || isBigWin || isBonus {
    r.roomDataManager.SyncDirtyData()
}

// Actor 退出时同步
func (r *ActorRoom) OnStop() {
    r.roomDataManager.SyncDirtyData()
}
```

### 3. 并发控制

```go
// Actor 是单线程的，但定时器是异步的
// 使用读写锁保护数据

type RoomDataManager struct {
    levelSessionDataMgr map[int32]*RoomDataInfo
    mutex               sync.RWMutex  // 读写锁
}

// 读操作
func (m *RoomDataManager) GetRoomData(roomId int32) *RoomDataInfo {
    m.mutex.RLock()
    defer m.mutex.RUnlock()
    return m.levelSessionDataMgr[roomId]
}

// 写操作
func (m *RoomDataManager) UpdateRoomData(roomId int32, updateFunc func(*RoomDataInfo)) {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    if data, ok := m.levelSessionDataMgr[roomId]; ok {
        updateFunc(data)
    }
}
```

### 4. 错误处理

```go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    // 1. 验证数据
    roomDataInfo := r.roomDataManager.GetRoomData(roomId)
    if roomDataInfo == nil {
        r.ResponseCode(session, code.RoomDataNotFound)
        return
    }
    
    // 2. 执行操作
    err := r.roomDataManager.UpdateRoomData(roomId, func(data *RoomDataInfo) error {
        // 业务逻辑
        return nil
    })
    
    // 3. 错误处理
    if err != nil {
        clog.Errorf("Spin 失败: %v", err)
        r.ResponseCode(session, code.SpinFailed)
        return
    }
}
```

### 5. 数据清理

```go
// 定期清理长时间未使用的数据
func (m *RoomDataManager) CleanupInactiveData(timeout time.Duration) {
    m.mutex.Lock()
    defer m.mutex.Unlock()
    
    now := time.Now().Unix()
    for roomId, data := range m.levelSessionDataMgr {
        if now - data.UpdatedAt > int64(timeout.Seconds()) {
            // 同步到数据库
            if data.IsDirty {
                db.SaveRoomData(data)
            }
            // 从内存中删除
            delete(m.levelSessionDataMgr, roomId)
        }
    }
}
```

## 🎯 推荐方案总结

### 最佳实践组合

1. **使用方案 1（写时标记 + 定时同步）作为基础**
2. **关键操作使用立即同步**
3. **Actor 退出时强制同步**
4. **使用乐观锁防止并发冲突**
5. **定期清理不活跃数据**

### 实施步骤

1. ✅ 在 `RoomDataInfo` 中添加 `IsDirty`、`UpdatedAt`、`Version` 字段
2. ✅ 在 `RoomDataManager` 中实现 `UpdateRoomData` 和 `SyncDirtyData` 方法
3. ✅ 在 `ActorRoom.OnInit` 中启动定时同步
4. ✅ 在 `spin`、`bonus`、`collect` 中使用 `UpdateRoomData` 更新数据
5. ✅ 在 `ActorRoom.OnStop` 中同步所有脏数据
6. ✅ 实现数据库访问层（`SaveRoomData`、`LoadRoomData`）

### 关键要点

- **指针是安全的**：因为 Actor 是单线程的，只要使用锁保护异步操作
- **不需要每次都存储**：使用脏标记 + 定时同步即可
- **关键操作立即同步**：Jackpot、大奖等重要操作
- **使用乐观锁**：防止并发冲突
- **定期清理内存**：避免内存泄漏

这样既保证了性能，又保证了数据安全！
