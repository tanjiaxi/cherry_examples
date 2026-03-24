# Pomelo 协议 Message ID (MID) 匹配机制详解

## 问题

客户端发送请求：
```javascript
pomelo.request("game.player.select", noneData, function (data) {
    // 回调函数
});
```

服务器回复消息时没有 route，那么客户端如何知道这个响应对应哪个请求？

## 答案：Message ID (MID)

Pomelo 协议使用 **Message ID (MID)** 来匹配请求和响应，这是一个自增的唯一标识符。

## 完整流程详解

### 1. 客户端发送请求

**客户端代码** (`pomelo-client-protobuf.js`):

```javascript
var reqId = 0;  // 全局的请求 ID 计数器
var callbacks = {};  // 存储回调函数的字典
var routeMap = {};   // 存储 reqId 到 route 的映射

pomelo.request = function(route, msg, cb) {
    // 1. reqId 自增
    reqId++;
    
    // 2. 发送消息（包含 reqId 和 route）
    sendMessage(reqId, route, msg);
    
    // 3. 保存回调函数（使用 reqId 作为 key）
    callbacks[reqId] = cb;
    
    // 4. 保存 route 映射（用于后续解码）
    routeMap[reqId] = route;
};
```

**示例**:
```javascript
// 第一次请求
pomelo.request("game.player.select", {}, callback1);
// reqId = 1, callbacks[1] = callback1, routeMap[1] = "game.player.select"

// 第二次请求
pomelo.request("game.player.enter", {playerId: 123}, callback2);
// reqId = 2, callbacks[2] = callback2, routeMap[2] = "game.player.enter"
```

### 2. 消息编码

**编码函数** (`pomelo-client-protobuf.js`):

```javascript
var defaultEncode = pomelo.encode = function(reqId, route, msg) {
    // 1. 确定消息类型
    var type = reqId ? Message.TYPE_REQUEST : Message.TYPE_NOTIFY;
    
    // 2. 序列化消息体
    if(decodeIO_encoder && decodeIO_encoder.lookup(route)) {
        var Builder = decodeIO_encoder.build(route);
        msg = new Builder(msg).encodeNB();
    }
    
    // 3. 路由压缩（可选）
    var compressRoute = 0;
    if(dict && dict[route]) {
        route = dict[route];
        compressRoute = 1;
    }
    
    // 4. 编码 Message（包含 reqId、type、route、msg）
    return Message.encode(reqId, type, compressRoute, route, msg);
};
```

**Message 结构** (`message.go`):

```go
type Message struct {
    Type   Type    // 消息类型（Request/Response/Push/Notify）
    ID     uint    // Message ID（reqId）
    Route  string  // 路由（Request 和 Notify 有，Response 没有）
    Data   []byte  // 消息体
    Error  bool    // 是否错误
}
```

**编码后的二进制格式**:

```
Request 消息:
+------+------------+-------+------+
| flag | message id | route | data |
+------+------------+-------+------+
  1B      0-5B       0-256B   ...

flag = 000- (Request type)
message id = reqId (变长编码)
route = "game.player.select"
data = 序列化后的请求数据
```

### 3. 服务器接收请求

**Gate 节点接收** (`route.go`):

```go
func onPomeloDataRoute(agent *pomelo.Agent, packet *ppacket.Packet) {
    // 1. 解码 Message
    msg, err := pmessage.Decode(packet.Data())
    
    // 2. 打印接收日志
    clog.Infof("[GATE-IN] route=%s, uid=%d, sid=%s, mid=%d, size=%d bytes",
        msg.Route, session.Uid, session.Sid, msg.ID, len(msg.Data))
    
    // 3. 构建 Session（包含 MID）
    session := pomelo.BuildSession(agent, msg)
    session.SetMID(uint32(msg.ID))  // 保存 Message ID
    
    // 4. 路由到对应的 Handler
    route := pmessage.ParseRoute(msg.Route)
    pomelo.DefaultDataRoute(agent, route, msg)
}
```

**Session 结构**:

```go
type Session struct {
    Sid       string  // Session ID
    Uid       int64   // User ID
    AgentPath string  // Agent 路径
    Data      map[string]string
    mid       uint32  // Message ID（关键！）
}

func (s *Session) GetMID() uint32 {
    return s.mid
}
```

### 4. Game 节点处理请求

**Game Actor** (`actor_player.go`):

```go
func (p *actorPlayer) playerSelect(session *cproto.Session, _ *pb.None) {
    response := &pb.PlayerSelectResponse{}
    
    // ... 业务逻辑 ...
    
    // 调用 Response（session 中包含 MID）
    p.Response(session, response)
}
```

### 5. 服务器回复响应

**ActorBase.Response** (`actor_base.go`):

```go
func (p *ActorBase) Response(session *cproto.Session, v any) {
    Response(p, session.AgentPath, session.Sid, session.GetMID(), v)
}

func Response(iActor cfacade.IActor, agentPath, sid string, mid uint32, v any) {
    // 1. 序列化响应数据
    data, err := iActor.App().Serializer().Marshal(v)
    
    // 2. 构造 RPC 请求（包含 MID）
    rsp := &cproto.PomeloResponse{
        Sid:  sid,
        Mid:  mid,    // 关键：使用请求时的 MID
        Data: data,
    }
    
    // 3. RPC 调用 Gate 的 Actor.response
    iActor.Call(agentPath, ResponseFuncName, rsp)
}
```

**Gate Actor 处理响应** (`actor.go`):

```go
func (p *Actor) response(rsp *cproto.PomeloResponse) {
    // 1. 根据 SID 查找 Agent
    agent, found := GetAgentWithSID(rsp.Sid)
    if !found {
        return
    }
    
    // 2. 调用 Agent.ResponseMID（传递 MID）
    if ccode.IsOK(rsp.Code) {
        agent.ResponseMID(rsp.Mid, rsp.Data, false)
    } else {
        errRsp := &cproto.Response{Code: rsp.Code}
        agent.ResponseMID(rsp.Mid, errRsp, true)
    }
}
```

**Agent 发送响应** (`agent.go`):

```go
func (a *Agent) ResponseMID(mid uint32, v interface{}, isError ...bool) {
    isErr := false
    if len(isError) > 0 {
        isErr = isError[0]
    }
    
    // 发送到 pending 队列（包含 MID）
    a.sendPending(pomeloMessage.Response, "", mid, v, isErr)
}

func (a *Agent) sendPending(typ pomeloMessage.Type, route string, mid uint32, v interface{}, isError bool) {
    pending := &pendingMessage{
        typ:     typ,        // Response 类型
        mid:     uint(mid),  // Message ID
        route:   route,      // Response 没有 route（为空）
        payload: v,
        err:     isError,
    }
    
    a.chPending <- pending
}

func (a *Agent) processPending(data *pendingMessage) {
    // 1. 序列化 payload
    payload, _ := a.Serializer().Marshal(data.payload)
    
    // 2. 构造 Message（包含 MID，但没有 route）
    m := &pomeloMessage.Message{
        Type:  data.typ,     // Response
        ID:    data.mid,     // Message ID（关键！）
        Route: data.route,   // 空字符串
        Data:  payload,
        Error: data.err,
    }
    
    // 3. 编码并发送
    em, _ := pomeloMessage.Encode(m)
    a.SendPacket(pomeloPacket.Data, em)
}
```

**编码后的二进制格式**:

```
Response 消息:
+------+------------+------+
| flag | message id | data |
+------+------------+------+
  1B      0-5B        ...

flag = 010- (Response type)
message id = mid (与请求时相同)
route = 无（Response 类型不包含 route）
data = 序列化后的响应数据
```

### 6. 客户端接收响应

**解码函数** (`pomelo-client-protobuf.js`):

```javascript
var defaultDecode = pomelo.decode = function(data) {
    // 1. 解码 Message
    var msg = Message.decode(data);
    
    // 2. 如果有 id（Response 类型），从 routeMap 恢复 route
    if(msg.id > 0){
        msg.route = routeMap[msg.id];  // 根据 MID 查找 route
        delete routeMap[msg.id];       // 删除映射
        if(!msg.route){
            return;
        }
    }
    
    // 3. 反序列化消息体
    msg.body = deCompose(msg);
    return msg;
};
```

**处理消息** (`pomelo-client-protobuf.js`):

```javascript
var processMessage = function(pomelo, msg) {
    // 1. 如果没有 id，说明是服务器 Push 消息
    if(!msg.id) {
        pomelo.emit(msg.route, msg.body);
        return;
    }
    
    // 2. 如果有 id，说明是 Response 消息
    // 根据 id 查找回调函数
    var cb = callbacks[msg.id];
    
    // 3. 删除回调函数（一次性使用）
    delete callbacks[msg.id];
    
    // 4. 调用回调函数
    if(typeof cb !== 'function') {
        return;
    }
    
    cb(msg);  // 执行用户的回调函数
};
```

## 完整的 MID 匹配流程图

```
客户端                                    服务器
  |                                         |
  | 1. pomelo.request("game.player.select", {}, callback)
  |    reqId = 1                            |
  |    callbacks[1] = callback              |
  |    routeMap[1] = "game.player.select"  |
  |                                         |
  | 2. 编码 Message                         |
  |    Type: Request                        |
  |    ID: 1                                |
  |    Route: "game.player.select"         |
  |    Data: {}                             |
  |                                         |
  | 3. 发送 ----------------------->        |
  |                                         | 4. 解码 Message
  |                                         |    msg.ID = 1
  |                                         |    msg.Route = "game.player.select"
  |                                         |
  |                                         | 5. 构建 Session
  |                                         |    session.SetMID(1)
  |                                         |
  |                                         | 6. 路由到 Handler
  |                                         |    playerSelect(session, req)
  |                                         |
  |                                         | 7. 处理业务逻辑
  |                                         |    response = {...}
  |                                         |
  |                                         | 8. 回复响应
  |                                         |    Response(session, response)
  |                                         |    使用 session.GetMID() = 1
  |                                         |
  |                                         | 9. 编码 Message
  |                                         |    Type: Response
  |                                         |    ID: 1 (与请求相同)
  |                                         |    Route: "" (空)
  |                                         |    Data: response
  |                                         |
  | 10. 接收 <-----------------------       |
  |                                         |
  | 11. 解码 Message                        |
  |     msg.id = 1                          |
  |     msg.route = routeMap[1] = "game.player.select"
  |                                         |
  | 12. 查找回调函数                        |
  |     cb = callbacks[1]                   |
  |                                         |
  | 13. 执行回调                            |
  |     cb(msg)                             |
  |     用户的 callback 函数被调用          |
  |                                         |
```

## 关键数据结构对比

### 客户端

```javascript
// 全局变量
var reqId = 0;                    // 请求 ID 计数器
var callbacks = {};               // {reqId: callback}
var routeMap = {};                // {reqId: route}

// 发送请求
reqId++;                          // reqId = 1
callbacks[1] = callback;          // 保存回调
routeMap[1] = "game.player.select"; // 保存路由

// 接收响应
msg.id = 1;                       // 从响应中获取 id
msg.route = routeMap[1];          // 恢复路由
var cb = callbacks[1];            // 查找回调
cb(msg);                          // 执行回调
```

### 服务器

```go
// 接收请求
msg.ID = 1                        // 从请求中获取 ID
msg.Route = "game.player.select"  // 从请求中获取 Route

// 构建 Session
session.SetMID(uint32(msg.ID))    // 保存 MID = 1

// 回复响应
mid := session.GetMID()           // 获取 MID = 1
Response(agentPath, sid, mid, response)  // 使用相同的 MID
```

## 消息类型对比

| 消息类型 | Type | ID (MID) | Route | 说明 |
|---------|------|----------|-------|------|
| Request | 0 | ✅ 有 | ✅ 有 | 客户端请求，需要响应 |
| Notify | 1 | ❌ 无 | ✅ 有 | 客户端通知，不需要响应 |
| Response | 2 | ✅ 有 | ❌ 无 | 服务器响应，通过 MID 匹配请求 |
| Push | 3 | ❌ 无 | ✅ 有 | 服务器推送，主动发送 |

## 为什么 Response 不需要 Route？

1. **节省带宽**: Response 消息不包含 route，减少数据传输量
2. **客户端恢复**: 客户端通过 `routeMap[msg.id]` 恢复 route
3. **一一对应**: 每个 Request 对应一个 Response，通过 MID 唯一匹配

## Push 消息的区别

Push 消息是服务器主动推送，没有对应的请求：

```javascript
// 客户端监听 Push
pomelo.on("game.player.levelUp", function(data) {
    console.log("Level up!", data);
});

// 服务器发送 Push
p.Push(session, "game.player.levelUp", data)

// Push 消息结构
Type: Push (3)
ID: 0 (无 ID)
Route: "game.player.levelUp" (有 route)
Data: data
```

**处理逻辑**:

```javascript
var processMessage = function(pomelo, msg) {
    if(!msg.id) {
        // 没有 id，说明是 Push 消息
        // 直接通过 route 触发事件
        pomelo.emit(msg.route, msg.body);
        return;
    }
    
    // 有 id，说明是 Response 消息
    // 通过 id 查找回调函数
    var cb = callbacks[msg.id];
    cb(msg);
};
```

## 并发请求的处理

客户端可以同时发送多个请求，每个请求有不同的 MID：

```javascript
// 同时发送 3 个请求
pomelo.request("game.player.select", {}, callback1);   // reqId = 1
pomelo.request("game.player.enter", {}, callback2);    // reqId = 2
pomelo.request("game.slots.spin", {}, callback3);      // reqId = 3

// callbacks 状态
callbacks = {
    1: callback1,
    2: callback2,
    3: callback3
}

// 响应可能乱序到达
// 响应 3 先到达 -> 执行 callback3
// 响应 1 后到达 -> 执行 callback1
// 响应 2 最后到达 -> 执行 callback2
```

**服务器处理**:
- 每个请求独立处理
- 响应可能乱序返回
- 客户端通过 MID 正确匹配

## 总结

### MID 的作用

1. **请求-响应匹配**: 通过 MID 将响应与请求关联
2. **节省带宽**: Response 不需要包含 route
3. **支持并发**: 多个请求可以同时进行
4. **回调管理**: 客户端通过 MID 管理回调函数

### 关键点

- ✅ 客户端发送请求时生成 MID
- ✅ 服务器接收请求时保存 MID 到 Session
- ✅ 服务器回复响应时使用相同的 MID
- ✅ 客户端接收响应时通过 MID 查找回调函数
- ✅ Response 消息不包含 route，通过 MID 恢复
- ✅ Push 消息没有 MID，直接通过 route 触发事件

这就是 Pomelo 协议中 Message ID 的完整匹配机制！
