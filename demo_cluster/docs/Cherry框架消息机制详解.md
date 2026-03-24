# Cherry 框架消息回复和推送机制详解

## 概述

Cherry 框架使用 Actor 模型实现分布式游戏服务器架构。消息的回复和推送通过 Actor 之间的通信完成，支持本地调用和跨节点 RPC 调用。

## 架构层次

```
客户端 (Client)
    ↕
Gate 节点 (Agent)
    ↕ RPC
Game 节点 (Actor)
```

## 核心组件

### 1. Agent (vendor/github.com/cherry-game/cherry/net/parser/pomelo/agent.go)

Agent 代表一个客户端连接，运行在 Gate 节点上。

**核心方法**:
- `Response()` - 回复客户端请求
- `Push()` - 主动推送消息给客户端
- `Kick()` - 踢出客户端

### 2. ActorBase (vendor/github.com/cherry-game/cherry/net/parser/pomelo/actor_base.go)

ActorBase 提供了 Game 节点 Actor 与 Gate 节点 Agent 通信的封装方法。

**核心方法**:
- `Response()` - 通过 RPC 调用 Gate 的 Agent 回复消息
- `Push()` - 通过 RPC 调用 Gate 的 Agent 推送消息
- `Kick()` - 通过 RPC 调用 Gate 的 Agent 踢出客户端

### 3. Actor (vendor/github.com/cherry-game/cherry/net/parser/pomelo/actor.go)

Actor 是 Gate 节点上的消息路由器，负责接收来自 Game 节点的 RPC 调用。

**注册的 RPC 方法**:
- `response` - 处理回复请求
- `push` - 处理推送请求
- `kick` - 处理踢出请求
- `broadcast` - 处理广播请求

## 消息流程详解

### 场景1：Gate 节点直接回复（登录场景）

```
客户端 → Gate(Agent) → Gate(ActorAgent.login) → Agent.Response() → 客户端
```

**代码示例** (`demo_cluster/nodes/gate/actor_agent.go`):
```go
func (p *ActorAgent) login(session *cproto.Session, req *pb.LoginRequest) {
    // ... 业务逻辑 ...
    
    response := &pb.LoginResponse{
        UserId: userId,
        Pid:    userToken.PID,
        OpenId: userToken.OpenID,
    }
    
    // 直接调用 Agent.Response()
    agent.Response(session, response)
}
```

**流程**:
1. 客户端发送登录请求到 Gate
2. Gate 的 route.go 路由到 ActorAgent.login
3. ActorAgent 处理业务逻辑
4. 调用 `agent.Response()` 直接回复客户端
5. Agent 将消息编码后发送给客户端

### 场景2：Game 节点回复（跨节点 RPC）

```
客户端 → Gate(Agent) → Game(ActorPlayer.playerSelect) 
    → ActorBase.Response() → RPC → Gate(Actor.response) 
    → Agent.Response() → 客户端
```

**代码示例** (`demo_cluster/nodes/game/module/player/actor_player.go`):
```go
func (p *actorPlayer) playerSelect(session *cproto.Session, _ *pb.None) {
    response := &pb.PlayerSelectResponse{}
    userId := session.Uid
    
    if userId > 0 {
        playerTable, found := db.GetPlayerTable(userId)
        if found {
            playerInfo := buildPBPlayer(playerTable)
            response.List = append(response.List, &playerInfo)
        }
    }
    
    // 调用 ActorBase.Response()，会通过 RPC 调用 Gate 的 Agent
    p.Response(session, response)
}
```

**详细流程**:

#### 步骤1：Game 节点调用 ActorBase.Response()

**文件**: `vendor/github.com/cherry-game/cherry/net/parser/pomelo/actor_base.go`

```go
func (p *ActorBase) Response(session *cproto.Session, v any) {
    Response(p, session.AgentPath, session.Sid, session.GetMID(), v)
}

func Response(iActor cfacade.IActor, agentPath, sid string, mid uint32, v any) {
    // 1. 序列化响应数据
    data, err := iActor.App().Serializer().Marshal(v)
    if err != nil {
        clog.Warnf("[Response] Marshal error. v = %+v", v)
        return
    }

    // 2. 构造 RPC 请求
    rsp := &cproto.PomeloResponse{
        Sid:  sid,      // Session ID
        Mid:  mid,      // Message ID
        Data: data,     // 序列化后的响应数据
    }

    // 3. 通过 RPC 调用 Gate 节点的 Actor.response 方法
    iActor.Call(agentPath, ResponseFuncName, rsp)
}
```

**关键参数**:
- `agentPath`: Gate 节点的 Actor 路径（例如：`gate-1.user`）
- `sid`: Session ID，用于在 Gate 节点找到对应的 Agent
- `mid`: Message ID，客户端请求的消息 ID
- `data`: 序列化后的响应数据

#### 步骤2：Gate 节点接收 RPC 调用

**文件**: `vendor/github.com/cherry-game/cherry/net/parser/pomelo/actor.go`

```go
func (p *Actor) OnInit() {
    // 注册 RPC 方法
    p.Remote().Register(ResponseFuncName, p.response)
    p.Remote().Register(PushFuncName, p.push)
    p.Remote().Register(KickFuncName, p.kick)
    p.Remote().Register(BroadcastName, p.broadcast)
}

func (p *Actor) response(rsp *cproto.PomeloResponse) {
    // 1. 根据 SID 查找对应的 Agent
    agent, found := GetAgentWithSID(rsp.Sid)
    if !found {
        clog.Debugf("[response] Not found agent. [rsp = %+v]", rsp)
        return
    }

    // 2. 调用 Agent.ResponseMID() 发送响应给客户端
    if ccode.IsOK(rsp.Code) {
        agent.ResponseMID(rsp.Mid, rsp.Data, false)
    } else {
        errRsp := &cproto.Response{
            Code: rsp.Code,
        }
        agent.ResponseMID(rsp.Mid, errRsp, true)
    }
}
```

#### 步骤3：Agent 发送消息给客户端

**文件**: `vendor/github.com/cherry-game/cherry/net/parser/pomelo/agent.go`

```go
func (a *Agent) Response(session *cproto.Session, v interface{}, isError ...bool) {
    // [CUSTOM] 统一打印响应消息（Info 级别）
    clog.Infof("[GATE-OUT] uid=%d, sid=%s, mid=%d",
        session.Uid, session.Sid, session.GetMID())
    
    // [CUSTOM] 详细日志（Debug 级别）
    if clog.PrintLevel(zapcore.DebugLevel) {
        if payload, err := a.Serializer().Marshal(v); err == nil {
            clog.Debugf("[GATE-OUT-DETAIL] uid=%d, sid=%s, mid=%d, resp=%s",
                session.Uid, session.Sid, session.GetMID(), string(payload))
        }
    }
    
    a.ResponseMID(session.GetMID(), v, isError...)
}

func (a *Agent) ResponseMID(mid uint32, v interface{}, isError ...bool) {
    isErr := false
    if len(isError) > 0 {
        isErr = isError[0]
    }

    // 发送到 pending 队列
    a.sendPending(pomeloMessage.Response, "", mid, v, isErr)
}

func (a *Agent) sendPending(typ pomeloMessage.Type, route string, mid uint32, v interface{}, isError bool) {
    // 构造 pending 消息
    pending := &pendingMessage{
        typ:     typ,
        mid:     uint(mid),
        route:   route,
        payload: v,
        err:     isError,
    }

    // 发送到 pending 队列
    a.chPending <- pending
}

func (a *Agent) processPending(data *pendingMessage) {
    // 1. 序列化 payload
    payload, err := a.Serializer().Marshal(data.payload)
    if err != nil {
        clog.Warnf("[sid = %s,uid = %d] Payload marshal error.", a.SID(), a.UID())
        return
    }

    // 2. 构造 Pomelo Message
    m := &pomeloMessage.Message{
        Type:  data.typ,
        ID:    data.mid,
        Route: data.route,
        Data:  payload,
        Error: data.err,
    }

    // 3. 编码 Message
    em, err := pomeloMessage.Encode(m)
    if err != nil {
        clog.Warn(err)
        return
    }

    // 4. 编码 Packet 并发送
    a.SendPacket(pomeloPacket.Data, em)
}

func (a *Agent) SendPacket(typ pomeloPacket.Type, data []byte) {
    pkg, err := pomeloPacket.Encode(typ, data)
    if err != nil {
        clog.Warn(err)
        return
    }
    a.SendRaw(pkg)
}

func (a *Agent) SendRaw(bytes []byte) {
    a.chWrite <- bytes
}

func (a *Agent) write(bytes []byte) {
    // 最终通过 TCP 连接发送给客户端
    _, err := a.conn.Write(bytes)
    if err != nil {
        clog.Warn(err)
    }
}
```

### 场景3：Game 节点主动推送

```
Game(ActorPlayer) → ActorBase.Push() → RPC → Gate(Actor.push) 
    → Agent.Push() → 客户端
```

**代码示例**:
```go
// Game 节点主动推送消息
func (p *actorPlayer) notifyLevelUp(session *cproto.Session) {
    pushData := &pb.LevelUpNotify{
        NewLevel: p.playerData.Level,
        Rewards:  []string{"gold:1000", "exp:500"},
    }
    
    // 调用 ActorBase.Push()
    p.Push(session, "game.player.levelUp", pushData)
}
```

**Push 流程** (`actor_base.go`):
```go
func (p *ActorBase) Push(session *cproto.Session, route string, v any) {
    PushWithSID(p, session.AgentPath, session.Sid, route, v)
}

func PushWithSID(iActor cfacade.IActor, agentPath, sid, route string, v any) {
    Push(iActor, agentPath, sid, 0, route, v)
}

func Push(iActor cfacade.IActor, agentPath, sid string, uid cfacade.UID, route string, v any) {
    // 1. 序列化数据
    data, err := iActor.App().Serializer().Marshal(v)
    if err != nil {
        clog.Warnf("[Push] Marshal error. route =%s, v = %+v", route, v)
        return
    }

    // 2. 构造 RPC 请求
    rsp := &cproto.PomeloPush{
        Sid:   sid,
        Uid:   uid,
        Route: route,
        Data:  data,
    }

    // 3. 通过 RPC 调用 Gate 节点的 Actor.push 方法
    iActor.Call(agentPath, PushFuncName, rsp)
}
```

**Gate 节点处理 Push** (`actor.go`):
```go
func (p *Actor) push(rsp *cproto.PomeloPush) {
    // 根据 SID 或 UID 查找 Agent
    if rsp.Sid != "" || rsp.Uid > 0 {
        if agent, found := GetAgent(rsp.Sid, rsp.Uid); found {
            agent.Push(rsp.Route, rsp.Data)
        }
        return
    }
}
```

**Agent 推送消息** (`agent.go`):
```go
func (a *Agent) Push(route string, val interface{}) {
    // [CUSTOM] 统一打印推送消息（Info 级别）
    clog.Infof("[GATE-PUSH] uid=%d, sid=%s, route=%s",
        a.UID(), a.SID(), route)
    
    // [CUSTOM] 详细日志（Debug 级别）
    if clog.PrintLevel(zapcore.DebugLevel) {
        if payload, err := a.Serializer().Marshal(val); err == nil {
            clog.Debugf("[GATE-PUSH-DETAIL] uid=%d, sid=%s, route=%s, data=%s",
                a.UID(), a.SID(), route, string(payload))
        }
    }
    
    a.sendPending(pomeloMessage.Push, route, 0, val, false)
}
```

## Session 的作用

Session 是连接客户端和服务器的关键数据结构：

```go
type Session struct {
    Sid       string            // Session ID（唯一标识）
    Uid       int64             // User ID（绑定后的用户 ID）
    AgentPath string            // Agent 的 Actor 路径（例如：gate-1.user）
    Data      map[string]string // Session 数据
    Ip        string            // 客户端 IP
}
```

**关键字段**:
- `Sid`: 用于在 Gate 节点查找对应的 Agent
- `Uid`: 用户 ID，绑定后可以通过 UID 查找 Agent
- `AgentPath`: Gate 节点的 Actor 路径，用于 RPC 调用
- `Data`: 存储 Session 相关数据（如 PlayerID、ServerID 等）

**Session 的传递**:
1. 客户端请求到达 Gate → 创建 Session
2. Gate 路由到 Game → Session 通过 RPC 传递
3. Game 处理完成 → 使用 Session 中的 AgentPath 和 Sid 回复消息

## 消息类型

### 1. Response（回复）
- **特点**: 有 Message ID，客户端等待响应
- **使用场景**: 客户端请求-响应模式
- **示例**: 登录、查询角色、进入游戏

### 2. Push（推送）
- **特点**: 无 Message ID，服务器主动推送
- **使用场景**: 服务器主动通知客户端
- **示例**: 等级提升、新邮件、好友上线

### 3. Kick（踢出）
- **特点**: 强制断开连接
- **使用场景**: 重复登录、封号、维护
- **示例**: 挤号、账号异常

## 广播机制

Cherry 框架支持广播消息给多个客户端：

```go
// 广播给所有在线用户
func (p *actorPlayer) broadcastToAll() {
    data := &pb.SystemNotify{
        Message: "服务器将在 10 分钟后维护",
    }
    
    p.PushWithUIDS("gate-1.user", nil, true, "system.notify", data)
}

// 广播给指定用户列表
func (p *actorPlayer) broadcastToUsers(uidList []int64) {
    data := &pb.GuildNotify{
        Message: "公会战即将开始",
    }
    
    p.PushWithUIDS("gate-1.user", uidList, false, "guild.notify", data)
}
```

**广播流程** (`actor_base.go`):
```go
func PushWithUIDS(iActor cfacade.IActor, agentPath string, uidList []int64, allUID bool, route string, v any) {
    // 1. 序列化数据
    data, err := iActor.App().Serializer().Marshal(v)
    if err != nil {
        return
    }

    // 2. 构造广播请求
    rsp := &cproto.PomeloBroadcast{
        Route: route,
        Data:  data,
    }

    if allUID {
        rsp.PushType = cproto.PomeloBroadcast_AllUID
    } else {
        rsp.PushType = cproto.PomeloBroadcast_UID
        rsp.UidList = uidList
    }

    // 3. 通过 RPC 调用 Gate 节点的 Actor.broadcast 方法
    iActor.Call(agentPath, BroadcastName, rsp)
}
```

**Gate 节点处理广播** (`actor.go`):
```go
func (p *Actor) broadcast(rsp *cproto.PomeloBroadcast) {
    switch rsp.PushType {
    case cproto.PomeloBroadcast_AllUID:
        // 遍历所有已绑定的 Agent
        ForeachAgent(func(agent *Agent) {
            if agent.IsBind() {
                agent.Push(rsp.Route, rsp.Data)
            }
        })
        
    case cproto.PomeloBroadcast_UID:
        // 遍历指定的 UID 列表
        for _, uid := range rsp.UidList {
            if agent, found := GetAgentWithUID(uid); found {
                agent.Push(rsp.Route, rsp.Data)
            }
        }
    }
}
```

## 完整的消息流程图

### Response 流程
```
客户端
  ↓ 1. 发送请求（route + data + mid）
Gate(Agent)
  ↓ 2. 路由消息
Gate(ActorAgent) 或 Game(ActorPlayer)
  ↓ 3. 处理业务逻辑
  ↓ 4. 调用 Response(session, data)
ActorBase.Response()
  ↓ 5. 序列化数据
  ↓ 6. 构造 PomeloResponse{sid, mid, data}
  ↓ 7. RPC 调用 Gate 的 Actor.response
Gate(Actor.response)
  ↓ 8. 根据 sid 查找 Agent
  ↓ 9. 调用 Agent.ResponseMID(mid, data)
Gate(Agent)
  ↓ 10. 发送到 pending 队列
  ↓ 11. 编码 Message + Packet
  ↓ 12. 通过 TCP 发送
客户端
```

### Push 流程
```
Game(ActorPlayer)
  ↓ 1. 调用 Push(session, route, data)
ActorBase.Push()
  ↓ 2. 序列化数据
  ↓ 3. 构造 PomeloPush{sid, route, data}
  ↓ 4. RPC 调用 Gate 的 Actor.push
Gate(Actor.push)
  ↓ 5. 根据 sid/uid 查找 Agent
  ↓ 6. 调用 Agent.Push(route, data)
Gate(Agent)
  ↓ 7. 发送到 pending 队列
  ↓ 8. 编码 Message + Packet
  ↓ 9. 通过 TCP 发送
客户端
```

## 关键数据结构

### PomeloResponse
```go
type PomeloResponse struct {
    Sid  string  // Session ID
    Mid  uint32  // Message ID
    Code int32   // 错误码（0 表示成功）
    Data []byte  // 响应数据（序列化后）
}
```

### PomeloPush
```go
type PomeloPush struct {
    Sid   string  // Session ID
    Uid   int64   // User ID
    Route string  // 推送路由
    Data  []byte  // 推送数据（序列化后）
}
```

### PomeloBroadcast
```go
type PomeloBroadcast struct {
    PushType int32    // 广播类型（AllUID 或 UID）
    Route    string   // 推送路由
    Data     []byte   // 推送数据（序列化后）
    UidList  []int64  // 用户 ID 列表
}
```

## 性能优化

### 1. 消息队列
Agent 使用 channel 实现消息队列，避免阻塞：
```go
chPending chan *pendingMessage  // pending 消息队列
chWrite   chan []byte            // 写入队列
```

### 2. 序列化缓存
避免重复序列化相同的数据：
```go
// 在 ActorBase.Response 中序列化一次
data, err := iActor.App().Serializer().Marshal(v)

// 通过 RPC 传递序列化后的数据
rsp := &cproto.PomeloResponse{
    Data: data,  // 已序列化
}
```

### 3. 日志级别控制
使用 `PrintLevel` 避免不必要的序列化：
```go
if clog.PrintLevel(zapcore.DebugLevel) {
    if payload, err := a.Serializer().Marshal(v); err == nil {
        clog.Debugf("[GATE-OUT-DETAIL] resp=%s", string(payload))
    }
}
```

## 总结

Cherry 框架的消息机制核心要点：

1. **Actor 模型**: 使用 Actor 模型实现分布式通信
2. **RPC 调用**: Game 节点通过 RPC 调用 Gate 节点的 Agent
3. **Session 传递**: Session 包含 AgentPath 和 Sid，用于定位 Agent
4. **消息队列**: Agent 使用 channel 实现异步消息处理
5. **统一日志**: 在 Agent 底层统一打印消息日志
6. **性能优化**: 序列化缓存、日志级别控制、消息队列

这种设计实现了：
- ✅ Gate 和 Game 节点的解耦
- ✅ 支持水平扩展
- ✅ 统一的消息处理流程
- ✅ 高性能的消息传递
