# NATS 重复消息问题分析（更新版）

## 问题现象

日志显示同一个响应在极短时间内被接收了两次：

```
10:29:09.273  NatsPublis id = 1, reqID = 1  ← 发送请求
10:29:09.283  NatsRec id = 1, reqID = 1     ← 第一次接收响应（10ms后）
10:29:09.283  NatsRes id = 1, reqID = 1     ← 处理完成
10:29:09.304  Waiter not found for id = 1, reqID = 1  ← 第二次接收（21ms后）
```

**关键发现：**
- 都是同一个连接（id = 1），所以不是连接池的问题
- 时间间隔很短（21ms），不是网络重试
- 已经调用了 `msg.Ack()`，但仍然收到重复消息

## 根本原因分析

### 1. msg.Ack() 无效的原因

**重要：`msg.Ack()` 只在 NATS JetStream 中有效！**

```go
func (p *Connect) initReplySubscribe() {
    err := p.Subscribe(p.reply, func(msg *nats.Msg) {
        if chMsg, ok := p.waiters.LoadAndDelete(reqID); ok {
            ch := chMsg.(chan *nats.Msg)
            select {
            case ch <- msg:
                msg.Ack()  // ← 如果使用普通 NATS，这个调用无效
            default:
            }
            close(ch)
        } else {
            msg.Ack()  // ← 同样无效
            clog.Warnf("Waiter not found for id = %d, reqID = %s", p.id, reqID)
        }
    })
}
```

**普通 NATS vs JetStream：**
- **普通 NATS**：消息投递是"fire-and-forget"，没有 Ack 机制，`msg.Ack()` 调用无效
- **NATS JetStream**：消息持久化，支持 Ack 确认，未 Ack 的消息会重新投递

### 2. 可能的原因

既然不是 NATS 重试，那么问题可能出在：

#### 原因 1：响应端重复发送（最可能）

**场景：** Actor 消息处理逻辑可能被调用了两次

```go
func (p *Cluster) remoteProcess() {
    process := func(natsMsg *nats.Msg) {
        packet, err := cproto.UnmarshalPacket(natsMsg.Data)
        
        // 解析目标 Actor，检查是否需要并发处理
        targetPath, err := cfacade.ToActorPath(packet.TargetPath)
        if err == nil {
            if handlerInfo, ok := isConcurrentHandler(targetPath.ActorID, packet.FuncName); ok {
                // 并发处理：直接开 goroutine，绕过 Actor mailbox
                go p.handleConcurrent(natsMsg, packet, handlerInfo)
                return
            }
        }
        
        // 非并发：走原有 Actor 逻辑
        message := cfacade.BuildClusterMessage(packet)
        p.app.ActorSystem().PostRemote(&message)  // ← 可能被处理两次？
    }
    
    conn := cnats.GetConnect()
    err := conn.Subscribe(p.remoteSubject, process)
}
```

**可能的重复发送场景：**
1. Actor 消息处理函数被调用两次
2. 业务逻辑中有重试机制
3. 响应逻辑被触发两次（例如：Response + Push）

#### 原因 2：NATS 订阅重复

**场景：** 同一个 subject 被订阅了两次

```go
func (p *Connect) Subscribe(subject string, cb nats.MsgHandler) error {
    sub, err := p.Conn.Subscribe(subject, cb)
    if err != nil {
        return err
    }
    
    if sub != nil {
        p.subs = append(p.subs, sub)  // ← 如果多次调用，会重复订阅
    }
    
    return nil
}
```

**检查方法：** 查看 `p.subs` 中是否有重复的 subject

#### 原因 3：请求端重复发送

**场景：** 请求本身被发送了两次，导致两个响应

```go
func (p *Connect) RequestSync(subject string, data []byte, tod ...time.Duration) ([]byte, error) {
    reqID := strconv.FormatUint(atomic.AddUint64(&p.seq, 1), 10)
    ch := make(chan *nats.Msg, 1)
    p.waiters.Store(reqID, ch)
    
    // ... 发送请求
    err := p.PublishMsg(msg)  // ← 如果被调用两次？
}
```

但这个可能性较小，因为 reqID 是递增的，如果请求发送两次，reqID 应该不同。

## 诊断方法

### 步骤 1：添加响应发送日志

已添加诊断日志到 `sendResponse`：

```go
func (p *Cluster) sendResponse(natsMsg *nats.Msg, rsp *cproto.Response) {
    // 添加诊断日志
    reqID := natsMsg.Header.Get("reqID")
    conID := natsMsg.Header.Get("conID")
    clog.Infof("[sendResponse] Sending response: reqID=%s, conID=%s, reply=%s", reqID, conID, natsMsg.Reply)
    
    // ... 原有代码
}
```

**预期结果：**
- 如果看到同一个 reqID 的 `[sendResponse]` 日志出现两次，说明响应端重复发送
- 如果只看到一次，说明是接收端的问题

### 步骤 2：检查订阅数量

添加日志查看订阅情况：

```go
func (p *Connect) initReplySubscribe() {
    clog.Infof("[initReplySubscribe] Subscribing to: %s, current subs count: %d", p.reply, len(p.subs))
    
    err := p.Subscribe(p.reply, func(msg *nats.Msg) {
        // ...
    })
    
    clog.Infof("[initReplySubscribe] After subscribe, subs count: %d", len(p.subs))
}
```

### 步骤 3：检查 NATS 连接类型

确认是否使用了 JetStream：

```bash
# 查看 NATS 服务器配置
cat 3rd/nats-server/nats.conf

# 或者查看连接日志
grep -i "jetstream" logs/*
```

## 解决方案

### 方案 1：添加响应去重机制（推荐）

在接收端添加去重逻辑：

```go
type Connect struct {
    *nats.Conn
    options
    id         int
    seq        uint64
    waiters    sync.Map
    processed  sync.Map  // ← 新增：记录已处理的 reqID
    subs       []*nats.Subscription
    reply      string
}

func (p *Connect) initReplySubscribe() {
    err := p.Subscribe(p.reply, func(msg *nats.Msg) {
        reqID := msg.Header.Get(REQ_ID)
        if reqID == "" {
            clog.Infof("header = %v, subject = %v", msg.Header, msg.Subject)
            return
        }
        
        // ← 新增：检查是否已处理
        if _, loaded := p.processed.LoadOrStore(reqID, true); loaded {
            clog.Debugf("Duplicate response ignored: id = %d, reqID = %s", p.id, reqID)
            msg.Ack()  // 即使无效也调用，保持一致性
            return
        }
        
        // 设置过期时间，避免 processed map 无限增长
        go func() {
            time.Sleep(30 * time.Second)
            p.processed.Delete(reqID)
        }()
        
        if chMsg, ok := p.waiters.LoadAndDelete(reqID); ok {
            clog.Infof("NatsRec id = %d, reqID = %s", p.id, reqID)
            ch := chMsg.(chan *nats.Msg)
            select {
            case ch <- msg:
                msg.Ack()
            default:
            }
            close(ch)
        } else {
            msg.Ack()
            clog.Debugf("Waiter not found for id = %d, reqID = %s", p.id, reqID)  // ← 改为 Debugf
        }
    })
}
```

### 方案 2：排查响应端重复发送

如果诊断日志显示 `sendResponse` 被调用两次，需要排查：

1. **检查 Actor 消息处理逻辑**：
   - 是否有重试机制
   - 是否有多个地方调用了响应函数

2. **检查并发处理逻辑**：
   - `handleConcurrent` 是否被调用多次
   - Actor 消息是否被重复投递

3. **检查业务逻辑**：
   - 是否同时调用了 Response 和 Push
   - 是否有定时器或回调导致重复调用

### 方案 3：临时方案 - 降低日志级别

如果问题不影响功能，可以先降低日志级别：

```go
clog.Debugf("Waiter not found for id = %d, reqID = %s", p.id, reqID)  // Warnf 改为 Debugf
```

## 下一步行动

1. **运行程序，查看新增的诊断日志**：
   - 检查 `[sendResponse]` 日志是否出现两次
   - 确认是响应端重复发送还是接收端重复接收

2. **根据诊断结果选择方案**：
   - 如果是响应端重复发送 → 排查业务逻辑
   - 如果是接收端重复接收 → 实施方案 1（去重机制）

3. **验证修复效果**：
   - 运行压力测试
   - 确认警告消失

## 总结

**之前的分析错误：** 不是连接池的问题（日志显示都是 id = 1）

**新的分析：**
1. `msg.Ack()` 在普通 NATS 中无效，不能阻止重复
2. 最可能的原因是响应端重复发送（需要诊断日志确认）
3. 推荐方案是添加去重机制，彻底解决重复接收问题

**关键诊断：** 查看新增的 `[sendResponse]` 日志，确认响应是否被发送两次。

### 1. 连接池架构

**pool.go 中的连接池设计：**

```go
var (
    connectPool    []*Connect                      // connect pool
    connectSize    uint64                          // connect size
    roundIndex     *uint64       = new(uint64)     // round-robin index
)

func NewPool(replySubject string, config cfacade.ProfileJSON, isConnect bool) {
    poolSize := config.GetInt("pool_size", 1)
    
    for id := 1; id <= poolSize; id++ {
        conn := NewConnect(id, replySubject, ...)
        connectPool = append(connectPool, conn)
    }
}

func GetConnect() *Connect {
    index := atomic.AddUint64(roundIndex, 1)
    return connectPool[index%connectSize]  // ← 轮询获取连接
}
```

**关键点：**
- 连接池包含多个 NATS 连接（pool_size 配置）
- 使用 Round-Robin 算法轮询选择连接
- 每个连接有独立的 reply subject：`{replySubject}.{id}`

### 2. Reply Subject 订阅机制

**connect.go 中每个连接的订阅：**

```go
func NewConnect(id int, replySubject string, opts ...OptionFunc) *Connect {
    conn := &Connect{
        id:    id,
        reply: fmt.Sprintf("%s.%d", replySubject, id),  // ← 每个连接有独立的 reply subject
    }
    return conn
}

func (p *Connect) initReplySubscribe() {
    err := p.Subscribe(p.reply, func(msg *nats.Msg) {
        reqID := msg.Header.Get(REQ_ID)
        if chMsg, ok := p.waiters.LoadAndDelete(reqID); ok {
            ch := chMsg.(chan *nats.Msg)
            ch <- msg
            close(ch)
        } else {
            clog.Warnf("Waiter not found for id = %d, reqID = %s", p.id, reqID)
        }
    })
}
```

**关键点：**
- 每个连接订阅自己的 reply subject：`node.gate.gate-1.reply.1`, `node.gate.gate-1.reply.2` 等
- 每个连接维护自己的 `waiters` map

### 3. 请求发送逻辑

**connect.go 中的 RequestSync：**

```go
func (p *Connect) RequestSync(subject string, data []byte, tod ...time.Duration) ([]byte, error) {
    reqID := strconv.FormatUint(atomic.AddUint64(&p.seq, 1), 10)
    ch := make(chan *nats.Msg, 1)
    p.waiters.Store(reqID, ch)  // ← 在当前连接的 waiters 中注册
    
    msg := GetMsg()
    msg.Subject = subject
    msg.Reply = p.reply  // ← 使用当前连接的 reply subject
    msg.Header.Set(REQ_ID, reqID)
    msg.Data = data
    
    err := p.PublishMsg(msg)
    // ...
}
```

**关键点：**
- 请求使用当前连接的 reply subject
- waiter 只注册在当前连接的 waiters map 中

### 4. 响应发送逻辑

**cluster.go 中的 sendResponse：**

```go
func (p *Cluster) sendResponse(natsMsg *nats.Msg, rsp *cproto.Response) {
    rspData, _ := proto.Marshal(rsp)
    
    rspMsg := cnats.GetMsg()
    rspMsg.Header = natsMsg.Header  // ← 复制请求的 header（包含 reqID）
    rspMsg.Subject = natsMsg.Reply  // ← 使用请求的 reply subject
    rspMsg.Data = rspData
    
    if err := cnats.GetConnect().PublishMsg(rspMsg); err != nil {  // ← 问题在这里！
        clog.Warn(err)
    }
    cnats.ReleaseMsg(rspMsg)
}
```

**问题所在：**
- `cnats.GetConnect()` 使用 Round-Robin 算法随机选择一个连接
- 响应可能通过**不同于请求的连接**发送
- 但响应的 subject 是请求连接的 reply subject

## 问题场景重现

### 场景 1：正常情况（pool_size = 1）

```
1. 请求：通过 conn-1 发送，reply = "node.gate.gate-1.reply.1"
2. 响应：通过 conn-1 发送到 "node.gate.gate-1.reply.1"
3. 接收：conn-1 订阅了 "node.gate.gate-1.reply.1"，正常接收
```

### 场景 2：问题情况（pool_size > 1）

```
1. 请求：通过 conn-1 发送
   - reply = "node.gate.gate-1.reply.1"
   - reqID = "123"
   - waiter 注册在 conn-1.waiters["123"]

2. 响应：通过 conn-2 发送（Round-Robin 选择）
   - subject = "node.gate.gate-1.reply.1"（来自请求的 reply）
   - reqID = "123"

3. 接收问题：
   - conn-1 订阅了 "node.gate.gate-1.reply.1"，收到响应
   - conn-1.waiters["123"] 存在，正常处理 ✓
   
4. 但是！如果 NATS 使用了 JetStream 或者有重试机制：
   - 消息可能被重新投递
   - 或者响应端重复发送
   - 第二次接收时，waiter 已被删除，导致警告 ✗
```

### 场景 3：更严重的情况

```
1. 请求 A：通过 conn-1 发送，reqID = "123"
2. 请求 B：通过 conn-2 发送，reqID = "123"（seq 重置或并发）
3. 响应 A：发送到 conn-1 的 reply subject
4. 响应 B：发送到 conn-2 的 reply subject
5. 如果响应发送时选错了连接，可能导致：
   - 响应 A 被 conn-2 接收（找不到 waiter）
   - 响应 B 被 conn-1 接收（找不到 waiter）
```

## 为什么会延迟 18 秒？

可能的原因：

1. **NATS JetStream 重试机制**：
   - 如果使用了 JetStream，消息会至少投递一次
   - 如果第一次没有 Ack（或 Ack 失败），会重新投递
   - 重试间隔可能是 18 秒

2. **网络延迟或重连**：
   - NATS 连接断开重连
   - 重连后消息重新投递

3. **响应端重复发送**：
   - Actor 消息重复处理
   - 业务逻辑重复调用 sendResponse

## 解决方案

### 方案 1：修复 sendResponse 使用正确的连接（推荐）

**问题：** `sendResponse` 使用 `cnats.GetConnect()` 随机选择连接

**解决：** 从请求消息的 header 中获取连接 ID，使用相同的连接发送响应

```go
func (p *Cluster) sendResponse(natsMsg *nats.Msg, rsp *cproto.Response) {
    rspData, _ := proto.Marshal(rsp)
    
    rspMsg := cnats.GetMsg()
    rspMsg.Header = natsMsg.Header
    rspMsg.Subject = natsMsg.Reply
    rspMsg.Data = rspData
    
    // ← 修改：从 header 获取连接 ID，使用相同的连接
    conIDStr := natsMsg.Header.Get(CON_ID)
    if conIDStr != "" {
        if conID, err := strconv.Atoi(conIDStr); err == nil {
            conn := cnats.GetConnectByID(conID)
            if conn != nil {
                if err := conn.PublishMsg(rspMsg); err != nil {
                    clog.Warn(err)
                }
                cnats.ReleaseMsg(rspMsg)
                return
            }
        }
    }
    
    // 降级：如果找不到指定连接，使用默认连接
    if err := cnats.GetConnect().PublishMsg(rspMsg); err != nil {
        clog.Warn(err)
    }
    cnats.ReleaseMsg(rspMsg)
}
```

**需要在 pool.go 中添加：**

```go
func GetConnectByID(id int) *Connect {
    if id > 0 && id <= int(connectSize) {
        return connectPool[id-1]
    }
    return nil
}
```

### 方案 2：使用统一的 Reply Subject

**问题：** 每个连接有独立的 reply subject

**解决：** 所有连接使用同一个 reply subject，但通过 reqID 区分

```go
func NewConnect(id int, replySubject string, opts ...OptionFunc) *Connect {
    conn := &Connect{
        id:    id,
        reply: replySubject,  // ← 不再添加 .{id} 后缀
    }
    return conn
}
```

**问题：** 这样会导致所有连接都订阅同一个 subject，可能收到不属于自己的消息

### 方案 3：使用全局 waiters（不推荐）

**问题：** 每个连接有独立的 waiters map

**解决：** 使用全局的 waiters map

```go
var globalWaiters sync.Map  // 全局 waiters

func (p *Connect) RequestSync(...) {
    globalWaiters.Store(reqID, ch)  // 注册到全局
    // ...
}

func (p *Connect) initReplySubscribe() {
    err := p.Subscribe(p.reply, func(msg *nats.Msg) {
        reqID := msg.Header.Get(REQ_ID)
        if chMsg, ok := globalWaiters.LoadAndDelete(reqID); ok {
            // ...
        }
    })
}
```

**问题：** 破坏了连接池的隔离性，不推荐

### 方案 4：将警告改为 Debug 级别（临时方案）

如果上述方案实施复杂，可以先将警告降级：

```go
func (p *Connect) initReplySubscribe() {
    err := p.Subscribe(p.reply, func(msg *nats.Msg) {
        reqID := msg.Header.Get(REQ_ID)
        if chMsg, ok := p.waiters.LoadAndDelete(reqID); ok {
            // ...
        } else {
            msg.Ack()
            clog.Debugf("Waiter not found for id = %d, reqID = %s", p.id, reqID)  // ← Warnf 改为 Debugf
        }
    })
}
```

## 推荐实施顺序

1. **立即实施方案 4**：将警告降级为 Debug，减少日志噪音
2. **验证方案 1**：添加诊断日志，确认是否是连接选择问题
3. **实施方案 1**：修复 sendResponse 使用正确的连接
4. **测试验证**：运行压力测试，确认问题解决

## 验证方法

### 1. 添加诊断日志

在 `sendResponse` 中添加：

```go
func (p *Cluster) sendResponse(natsMsg *nats.Msg, rsp *cproto.Response) {
    conIDStr := natsMsg.Header.Get(CON_ID)
    reqIDStr := natsMsg.Header.Get(REQ_ID)
    currentConn := cnats.GetConnect()
    
    clog.Debugf("[sendResponse] reqID=%s, request_conID=%s, response_conID=%d, reply=%s",
        reqIDStr, conIDStr, currentConn.GetID(), natsMsg.Reply)
    
    // ... 原有代码
}
```

### 2. 检查日志

如果看到 `request_conID != response_conID`，说明确实是连接选择问题。

### 3. 检查配置

查看 `config/demo-cluster.json` 中的 `pool_size` 配置：

```json
{
  "cluster": {
    "nats": {
      "pool_size": 2  // ← 如果 > 1，可能触发问题
    }
  }
}
```

## 总结

**根本原因：** 响应发送时使用了错误的连接（Round-Robin 选择），导致响应可能通过不同于请求的连接发送，虽然 reply subject 正确，但如果有重试机制，可能导致重复接收。

**最佳解决方案：** 修改 `sendResponse` 从请求 header 中获取连接 ID，使用相同的连接发送响应，保证请求-响应使用同一个连接。

**临时方案：** 将警告降级为 Debug 级别，因为这个问题不影响功能（第一次已经正常处理）。
