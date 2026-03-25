# NATS RPC 请求-响应机制详解

## initReplySubscribe 函数的作用

`initReplySubscribe` 函数是 Cherry 框架中 NATS RPC 调用的**响应接收器**，用于接收跨节点 RPC 调用的响应消息。

## 问题背景

在分布式系统中，当一个节点（如 Gate）需要调用另一个节点（如 Game）的方法时：

```go
// Gate 节点调用 Game 节点的方法
data, code := cluster.RequestRemote(gameNodeID, packet, timeout)
```

这个调用需要：
1. 发送请求到 Game 节点
2. 等待 Game 节点处理
3. 接收 Game 节点的响应
4. 返回结果给调用者

**问题**: 如何在异步的 NATS 消息系统中实现同步的 RPC 调用？

**答案**: 使用 `initReplySubscribe` 创建一个专用的响应接收通道。

## 核心机制

### 1. 每个连接有唯一的 Reply Subject

```go
type Connect struct {
    *nats.Conn
    id     int     // 连接 ID
    reply  string  // 响应主题（唯一）
    waiters sync.Map  // 等待响应的请求 map[reqID]chan
}

func NewConnect(id int, replySubject string, opts ...OptionFunc) *Connect {
    conn := &Connect{
        id:    id,
        reply: fmt.Sprintf("%s.%d", replySubject, id),  // 例如: "reply.gate-1.0"
    }
    return conn
}
```

**关键点**:
- 每个 NATS 连接有一个唯一的 `reply` subject
- 格式: `reply.{nodeType}-{nodeID}.{connectionID}`
- 例如: `reply.gate-1.0`, `reply.game-1.0`

### 2. initReplySubscribe 订阅响应主题

```go
func (p *Connect) initReplySubscribe() {
    // 订阅自己的 reply subject
    err := p.Subscribe(p.reply, func(msg *nats.Msg) {
        // 1. 从消息头获取请求 ID
        reqID := msg.Header.Get(REQ_ID)
        if reqID == "" {
            return
        }

        // 2. 查找并删除等待的 channel
        if chMsg, ok := p.waiters.LoadAndDelete(reqID); ok {
            ch := chMsg.(chan *nats.Msg)
            
            // 3. 将响应消息发送到 channel
            select {
            case ch <- msg:
            default:
            }
            close(ch)
        } else {
            clog.Warnf("Waiter not found for reqID = %s", reqID)
        }
    })
}
```

**作用**:
- 订阅当前连接的响应主题
- 接收所有发送到这个主题的响应消息
- 根据 `reqID` 将响应分发到对应的等待 channel

## 完整的 RPC 调用流程

### 场景：Gate 节点调用 Game 节点的方法

```
Gate 节点                                    Game 节点
  |                                             |
  | 1. RequestRemote(gameNodeID, packet)       |
  |    ↓                                        |
  | 2. RequestSync(subject, data)              |
  |    - reqID = 1                              |
  |    - ch = make(chan *nats.Msg, 1)          |
  |    - waiters[1] = ch                       |
  |    - msg.Reply = "reply.gate-1.0"          |
  |    - msg.Header["reqID"] = "1"             |
  |                                             |
  | 3. PublishMsg(msg) ----------------------> | 4. remoteProcess()
  |                                             |    - 接收请求
  |                                             |    - 处理业务逻辑
  |                                             |    - 构造响应
  |                                             |
  | 5. initReplySubscribe 接收 <-------------- | 6. sendResponse()
  |    - 接收到响应消息                         |    - subject = "reply.gate-1.0"
  |    - reqID = "1"                            |    - Header["reqID"] = "1"
  |    - ch = waiters[1]                        |    - Data = response
  |    - ch <- msg                              |
  |    - close(ch)                              |
  |                                             |
  | 7. select 接收到响应                        |
  |    - resp := <-ch                           |
  |    - return resp.Data                       |
  |                                             |
```

## 详细代码分析

### 步骤1：发起 RPC 请求

**Gate 节点** (`cluster.go`):

```go
func (p *Cluster) RequestRemote(nodeID string, cpacket *cproto.ClusterPacket, timeout ...time.Duration) ([]byte, int32) {
    // 1. 序列化请求数据
    msg, err := proto.Marshal(cpacket)
    
    // 2. 构造 NATS subject
    subject := GetRemoteSubject(p.prefix, nodeType, nodeID)
    // 例如: "node.remote.game.game-1"
    
    // 3. 调用 RequestSync（关键！）
    natsData, err := cnats.GetConnect().RequestSync(subject, msg, timeout...)
    
    // 4. 解析响应
    rsp := &cproto.Response{}
    proto.Unmarshal(natsData, rsp)
    
    return rsp.Data, rsp.Code
}
```

### 步骤2：RequestSync 发送请求并等待响应

**NATS Connect** (`connect.go`):

```go
func (p *Connect) RequestSync(subject string, data []byte, tod ...time.Duration) ([]byte, error) {
    timeout := GetTimeout(tod...)

    // 1. 生成唯一的请求 ID
    reqID := strconv.FormatUint(atomic.AddUint64(&p.seq, 1), 10)
    
    // 2. 创建等待 channel
    ch := make(chan *nats.Msg, 1)
    p.waiters.Store(reqID, ch)  // 保存到 waiters map
    
    // 3. 构造 NATS 消息
    msg := GetMsg()
    msg.Subject = subject              // 目标主题: "node.remote.game.game-1"
    msg.Reply = p.reply                // 响应主题: "reply.gate-1.0"
    msg.Header.Set(REQ_ID, reqID)      // 请求 ID: "1"
    msg.Header.Set(CON_ID, strconv.FormatInt(int64(p.id), 10))
    msg.Data = data
    
    // 4. 发送消息
    err := p.PublishMsg(msg)
    ReleaseMsg(msg)
    
    if err != nil {
        p.waiters.Delete(reqID)
        close(ch)
        return nil, err
    }
    
    // 5. 等待响应（阻塞）
    select {
    case resp, ok := <-ch:
        // 接收到响应
        if !ok || resp == nil {
            return nil, cerror.ClusterRequestTimeout
        }
        return resp.Data, nil
        
    case <-time.After(timeout):
        // 超时
        p.waiters.Delete(reqID)
        close(ch)
        return nil, cerror.ClusterRequestTimeout
    }
}
```

**关键数据结构**:

```go
// 请求消息
{
    Subject: "node.remote.game.game-1",  // 发送到 Game 节点
    Reply: "reply.gate-1.0",             // 响应发送到这里
    Header: {
        "reqID": "1",                     // 请求 ID
        "conID": "0"                      // 连接 ID
    },
    Data: [序列化的请求数据]
}

// waiters map
waiters = {
    "1": chan *nats.Msg  // reqID -> channel
}
```

### 步骤3：Game 节点接收请求

**Game 节点** (`cluster.go`):

```go
func (p *Cluster) remoteProcess() {
    process := func(natsMsg *nats.Msg) {
        // 1. 解析请求
        packet, err := cproto.UnmarshalPacket(natsMsg.Data)
        
        // 2. 构造消息
        message := cfacade.BuildClusterMessage(packet)
        
        // 3. 保存响应信息（关键！）
        if len(natsMsg.Reply) > 0 {
            message.Header = natsMsg.Header  // 包含 reqID
            message.Reply = natsMsg.Reply    // "reply.gate-1.0"
        }
        
        // 4. 投递到 Actor 系统处理
        p.app.ActorSystem().PostRemote(&message)
    }

    // 订阅 remote subject
    conn := cnats.GetConnect()
    err := conn.Subscribe(p.remoteSubject, process)
}
```

### 步骤4：Game 节点发送响应

**Actor 系统** (`invoke.go`):

```go
func (p *ActorSystem) invokeRemote(message *cfacade.Message) {
    // 1. 调用 Actor 方法
    rets := funcInfo.Value.Call(args)
    
    // 2. 构造响应
    rsp := &cproto.Response{Code: ccode.OK}
    if len(rets) > 0 {
        rsp.Data, _ = p.app.Serializer().Marshal(rets[0].Interface())
    }
    
    // 3. 发送响应（如果有 Reply）
    if message.Reply != "" {
        p.sendResponse(message, rsp)
    }
}

func (p *ActorSystem) sendResponse(message *cfacade.Message, rsp *cproto.Response) {
    rspData, _ := proto.Marshal(rsp)
    
    // 构造响应消息
    rspMsg := cnats.GetMsg()
    rspMsg.Header = message.Header       // 包含 reqID
    rspMsg.Subject = message.Reply       // "reply.gate-1.0"
    rspMsg.Data = rspData
    
    // 发送响应
    cnats.GetConnect().PublishMsg(rspMsg)
    cnats.ReleaseMsg(rspMsg)
}
```

**响应消息**:

```go
{
    Subject: "reply.gate-1.0",  // 发送到 Gate 节点的 reply subject
    Header: {
        "reqID": "1",            // 请求 ID（用于匹配）
        "conID": "0"
    },
    Data: [序列化的响应数据]
}
```

### 步骤5：Gate 节点接收响应

**initReplySubscribe 回调** (`connect.go`):

```go
func (p *Connect) initReplySubscribe() {
    err := p.Subscribe(p.reply, func(msg *nats.Msg) {
        // 1. 获取请求 ID
        reqID := msg.Header.Get(REQ_ID)  // "1"
        
        // 2. 查找等待的 channel
        if chMsg, ok := p.waiters.LoadAndDelete(reqID); ok {
            ch := chMsg.(chan *nats.Msg)
            
            // 3. 发送响应到 channel
            select {
            case ch <- msg:  // 将响应消息发送到 channel
            default:
            }
            close(ch)
        }
    })
}
```

### 步骤6：RequestSync 返回结果

```go
// RequestSync 中的 select 接收到响应
select {
case resp, ok := <-ch:
    // 接收到响应消息
    return resp.Data, nil  // 返回给调用者
}
```

## 关键设计

### 1. 请求-响应匹配

通过 `reqID` 匹配请求和响应：

```go
// 发送请求时
reqID = "1"
waiters["1"] = channel

// 接收响应时
reqID = msg.Header.Get("reqID")  // "1"
ch = waiters["1"]
ch <- msg
```

### 2. 唯一的 Reply Subject

每个连接有唯一的 reply subject：

```go
// Gate 节点连接 0
reply = "reply.gate-1.0"

// Gate 节点连接 1
reply = "reply.gate-1.1"

// Game 节点连接 0
reply = "reply.game-1.0"
```

**优势**:
- 避免响应消息混乱
- 支持连接池（多个连接）
- 隔离不同节点的响应

### 3. 同步等待机制

使用 channel 实现同步等待：

```go
// 创建 channel
ch := make(chan *nats.Msg, 1)
waiters[reqID] = ch

// 发送请求
PublishMsg(msg)

// 等待响应（阻塞）
select {
case resp := <-ch:
    return resp.Data
case <-time.After(timeout):
    return timeout error
}
```

## 并发请求处理

多个请求可以同时进行，每个请求有不同的 reqID：

```go
// 请求 1
reqID = "1"
waiters["1"] = ch1
// 发送请求 1

// 请求 2
reqID = "2"
waiters["2"] = ch2
// 发送请求 2

// 请求 3
reqID = "3"
waiters["3"] = ch3
// 发送请求 3

// 响应可能乱序到达
// 响应 3 先到达 -> ch3 <- msg3
// 响应 1 后到达 -> ch1 <- msg1
// 响应 2 最后到达 -> ch2 <- msg2
```

**initReplySubscribe 处理**:

```go
// 接收响应 3
reqID = "3"
ch = waiters["3"]
ch <- msg3

// 接收响应 1
reqID = "1"
ch = waiters["1"]
ch <- msg1

// 接收响应 2
reqID = "2"
ch = waiters["2"]
ch <- msg2
```

## 超时处理

如果响应超时，清理资源：

```go
select {
case resp := <-ch:
    return resp.Data
    
case <-time.After(timeout):
    // 超时：删除 waiter，关闭 channel
    p.waiters.Delete(reqID)
    close(ch)
    return cerror.ClusterRequestTimeout
}
```

## 对比：NATS 原生 Request vs RequestSync

### NATS 原生 Request

```go
func (p *Connect) Request(subject string, data []byte, timeout time.Duration) ([]byte, error) {
    // NATS 内部处理 reply subject 和匹配
    natsMsg, err := p.Conn.Request(subject, data, timeout)
    return natsMsg.Data, nil
}
```

**特点**:
- NATS 自动生成 reply subject
- NATS 内部处理响应匹配
- 简单但不够灵活

### Cherry RequestSync

```go
func (p *Connect) RequestSync(subject string, data []byte, timeout time.Duration) ([]byte, error) {
    // 自定义 reply subject 和匹配逻辑
    reqID := generateReqID()
    ch := make(chan *nats.Msg, 1)
    waiters[reqID] = ch
    
    msg.Reply = p.reply  // 固定的 reply subject
    msg.Header["reqID"] = reqID
    
    PublishMsg(msg)
    
    select {
    case resp := <-ch:
        return resp.Data, nil
    case <-time.After(timeout):
        return nil, timeout
    }
}
```

**特点**:
- 使用固定的 reply subject（每个连接一个）
- 通过 reqID 匹配请求和响应
- 支持连接池
- 更灵活的控制

## 总结

### initReplySubscribe 的作用

1. **订阅响应主题**: 订阅当前连接的 reply subject
2. **接收响应消息**: 接收所有发送到这个主题的响应
3. **分发响应**: 根据 reqID 将响应分发到对应的等待 channel
4. **唤醒等待者**: 通过 channel 唤醒阻塞的 RequestSync 调用

### 关键机制

- ✅ 每个连接有唯一的 reply subject
- ✅ 通过 reqID 匹配请求和响应
- ✅ 使用 channel 实现同步等待
- ✅ 支持并发请求
- ✅ 支持超时处理
- ✅ 自动清理资源

### 完整流程

```
1. RequestSync 创建 channel 并保存到 waiters
2. 发送请求，设置 Reply 和 reqID
3. 阻塞等待 channel
4. Game 节点处理请求
5. Game 节点发送响应到 Reply subject
6. initReplySubscribe 接收响应
7. 根据 reqID 查找 channel
8. 将响应发送到 channel
9. RequestSync 接收到响应并返回
```

这就是 `initReplySubscribe` 函数在 Cherry 框架 NATS RPC 调用中的完整作用！
