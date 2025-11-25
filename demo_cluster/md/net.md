# 网络连接流程

客户端 WebSocket 请求

    ↓

http.Serve(listener, w)  ← WSConnector 实现了 http.Handler 接口

    ↓

ServeHTTP(rw, r)  ← HTTP 服务器自动调用

    ↓

upgrade.Upgrade()  ← 升级为 WebSocket 连接

    ↓

w.InChan(&conn)  ← 将连接放入 channel

    ↓

Connector.Start() 的 goroutine 监听 channel

    ↓

onConnectFunc(conn)  ← 调用注册的回调函数

    ↓

pomelo.Actor.defaultOnConnectFunc()  ← 创建 Agent

    ↓

应用层处理

# 完整的数据流：

WebSocket 连接建立
    ↓
pomelo.Actor.defaultOnConnectFunc()
    ↓
创建 Agent（包含 Session 信息）
    ↓
调用 SetOnNewAgent 注册的回调
    ↓
创建 ActorAgent 子 Actor
    ↓
agent.Run() 启动消息循环
    ↓
读取客户端消息 → 路由到对应的 Actor 处理

# 完整的数据流

客户端发送: {"route": "game.player.enter", "data": {...}}
    ↓
WebSocket 连接接收数据
    ↓
Agent.Run() 读取数据
    ↓
解析 Packet (packet.go)
    ↓
dataCommand() 处理数据包 (command.go:179)
    ↓
cmd.onDataRouteFunc(agent, route, msg)  ← 这里！
    ↓
onPomeloDataRoute() 你的路由逻辑 (gate/route.go)
    ↓
检查登录状态、选择目标节点
    ↓
ClusterLocalDataRoute() 跨节点转发
    ↓
目标节点的 Actor 处理消息
    ↓
game.player.enter() 方法被调用

# 玩家连接时

// 1. 客户端连接到 Gate
WebSocket 连接建立
    ↓
// 2. agentActor 创建 Agent（网络层封装）
agent := NewAgent(conn, session)  // sid = "abc123"
    ↓
// 3. 回调：创建 ActorAgent（业务层）
agentActor.SetOnNewAgent(func(newAgent *Agent) {
    childActor := &ActorAgent{}
    agentActor.Child().Create(newAgent.SID(), childActor)
    // 路径：gate-01.user.abc123
})
    ↓
// 4. Agent 开始接收消息
agent.Run()

# 玩家发送登录请求：

// 客户端发送：{"route": "gate.user.login", "data": {...}}

// 1. Agent 接收数据
agent.Run() → 读取数据包
    ↓
// 2. agentActor 解析协议
dataCommand() → 解析 Pomelo 消息
    ↓
// 3. 路由到 ActorAgent
onPomeloDataRoute() 
    → route = "gate.user.login"
    → targetPath = "gate-01.user.abc123"
    → funcName = "login"
    ↓
// 4. ActorAgent 处理登录
ActorAgent.login(session, req)
    → 验证 token
    → 绑定 UID
    → 返回响应

# 玩家断开连接：

// 1. 连接断开
agent.Close()
    ↓
// 2. 触发关闭回调
newAgent.AddOnClose(childActor.onSessionClose)
    ↓
// 3. ActorAgent 清理资源
ActorAgent.onSessionClose()
    → 通知 Game 节点
    → p.Exit()  // 销毁自己
    ↓
// 4. Agent 被清理
pomelo.UnbindSID(sid)
