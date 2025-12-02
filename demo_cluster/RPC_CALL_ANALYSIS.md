# Cherry 框架 RPC 调用机制深度解析

## 🎯 问题

为什么 `GetUserInfo` 调用 `getPlayerData` 时，`userInfo` 不能赋值成功？

## 📚 核心概念

### 1. Local 调用 vs Remote 调用

Cherry 框架中有两种 Actor 方法调用方式：

```go
// Local 调用：来自客户端的请求，需要 Session
p.Local().Register("enter", p.playerEnter)
// 函数签名：func(session *cproto.Session, req *pb.Request)

// Remote 调用：Actor 之间的 RPC 调用，不需要 Session
p.Remote().Register("getPlayerData", p.getPlayerData)
// 函数签名：func(req *pb.Request) (response, code)
```

## 🔍 问题分析

### 你的代码现状

#### 1. RPC 调用方（game.go）

```go
func GetUserInfo(a cfacade.IActor, session *cproto.Session) *tableModel.SlotsUser {
    userInfo := &tableModel.SlotsUser{}
    targetPath := cfacade.NewChildPath("", playerActor, session.Uid)
    
    // ❌ 问题：CallWait 的第4个参数期望接收返回值
    a.CallWait(targetPath, getPlayerData, &pb.Int32{
        Value: int32(session.Uid),
    }, userInfo)  // ← userInfo 期望被填充
    
    return userInfo
}
```

#### 2. RPC 被调用方（actor_player.go）

```go
// Remote 方法注册
p.Remote().Register("getPlayerData", p.getPlayerData)

// ✅ 正确的函数签名
func (p *actorPlayer) getPlayerData(msg *pb.Int32) (*tableModel.SlotsUser, int32) {
    if p.playerData == nil || p.playerData.UserID == 0 {
        err := p.loadPlayerData(msg.Value)
        if err != nil {
            return nil, code.PlayerIDError
        }
    }
    return &p.playerData.SlotsUser, code.OK
}
```

### 问题根源

让我们深入 `invoke.go` 的 `InvokeRemoteFunc` 来理解：

```go
func InvokeRemoteFunc(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) {
    // 1. 编码参数
    EncodeRemoteArgs(app, fi, m)
    
    // 2. 准备反射调用的参数
    values := make([]reflect.Value, fi.InArgsLen)
    if fi.InArgsLen > 0 {
        values[0] = reflect.ValueOf(m.Args) // 只传入请求参数
    }
    
    // 3. 本地调用处理
    if !m.IsCluster {
        if m.ChanResult == nil {
            // ❌ 没有返回通道，直接调用，返回值丢失
            fi.Value.Call(values)
        } else {
            // ✅ 有返回通道，返回值通过 channel 传递
            rets := fi.Value.Call(values)
            rsp := retValue(app.Serializer(), rets)
            m.ChanResult <- rsp  // ← 返回值在这里
        }
    }
}
```

### 关键发现

#### 1. 本地 RPC 调用流程

```
调用方 (CallWait)
  ↓
创建 Message (包含 ChanResult channel)
  ↓
发送到目标 Actor 的消息队列
  ↓
目标 Actor 处理消息 (InvokeRemoteFunc)
  ↓
执行目标方法 getPlayerData()
  ↓
返回值通过 retValue() 序列化
  ↓
返回值发送到 ChanResult channel
  ↓
调用方从 channel 接收返回值
  ↓
反序列化到 userInfo
```

#### 2. 返回值处理（retValue 函数）

```go
func retValue(serializer cfacade.ISerializer, rets []reflect.Value) *cproto.Response {
    rsp := &cproto.Response{
        Code: ccode.OK,
    }

    retsLen := len(rets)
    switch retsLen {
    case 1:
        // 只有一个返回值（通常是 error code）
        if val := rets[0].Interface(); val != nil {
            if c, ok := val.(int32); ok {
                rsp.Code = c
            }
        }
    case 2:
        // 两个返回值：(data, code)
        if !rets[0].IsNil() {
            // ✅ 第一个返回值序列化到 rsp.Data
            data, err := serializer.Marshal(rets[0].Interface())
            if err != nil {
                rsp.Code = ccode.RPCRemoteExecuteError
            } else {
                rsp.Data = data  // ← 数据在这里
            }
        }

        if val := rets[1].Interface(); val != nil {
            if c, ok := val.(int32); ok {
                rsp.Code = c  // ← 错误码在这里
            }
        }
    }

    return rsp
}
```

## 🎓 协议定义的必要性

### 问题：本地 RPC 调用是否需要定义协议？

**答案：是的，必须定义！**

### 原因分析

#### 1. 序列化/反序列化

即使是本地调用，Cherry 框架也会进行序列化和反序列化：

```go
// 调用方：序列化请求参数
requestBytes := serializer.Marshal(request)

// 传输（即使是本地，也通过 channel）
message.Args = requestBytes

// 被调用方：反序列化请求参数
EncodeRemoteArgs(app, fi, m)  // ← 这里会反序列化

// 被调用方：序列化返回值
responseBytes := serializer.Marshal(response)

// 调用方：反序列化返回值
serializer.Unmarshal(responseBytes, userInfo)  // ← 这里会反序列化
```

#### 2. 为什么要序列化？

1. **统一接口**：本地调用和远程调用使用相同的机制
2. **类型安全**：通过 protobuf 保证类型一致性
3. **跨语言支持**：protobuf 可以跨语言
4. **版本兼容**：protobuf 支持向后兼容
5. **性能优化**：protobuf 比 JSON 更高效

#### 3. 数据流转过程

```
调用方内存对象 (Go struct)
  ↓ Marshal
字节数组 ([]byte)
  ↓ 通过 channel 传递
字节数组 ([]byte)
  ↓ Unmarshal
被调用方内存对象 (Go struct)
  ↓ 执行业务逻辑
返回值对象 (Go struct)
  ↓ Marshal
字节数组 ([]byte)
  ↓ 通过 channel 传递
字节数组 ([]byte)
  ↓ Unmarshal
调用方接收对象 (Go struct)
```

## 🐛 你的问题诊断

### 可能的原因

#### 1. 返回值类型不匹配

```go
// actor_player.go
func (p *actorPlayer) getPlayerData(msg *pb.Int32) (*tableModel.SlotsUser, int32) {
    return &p.playerData.SlotsUser, code.OK
}

// game.go
userInfo := &tableModel.SlotsUser{}  // ← 期望接收 *tableModel.SlotsUser
a.CallWait(targetPath, getPlayerData, &pb.Int32{...}, userInfo)
```

**问题**：`CallWait` 的第4个参数应该是指针，用于接收反序列化后的数据。

#### 2. CallWait 的正确用法

让我查看 CallWait 的实现：

```go
// CallWait 会：
// 1. 创建一个 channel 用于接收返回值
// 2. 发送消息到目标 Actor
// 3. 等待 channel 返回结果
// 4. 将结果反序列化到第4个参数

// 正确用法：
result := &pb.Response{}
actor.CallWait(targetPath, funcName, request, result)
// result 会被填充
```

#### 3. 序列化问题

如果 `tableModel.SlotsUser` 没有正确实现序列化接口，反序列化会失败：

```go
// 检查 SlotsUser 是否是 protobuf 生成的类型
type SlotsUser struct {
    // 如果是 GORM 生成的，可能不支持 protobuf 序列化
    UserID int64 `gorm:"column:user_id" json:"user_id"`
    // ...
}
```

## 💡 解决方案

### 方案 1：使用 Protobuf 定义返回类型（推荐）

#### 步骤 1：定义 protobuf 消息

```protobuf
// player.proto
message GetPlayerDataRequest {
    int32 userId = 1;
}

message GetPlayerDataResponse {
    int64 userId = 1;
    string name = 2;
    int32 level = 3;
    int64 exp = 4;
    int64 money = 5;
    int64 diamond = 6;
}
```

#### 步骤 2：修改 actor_player.go

```go
func (p *actorPlayer) getPlayerData(req *pb.GetPlayerDataRequest) (*pb.GetPlayerDataResponse, int32) {
    if p.playerData == nil || p.playerData.UserID == 0 {
        err := p.loadPlayerData(req.UserId)
        if err != nil {
            return nil, code.PlayerIDError
        }
    }
    
    // 转换为 protobuf 类型
    response := &pb.GetPlayerDataResponse{
        UserId:  p.playerData.UserID,
        Name:    p.playerData.Name,
        Level:   p.playerData.Level,
        Exp:     p.playerData.Exp,
        Money:   p.playerData.Money,
        Diamond: p.playerData.Diamond,
    }
    
    return response, code.OK
}
```

#### 步骤 3：修改 game.go

```go
func GetUserInfo(a cfacade.IActor, session *cproto.Session) *pb.GetPlayerDataResponse {
    targetPath := cfacade.NewChildPath("", playerActor, session.Uid)
    
    request := &pb.GetPlayerDataRequest{
        UserId: int32(session.Uid),
    }
    
    response := &pb.GetPlayerDataResponse{}
    
    result := a.CallWait(targetPath, getPlayerData, request, response)
    
    if result.Code != code.OK {
        clog.Errorf("GetUserInfo failed: code=%d", result.Code)
        return nil
    }
    
    return response
}
```

### 方案 2：使用 JSON 序列化（不推荐）

如果你坚持使用 GORM 生成的类型，需要确保使用 JSON 序列化器：

```go
// 配置文件中设置序列化器为 JSON
{
    "serializer": "json"
}
```

但这样会失去 protobuf 的优势。

### 方案 3：手动转换（临时方案）

```go
func GetUserInfo(a cfacade.IActor, session *cproto.Session) *tableModel.SlotsUser {
    targetPath := cfacade.NewChildPath("", playerActor, session.Uid)
    
    // 使用 protobuf 类型接收
    response := &pb.GetPlayerDataResponse{}
    
    result := a.CallWait(targetPath, getPlayerData, &pb.GetPlayerDataRequest{
        UserId: int32(session.Uid),
    }, response)
    
    if result.Code != code.OK {
        return nil
    }
    
    // 手动转换为 GORM 类型
    userInfo := &tableModel.SlotsUser{
        UserID:  response.UserId,
        Name:    response.Name,
        Level:   response.Level,
        Exp:     response.Exp,
        Money:   response.Money,
        Diamond: response.Diamond,
    }
    
    return userInfo
}
```

## 📊 调试技巧

### 1. 添加日志

```go
func (p *actorPlayer) getPlayerData(msg *pb.Int32) (*tableModel.SlotsUser, int32) {
    clog.Infof("getPlayerData called: userId=%d", msg.Value)
    
    if p.playerData == nil {
        clog.Warnf("playerData is nil, loading...")
        err := p.loadPlayerData(msg.Value)
        if err != nil {
            clog.Errorf("loadPlayerData failed: %v", err)
            return nil, code.PlayerIDError
        }
    }
    
    clog.Infof("returning playerData: %+v", p.playerData.SlotsUser)
    return &p.playerData.SlotsUser, code.OK
}
```

### 2. 检查序列化

```go
func GetUserInfo(a cfacade.IActor, session *cproto.Session) *tableModel.SlotsUser {
    userInfo := &tableModel.SlotsUser{}
    
    result := a.CallWait(targetPath, getPlayerData, request, userInfo)
    
    clog.Infof("CallWait result: code=%d, userInfo=%+v", result.Code, userInfo)
    
    return userInfo
}
```

### 3. 检查类型

```go
// 确认 SlotsUser 的类型
clog.Infof("SlotsUser type: %T", userInfo)

// 确认是否实现了序列化接口
if _, ok := interface{}(userInfo).(proto.Message); ok {
    clog.Info("SlotsUser implements proto.Message")
} else {
    clog.Warn("SlotsUser does NOT implement proto.Message")
}
```

## 🎯 总结

### 问题原因

1. **类型不匹配**：`tableModel.SlotsUser` 可能不是 protobuf 类型
2. **序列化失败**：GORM 生成的类型不支持 protobuf 序列化
3. **返回值处理**：Cherry 框架通过序列化/反序列化传递返回值

### 是否需要定义协议？

**是的，必须定义！**

即使是本地 RPC 调用，Cherry 框架也会：
- ✅ 序列化请求参数
- ✅ 反序列化请求参数
- ✅ 序列化返回值
- ✅ 反序列化返回值

### 最佳实践

1. **使用 protobuf 定义所有 RPC 接口**
2. **请求和响应都使用 protobuf 类型**
3. **在 Actor 内部使用 GORM 类型**
4. **在 RPC 边界进行类型转换**

### 推荐架构

```
┌─────────────────────────────────────────┐
│         RPC 调用方 (level_room.go)       │
│  使用 protobuf 类型                      │
│  pb.GetPlayerDataRequest                │
│  pb.GetPlayerDataResponse               │
└─────────────────┬───────────────────────┘
                  │ RPC (序列化)
                  ↓
┌─────────────────────────────────────────┐
│      RPC 被调用方 (actor_player.go)      │
│  1. 接收 protobuf 类型                   │
│  2. 转换为 GORM 类型（内部使用）         │
│  3. 执行业务逻辑                         │
│  4. 转换回 protobuf 类型                 │
│  5. 返回 protobuf 类型                   │
└─────────────────────────────────────────┘
```

这样既保证了 RPC 的类型安全，又能在内部使用 GORM 的便利性。
