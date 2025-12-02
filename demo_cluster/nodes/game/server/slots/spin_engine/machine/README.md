# Slots Machine 工厂模式设计

## 架构概览

本设计使用**工厂模式 + 策略模式**实现不同房间使用不同的 Spin 引擎逻辑。

## 核心组件

### 1. IMachine 接口
定义所有 Machine 必须实现的方法：
- `InitData()` - 初始化机器数据
- `GetBase()` - 获取基础信息
- `GetSpinResult(bet)` - 获取 Spin 结果
- `GetFeature()` - 获取特性信息
- 等等...

### 2. BaseMachine 基类
提供所有 Machine 的通用功能和默认实现：
- 房间配置管理
- Reel 配置管理
- 通用数据初始化
- 默认方法实现

### 3. MachineInfo1 / MachineInfo2
具体的 Machine 实现，继承 BaseMachine 并重写特定方法：
- `MachineInfo1` - 适用于规则 ID = 1 的房间
- `MachineInfo2` - 适用于规则 ID = 2 的房间

### 4. MachineFactory 工厂
根据 roomId 自动创建对应的 Machine 实例。

## 使用方式

### 在 level_room.go 中使用

```go
func (r *ActorRoom) machineinfo(session *cproto.Session, req *pb.MachineInfo) {
    roomId := req.Id
    
    // 获取房间数据
    roomDataInfo := r.roomDataManager.GetOrCreateRoomData(userId, roomId)
    
    // 工厂自动创建对应的 Machine
    machine, err := spinEngine.CreateMachineByRoomId(roomId, session, roomDataInfo)
    if err != nil {
        // 处理错误
        return
    }
    
    // 使用 Machine 获取信息
    baseInfo, _ := machine.GetBase()
    gameStage, _ := machine.ConvertStage()
    
    // 返回响应
    response := &pb.MachineInfoResponse{
        Id:        roomId,
        Base:      baseInfo,
        GameStage: gameStage,
    }
    r.Response(session, response)
}
```

### 在 spin 方法中使用

```go
func (r *ActorRoom) spin(session *cproto.Session, req *pb.Spin) {
    roomId := req.Id
    bet := req.Bet
    
    // 创建 Machine
    machine, err := spinEngine.CreateMachineByRoomId(roomId, session, roomDataInfo)
    if err != nil {
        return
    }
    
    // 调用 Spin 引擎计算结果
    spinResult, err := machine.GetSpinResult(bet)
    if err != nil {
        return
    }
    
    // 返回结果
    r.Response(session, spinResult)
}
```

## 工厂选择逻辑

工厂根据房间配置的 `RuleID` 字段来决定使用哪个 Machine：

```go
switch ruleId {
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

## 扩展新的 Machine

### 1. 创建新的 Machine 类

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
    // ...
    return response, nil
}
```

### 2. 在工厂中注册

```go
// machine_factory.go
switch ruleId {
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

## 优势

1. **解耦**: 不同房间的逻辑完全独立
2. **扩展性**: 新增 Machine 类型无需修改现有代码
3. **可维护性**: 每个 Machine 的逻辑清晰独立
4. **灵活性**: 可以根据配置动态选择 Machine
5. **复用性**: BaseMachine 提供通用功能，减少重复代码

## 配置映射

可以在数据库或配置文件中维护 roomId 到 Machine 类型的映射：

```json
{
  "room_machine_mapping": {
    "1001": "MachineInfo1",
    "1002": "MachineInfo1",
    "2001": "MachineInfo2",
    "2002": "MachineInfo2"
  }
}
```

或者使用房间配置表的 `RuleID` 字段来区分。

## 注意事项

1. 每个 Machine 实例都会调用 `InitData()` 进行初始化
2. Machine 实例是临时的，每次请求都会创建新实例
3. 如果需要缓存 Machine 实例，可以在 ActorRoom 中维护一个 map
4. 确保每个 Machine 的 Spin 算法是无状态的或正确管理状态
