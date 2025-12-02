# Feature 设计模式 - 多关卡特性管理

## 🎯 问题

`FeatureInfo` 包含了所有关卡的 Feature 字段（72+ 个），但每个关卡只需要填充自己对应的字段。如何优雅地处理这种情况？

## 📊 当前结构

### FeatureInfo 定义（protobuf）

```protobuf
message FeatureInfo {
    Amazing777Feature amazing777Feature = 1;
    Firebonus777Feature firebonus777Feature = 2;
    BigbucksFeature bigbucksFeature = 3;
    // ... 共 72+ 个不同关卡的 Feature
    Aztec105Feature aztec105Feature = 72;
}
```

### 特点

- **联合体设计**：所有关卡的 Feature 都在一个消息中
- **稀疏数据**：每次只有一个字段有值，其他都是 nil
- **类型安全**：每个关卡有自己独特的 Feature 类型

## 💡 解决方案

### 方案 1：在 Machine 中实现 GetFeature（推荐）

每个 Machine 实现自己的 `GetFeature` 方法，返回填充好的 `FeatureInfo`。

#### 实现步骤

**1. 定义接口**

```go
// machine_interface.go
type IMachine interface {
    // ... 其他方法
    
    // GetFeature 获取关卡特性信息
    GetFeature() (*pb.FeatureInfo, error)
}
```

**2. BaseMachine 提供默认实现**

```go
// machine_info_base.go
func (b *BaseMachine) GetFeature() (*pb.FeatureInfo, error) {
    // 默认返回空的 FeatureInfo
    return &pb.FeatureInfo{}, nil
}
```

**3. 每个 Machine 重写 GetFeature**

```go
// MachineInfo1.go (Amazing777)
func (m *MachineInfo1) GetFeature() (*pb.FeatureInfo, error) {
    // 只填充 Amazing777 的 Feature
    featureInfo := &pb.FeatureInfo{
        Amazing777Feature: &pb.Amazing777Feature{
            BonusInfo: &pb.Amazing777Bonus{
                BaseInfo: &pb.BonusBaseInfo{
                    WinType:     pb.WinType_WIN_BONUS,
                    RemainCount: 3,
                    WinMoney:    1000,
                },
                Result: []int32{5, 5, 10, 10, 20, 20},
                Stage: &pb.GameStage{
                    CurGameStage:  1,
                    NextGameStage: 2,
                },
            },
        },
    }
    
    return featureInfo, nil
}

// MachineInfo2.go (Firebonus777)
func (m *MachineInfo2) GetFeature() (*pb.FeatureInfo, error) {
    // 只填充 Firebonus777 的 Feature
    featureInfo := &pb.FeatureInfo{
        Firebonus777Feature: &pb.Firebonus777Feature{
            FreeSpin: &pb.FreeSpin{
                RemainCount: 10,
                TotalCount:  10,
                Win:         5000,
            },
            BonusInfo: &pb.Firebonus777Bonus{
                BaseInfo: &pb.BonusBaseInfo{
                    WinType:     pb.WinType_WIN_BONUS,
                    RemainCount: 5,
                    WinMoney:    2000,
                },
                Rewards: []int64{100, 200, 300},
                EndPos:  2,
            },
        },
    }
    
    return featureInfo, nil
}
```

**4. 在 level_room.go 中使用**

```go
func (r *ActorRoom) machineinfo(session *cproto.Session, req *pb.MachineInfo) {
    // ... 前面的代码
    
    // 获取 Feature 信息
    featureInfo, err := machine.GetFeature()
    if err != nil {
        clog.Errorf("获取 Feature 失败: %v", err)
        featureInfo = &pb.FeatureInfo{} // 使用空的 Feature
    }
    
    // 构造响应
    response := &pb.MachineInfoResponse{
        Base:      baseInfo,
        Stage:     gameStage,
        Feature:   featureInfo,  // ← 这里赋值
    }
    
    r.Response(session, response)
}
```

### 方案 2：使用工厂模式 + 配置映射

创建一个 Feature 工厂，根据 roomId 或 ruleId 创建对应的 Feature。

```go
// feature_factory.go
package machine

type FeatureFactory struct {
    featureMap map[int32]func() *pb.FeatureInfo
}

func NewFeatureFactory() *FeatureFactory {
    f := &FeatureFactory{
        featureMap: make(map[int32]func() *pb.FeatureInfo),
    }
    
    // 注册各个关卡的 Feature 创建函数
    f.featureMap[1] = createAmazing777Feature
    f.featureMap[2] = createFirebonus777Feature
    // ... 注册其他关卡
    
    return f
}

func (f *FeatureFactory) CreateFeature(ruleId int32, roomDataInfo *spinManager.RoomDataInfo) *pb.FeatureInfo {
    if creator, ok := f.featureMap[ruleId]; ok {
        return creator()
    }
    return &pb.FeatureInfo{} // 默认空 Feature
}

// 具体的创建函数
func createAmazing777Feature() *pb.FeatureInfo {
    return &pb.FeatureInfo{
        Amazing777Feature: &pb.Amazing777Feature{
            // ... 填充数据
        },
    }
}

func createFirebonus777Feature() *pb.FeatureInfo {
    return &pb.FeatureInfo{
        Firebonus777Feature: &pb.Firebonus777Feature{
            // ... 填充数据
        },
    }
}
```

### 方案 3：使用 Builder 模式

为每个关卡创建一个 FeatureBuilder。

```go
// feature_builder.go
type FeatureBuilder interface {
    Build(roomDataInfo *spinManager.RoomDataInfo) *pb.FeatureInfo
}

// Amazing777 的 Builder
type Amazing777FeatureBuilder struct{}

func (b *Amazing777FeatureBuilder) Build(roomDataInfo *spinManager.RoomDataInfo) *pb.FeatureInfo {
    return &pb.FeatureInfo{
        Amazing777Feature: &pb.Amazing777Feature{
            BonusInfo: &pb.Amazing777Bonus{
                BaseInfo: &pb.BonusBaseInfo{
                    WinType:     pb.WinType_WIN_BONUS,
                    RemainCount: roomDataInfo.FreeSpinNum,
                    WinMoney:    calculateWinMoney(roomDataInfo),
                },
            },
        },
    }
}

// Firebonus777 的 Builder
type Firebonus777FeatureBuilder struct{}

func (b *Firebonus777FeatureBuilder) Build(roomDataInfo *spinManager.RoomDataInfo) *pb.FeatureInfo {
    return &pb.FeatureInfo{
        Firebonus777Feature: &pb.Firebonus777Feature{
            FreeSpin: &pb.FreeSpin{
                RemainCount: roomDataInfo.FreeSpinNum,
                TotalCount:  roomDataInfo.SpinNum,
            },
        },
    }
}
```

## 🎨 推荐架构

### 完整实现示例

```go
// MachineInfo1.go
package machine

import (
    "github.com/cherry-game/examples/demo_cluster/internal/pb"
    spinManager "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_manager"
)

type MachineInfo1 struct {
    BaseMachine
}

// GetFeature 实现 Amazing777 的 Feature
func (m *MachineInfo1) GetFeature() (*pb.FeatureInfo, error) {
    // 从 roomDataInfo 获取数据
    roomData := m.roomDataInfo
    
    // 构建 Feature
    featureInfo := &pb.FeatureInfo{}
    
    // 根据游戏阶段填充不同的 Feature
    switch roomData.Stage {
    case 1: // Normal 阶段
        // 不需要填充 Feature
        
    case 2: // Bonus 阶段
        featureInfo.Amazing777Feature = m.buildAmazing777Bonus()
        
    case 3: // FreeSpin 阶段
        featureInfo.Amazing777Feature = m.buildAmazing777FreeSpin()
    }
    
    return featureInfo, nil
}

// buildAmazing777Bonus 构建 Bonus Feature
func (m *MachineInfo1) buildAmazing777Bonus() *pb.Amazing777Feature {
    return &pb.Amazing777Feature{
        BonusInfo: &pb.Amazing777Bonus{
            BaseInfo: &pb.BonusBaseInfo{
                WinType:     pb.WinType_WIN_BONUS,
                RemainCount: int32(m.roomDataInfo.FreeSpinNum),
                WinMoney:    0, // 从配置或计算获取
            },
            Result: []int32{5, 5, 10, 10, 20, 20, 50, 50, 100, 1000},
            Stage: &pb.GameStage{
                CurGameStage:  int32(m.roomDataInfo.Stage),
                NextGameStage: int32(m.roomDataInfo.NextStage),
            },
        },
    }
}

// buildAmazing777FreeSpin 构建 FreeSpin Feature
func (m *MachineInfo1) buildAmazing777FreeSpin() *pb.Amazing777Feature {
    return &pb.Amazing777Feature{
        BonusInfo: &pb.Amazing777Bonus{
            BaseInfo: &pb.BonusBaseInfo{
                WinType:     pb.WinType_WIN_FREESPIN,
                RemainCount: int32(m.roomDataInfo.FreeSpinNum),
                WinMoney:    0,
            },
        },
    }
}
```

## 📋 最佳实践

### 1. 数据来源

Feature 数据通常来自：

```go
// 从 roomDataInfo 获取
roomData := m.roomDataInfo
remainCount := roomData.FreeSpinNum
totalCount := roomData.SpinNum
stage := roomData.Stage

// 从配置获取
roomConfig := m.roomConfig
bonusConfig := m.reelCofig.BonusConfig

// 从计算获取
winMoney := m.calculateBonusWin()
```

### 2. 条件填充

```go
func (m *MachineInfo1) GetFeature() (*pb.FeatureInfo, error) {
    featureInfo := &pb.FeatureInfo{}
    
    // 只在特定阶段填充 Feature
    if m.roomDataInfo.Stage == STAGE_BONUS {
        featureInfo.Amazing777Feature = m.buildBonus()
    }
    
    // 或者根据是否有 FreeSpin
    if m.roomDataInfo.FreeSpinNum > 0 {
        featureInfo.Amazing777Feature = &pb.Amazing777Feature{
            // ... 填充 FreeSpin 相关数据
        }
    }
    
    return featureInfo, nil
}
```

### 3. 复用通用逻辑

```go
// 在 BaseMachine 中提供通用方法
func (b *BaseMachine) buildBaseInfo() *pb.BonusBaseInfo {
    return &pb.BonusBaseInfo{
        WinType:     pb.WinType_WIN_BONUS,
        RemainCount: int32(b.roomDataInfo.FreeSpinNum),
        WinMoney:    b.calculateWinMoney(),
    }
}

// 在具体 Machine 中使用
func (m *MachineInfo1) GetFeature() (*pb.FeatureInfo, error) {
    return &pb.FeatureInfo{
        Amazing777Feature: &pb.Amazing777Feature{
            BonusInfo: &pb.Amazing777Bonus{
                BaseInfo: m.buildBaseInfo(), // ← 复用
                Result:   m.getBonusResult(),
            },
        },
    }, nil
}
```

### 4. 错误处理

```go
func (m *MachineInfo1) GetFeature() (*pb.FeatureInfo, error) {
    featureInfo := &pb.FeatureInfo{}
    
    // 检查数据有效性
    if m.roomDataInfo == nil {
        return featureInfo, fmt.Errorf("roomDataInfo is nil")
    }
    
    // 安全地填充数据
    if m.roomDataInfo.Stage == STAGE_BONUS {
        bonusFeature, err := m.buildBonusFeature()
        if err != nil {
            clog.Errorf("构建 Bonus Feature 失败: %v", err)
            return featureInfo, err
        }
        featureInfo.Amazing777Feature = bonusFeature
    }
    
    return featureInfo, nil
}
```

## 🔧 实用工具

### Feature 辅助函数

```go
// feature_helper.go
package machine

// IsFeatureEmpty 检查 FeatureInfo 是否为空
func IsFeatureEmpty(feature *pb.FeatureInfo) bool {
    if feature == nil {
        return true
    }
    
    // 检查所有字段是否都是 nil
    return feature.Amazing777Feature == nil &&
           feature.Firebonus777Feature == nil &&
           feature.BigbucksFeature == nil
           // ... 检查其他字段
}

// GetFeatureType 获取 Feature 类型
func GetFeatureType(feature *pb.FeatureInfo) string {
    if feature == nil {
        return "none"
    }
    
    if feature.Amazing777Feature != nil {
        return "amazing777"
    }
    if feature.Firebonus777Feature != nil {
        return "firebonus777"
    }
    // ... 检查其他类型
    
    return "unknown"
}
```

## 📊 配置映射表

建议在配置文件中维护 roomId 到 Feature 类型的映射：

```json
{
  "room_feature_mapping": {
    "1001": "amazing777",
    "1002": "amazing777",
    "2001": "firebonus777",
    "2002": "firebonus777",
    "3001": "bigbucks"
  }
}
```

## 🎯 总结

### 推荐方案

**方案 1（在 Machine 中实现）** 是最推荐的，因为：

1. ✅ **封装性好**：每个 Machine 管理自己的 Feature
2. ✅ **类型安全**：编译时检查
3. ✅ **易于维护**：Feature 逻辑和 Machine 逻辑在一起
4. ✅ **灵活性高**：可以根据游戏状态动态填充

### 关键要点

1. **稀疏数据**：`FeatureInfo` 中只有一个字段有值
2. **条件填充**：根据游戏阶段决定是否填充 Feature
3. **数据来源**：从 `roomDataInfo`、配置、计算中获取数据
4. **错误处理**：确保数据有效性

### 示例代码

```go
// level_room.go
featureInfo, err := machine.GetFeature()
if err != nil {
    featureInfo = &pb.FeatureInfo{}
}

response := &pb.MachineInfoResponse{
    Base:    baseInfo,
    Stage:   gameStage,
    Feature: featureInfo,  // ← 每个关卡只填充自己的字段
}
```

这样设计既保持了类型安全，又实现了灵活的多关卡支持！
