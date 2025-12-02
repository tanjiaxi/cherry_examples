# Machine 工厂模式使用指南

## 📋 概述

本文档介绍如何在 `level_room.go` 中使用工厂模式动态调用不同的 Machine（MachineInfo1、MachineInfo2 等）。

## 🏗️ 架构设计

### 核心组件

```
┌─────────────────────────────────────────────────────────┐
│                    IMachine 接口                         │
│  - InitData()                                           │
│  - GetBase()                                            │
│  - GetSpinResult(bet)                                   │
│  - ConvertStage()                                       │
│  - ...                                                  │
└─────────────────────────────────────────────────────────┘
                            ▲
                            │ 实现
            ┌───────────────┼───────────────┐
            │               │               │
┌───────────▼────────┐ ┌───▼──────────┐ ┌──▼──────────────┐
│   BaseMachine      │ │ MachineInfo1 │ │  MachineInfo2   │
│  (基础实现)         │ │ (规则1)      │ │  (规则2)        │
│  - 通用方法        │ │ - 特殊逻辑   │ │  - 特殊逻辑     │
│  - 配置管理        │ │ - 重写方法   │ │  - 重写方法     │
└────────────────────┘ └──────────────┘ └─────────────────┘
                            ▲
                            │ 创建
                ┌───────────┴──────────┐
                │  MachineFactory      │
                │  根据 roomId/version │
                │  动态创建 Machine    │
                └──────────────────────┘
```

## 🎯 使用方式

### 1. 在 machineinfo 方法中使用

```go
func (r *ActorRoom) machineinfo(session *cproto.Session, req *pb.MachineInfo) {
    roomId := req.Id
    
    // 1. 验证房间配置
    n2CfgRoomlist, error := configCacheSlots.GetInstance().GetRoomConfig(roomId)
    if error != nil || n2CfgRoomlist == nil {
        // 返回错误
        return
    }
    
    // 2. 获取用户信息
    userInfo := rpcGame.GetUserInfo(r.Actor, session)
    if userInfo == nil {
        // 返回错误
        return
    }
    
    // 3. 获取房间数据
    roomDataInfo := r.roomDataManager.GetLevelSessionDataByRoomId(
        int32(userInfo.UserID), roomId)
    
    // 4. 🎯 使用工厂创建 Machine（自动根据配置选择 MachineInfo1 或 MachineInfo2）
    machine, err := spinEngine.CreateMachineByRoomId(roomId, session, roomDataInfo)
    if err != nil {
        // 处理错误
        return
    }
    
    // 5. 使用 Machine 获取信息
    baseInfo, err := machine.GetBase()
    gameStage, err := machine.ConvertStage()
    
    // 6. 返回响应
    response := &pb.MachineInfoResponse{
        // 填充字段
    }
    r.Response(session, response)
}
```

### 2. 在 spin 方法中使用

```go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomId := req.Id
    bet := req.Bet
    
    // 获取房间数据
    userInfo := rpcGame.GetUserInfo(r.Actor, session)
    roomDataInfo := r.roomDataManager.GetLevelSessionDataByRoomId(
        int32(userInfo.UserID), roomId)
    
    // 🎯 创建 Machine
    machine, err := spinEngine.CreateMachineByRoomId(roomId, session, roomDataInfo)
    if err != nil {
        clog.Errorf("创建 Machine 失败: %v", err)
        return
    }
    
    // 🎰 调用 Spin 引擎计算结果
    spinResult, err := machine.GetSpinResult(bet)
    if err != nil {
        clog.Errorf("Spin 计算失败: %v", err)
        return
    }
    
    // 返回结果
    r.Response(session, spinResult)
}
```

## ⚙️ 工厂选择逻辑

工厂根据房间配置的 `Version` 字段来决定使用哪个 Machine：

```go
// machine_factory.go
version := roomConfig.Version

switch version {
case 1:
    // 使用 MachineInfo1
    machine = &MachineInfo1{}
case 2:
    // 使用 MachineInfo2
    machine = &MachineInfo2{}
default:
    // 默认使用 MachineInfo1
    machine = &MachineInfo1{}
}
```

### 修改选择规则

如果你想使用其他字段来区分，可以修改 `machine_factory.go`：

```go
// 方案1: 使用 RoomID 范围
if roomId >= 1000 && roomId < 2000 {
    machine = &MachineInfo1{}
} else if roomId >= 2000 && roomId < 3000 {
    machine = &MachineInfo2{}
}

// 方案2: 使用配置表中的自定义字段
// 假设你在配置表中添加了 MachineType 字段
machineType := roomConfig.MachineType
switch machineType {
case "classic":
    machine = &MachineInfo1{}
case "advanced":
    machine = &MachineInfo2{}
}

// 方案3: 使用映射表
machineMap := map[int32]string{
    1001: "MachineInfo1",
    1002: "MachineInfo1",
    2001: "MachineInfo2",
    2002: "MachineInfo2",
}
```

## 🔧 扩展新的 Machine

### 步骤 1: 创建新的 Machine 类

```go
// MachineInfo3.go
package machine

type MachineInfo3 struct {
    BaseMachine
}

func NewMachineInfo3(base BaseMachine) *MachineInfo3 {
    return &MachineInfo3{
        BaseMachine: base,
    }
}

// 重写需要自定义的方法
func (m *MachineInfo3) GetSpinResult(bet int64) (*pb.SpinResponse, error) {
    // Machine3 特有的 Spin 逻辑
    // 例如：特殊的 Wild 符号、级联消除等
    
    response := &pb.SpinResponse{
        Id: m.roomId,
        // ... 填充结果
    }
    
    return response, nil
}

// 可以重写其他方法
func (m *MachineInfo3) GetBase() (*pb.BaseInfo, error) {
    // Machine3 特有的基础信息
    baseInfo, err := m.BaseMachine.GetBase()
    if err != nil {
        return nil, err
    }
    
    // 添加 Machine3 特有的配置
    // baseInfo.SpecialFeature = "cascade"
    
    return baseInfo, nil
}
```

### 步骤 2: 在工厂中注册

```go
// machine_factory.go
switch version {
case 1:
    machine = &MachineInfo1{}
case 2:
    machine = &MachineInfo2{}
case 3:
    machine = &MachineInfo3{}  // 新增
default:
    machine = &MachineInfo1{}
}
```

## 📊 配置示例

### 数据库配置表

```sql
-- n2_cfg_roomlist 表
INSERT INTO n2_cfg_roomlist (room_id, room_name, version, ...) VALUES
(1001, 'Classic Slots', 1, ...),  -- 使用 MachineInfo1
(1002, 'Fruit Machine', 1, ...),  -- 使用 MachineInfo1
(2001, 'Advanced Slots', 2, ...),  -- 使用 MachineInfo2
(2002, 'Mega Slots', 2, ...);      -- 使用 MachineInfo2
```

## 🎮 不同 Machine 的差异示例

### MachineInfo1 - 经典老虎机

```go
func (m *MachineInfo1) GetSpinResult(bet int64) (*pb.SpinResponse, error) {
    // 经典 3x3 老虎机逻辑
    // - 简单的连线赔付
    // - 基础 Wild 符号
    // - 固定赔率表
    
    symbols := m.generateClassicSymbols()  // 生成经典符号
    win := m.calculateClassicWin(symbols, bet)
    
    return &pb.SpinResponse{
        Id:       m.roomId,
        Results:  []*pb.SpinResult{{Symbols: symbols, Win: win}},
        Totalwin: win,
    }, nil
}
```

### MachineInfo2 - 高级老虎机

```go
func (m *MachineInfo2) GetSpinResult(bet int64) (*pb.SpinResponse, error) {
    // 高级 5x3 老虎机逻辑
    // - 多种 Feature（FreeSpin、Bonus、Multiplier）
    // - 动态 Wild 符号
    // - 级联消除
    // - 累积 Jackpot
    
    symbols := m.generateAdvancedSymbols()  // 生成高级符号
    
    // 检查是否触发 FreeSpin
    if m.checkFreeSpinTrigger(symbols) {
        m.roomDataInfo.FreeSpinNum = 10
    }
    
    // 计算赢钱（包含 Multiplier）
    win := m.calculateAdvancedWin(symbols, bet)
    
    // 检查 Jackpot
    jackpotWin := m.checkJackpot(bet)
    
    return &pb.SpinResponse{
        Id:       m.roomId,
        Results:  []*pb.SpinResult{{Symbols: symbols, Win: win + jackpotWin}},
        Totalwin: win + jackpotWin,
    }, nil
}
```

## 🔍 调试技巧

### 1. 日志记录

```go
func (r *ActorRoom) machineinfo(session *cproto.Session, req *pb.MachineInfo) {
    // ...
    
    machine, err := spinEngine.CreateMachineByRoomId(roomId, session, roomDataInfo)
    if err != nil {
        return
    }
    
    // 记录使用的 Machine 类型
    clog.Infof("使用 Machine 类型: %T, roomId=%d", machine, roomId)
    
    // ...
}
```

### 2. 性能监控

```go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    start := time.Now()
    defer func() {
        duration := time.Since(start)
        clog.Infof("Spin 耗时: %v, roomId=%d", duration, req.Id)
    }()
    
    // Spin 逻辑
    // ...
}
```

## ⚠️ 注意事项

1. **Machine 实例是临时的**：每次请求都会创建新的 Machine 实例
2. **状态管理**：游戏状态应该存储在 `RoomDataInfo` 中，而不是 Machine 中
3. **线程安全**：Actor 模型保证了单线程处理，无需担心并发问题
4. **配置热更新**：修改配置后需要重启服务或实现热更新机制
5. **错误处理**：确保每个步骤都有完善的错误处理

## 📈 性能优化建议

### 1. 缓存 Machine 实例（可选）

```go
type ActorRoom struct {
    pomelo.ActorBase
    curRoomId       int32
    roomDataManager *spinManager.RoomDataManager
    machineCache    map[int32]spinEngine.IMachine  // 缓存 Machine
}

func (r *ActorRoom) getMachine(roomId int32, session *cproto.Session, 
    roomDataInfo *spinManager.RoomDataInfo) (spinEngine.IMachine, error) {
    
    // 从缓存获取
    if machine, ok := r.machineCache[roomId]; ok {
        return machine, nil
    }
    
    // 创建新实例
    machine, err := spinEngine.CreateMachineByRoomId(roomId, session, roomDataInfo)
    if err != nil {
        return nil, err
    }
    
    // 存入缓存
    r.machineCache[roomId] = machine
    
    return machine, nil
}
```

### 2. 预加载配置

```go
func (b *BaseMachine) InitData() error {
    // 预加载所有需要的配置
    // 避免在 Spin 过程中频繁查询
    
    // 缓存房间配置
    // 缓存 Reel 配置
    // 缓存赔率表
    
    return nil
}
```

## 🎉 总结

通过工厂模式，你可以：

✅ **动态选择**：根据 roomId 自动选择对应的 Machine  
✅ **解耦逻辑**：不同房间的逻辑完全独立  
✅ **易于扩展**：新增 Machine 类型无需修改现有代码  
✅ **统一接口**：所有 Machine 实现相同的接口  
✅ **代码复用**：BaseMachine 提供通用功能  

现在你可以在 `machineinfo`、`spin`、`bonus` 等方法中使用工厂模式来动态调用不同的 Machine 了！
