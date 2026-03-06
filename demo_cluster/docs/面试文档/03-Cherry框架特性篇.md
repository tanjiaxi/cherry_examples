# 第三篇: Cherry框架核心特性

## 一、Cherry框架简介

Cherry是一个专为游戏服务器设计的Golang分布式框架,基于Actor模型,提供了完整的游戏服务器解决方案。

**核心特性**:
- Actor并发模型
- 分布式RPC通信
- 服务发现与注册
- 组件化架构
- 热更新支持
- 多协议支持(TCP/WebSocket/HTTP)

**GitHub**: https://github.com/cherry-game/cherry

## 二、Actor并发模型(核心优势)

### 2.1 Actor模型原理

**传统并发模型问题**:
- 共享内存 + 锁 = 复杂度高
- 死锁、竞态条件难以调试
- 性能瓶颈在锁竞争

**Actor模型优势**:
- 消息驱动,无共享状态
- 单线程处理,无需加锁
- 天然支持分布式
- 易于理解和维护

### 2.2 Cherry Actor实现

```go
// 定义Actor
type ActorPlayer struct {
    cherry.ActorBase
    UserId int64
    Coin   int64
}

// 初始化
func (a *ActorPlayer) OnInit() {
    a.loadData()
}

// 消息处理
func (a *ActorPlayer) OnReceive(msg cherry.Message) {
    switch msg.Route {
    case "player.spin":
        a.handleSpin(msg)
    }
}

// 停止
func (a *ActorPlayer) OnStop() {
    a.saveData()
}

// 创建Actor
playerActor := cherry.NewActor("player", userId)
```

### 2.3 Actor生命周期

```
创建 → 初始化(OnInit) → 运行(OnReceive) → 停止(OnStop) → 销毁
```

**生命周期管理**:
```go
// Actor管理器
type ActorManager struct {
    actors map[int64]*ActorPlayer
    mu     sync.RWMutex
}

// 获取或创建Actor
func (m *ActorManager) GetOrCreate(userId int64) *ActorPlayer {
    m.mu.RLock()
    actor, exists := m.actors[userId]
    m.mu.RUnlock()
    
    if exists {
        return actor
    }
    
    // 创建新Actor
    m.mu.Lock()
    defer m.mu.Unlock()
    
    actor = &ActorPlayer{UserId: userId}
    actor.OnInit()
    m.actors[userId] = actor
    
    return actor
}

// 移除Actor
func (m *ActorManager) Remove(userId int64) {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    if actor, exists := m.actors[userId]; exists {
        actor.OnStop()
        delete(m.actors, userId)
    }
}
```

## 三、分布式RPC通信

### 3.1 RPC调用方式

**本地调用**:
```go
// 同节点Actor调用
result := cherry.Call(
    "player.spin",
    &pb.SpinRequest{BetAmount: 100},
)
```

**远程调用**:
```go
// 跨节点RPC调用
result := cherry.CallRemote(
    "game-node-1",  // 目标节点
    "player.spin",
    &pb.SpinRequest{BetAmount: 100},
)
```

**广播调用**:
```go
// 广播到所有Game节点
cherry.Broadcast(
    "game",  // 节点类型
    "system.reload_config",
    nil,
)
```

### 3.2 RPC实现原理

```go
// RPC请求结构
type RPCRequest struct {
    RequestId string
    Route     string
    Data      []byte
    Timeout   time.Duration
}

// RPC响应结构
type RPCResponse struct {
    RequestId string
    Data      []byte
    Error     string
}

// RPC调用流程
func Call(route string, req interface{}) (interface{}, error) {
    // 1. 序列化请求
    data, _ := proto.Marshal(req)
    
    // 2. 生成请求ID
    requestId := uuid.New().String()
    
    // 3. 创建响应通道
    respChan := make(chan *RPCResponse, 1)
    pendingRequests[requestId] = respChan
    
    // 4. 发送请求
    natsConn.Publish(route, &RPCRequest{
        RequestId: requestId,
        Route:     route,
        Data:      data,
    })
    
    // 5. 等待响应
    select {
    case resp := <-respChan:
        return proto.Unmarshal(resp.Data)
    case <-time.After(5 * time.Second):
        return nil, errors.New("timeout")
    }
}
```

## 四、服务发现与注册

### 4.1 支持的注册中心

**etcd** (生产推荐):
```go
cherry.Configure(
    cherry.WithDiscovery("etcd"),
    cherry.WithDiscoveryAddress("127.0.0.1:2379"),
)
```

**NATS** (轻量级):
```go
cherry.Configure(
    cherry.WithDiscovery("nats"),
    cherry.WithDiscoveryAddress("nats://127.0.0.1:4222"),
)
```

**默认** (单机开发):
```go
cherry.Configure(
    cherry.WithDiscovery("default"),
)
```

### 4.2 服务注册流程

```go
// 节点启动时注册
func (n *GameNode) Start() {
    // 1. 生成节点ID
    nodeId := fmt.Sprintf("game-%s", uuid.New())
    
    // 2. 注册到etcd
    discovery.Register(&NodeInfo{
        NodeId:   nodeId,
        NodeType: "game",
        Address:  "127.0.0.1:10001",
        Metadata: map[string]string{
            "version": "1.0.0",
            "region":  "asia",
        },
    })
    
    // 3. 定时心跳
    go n.heartbeat()
}

// 心跳保活
func (n *GameNode) heartbeat() {
    ticker := time.NewTicker(10 * time.Second)
    for range ticker.C {
        discovery.Heartbeat(n.NodeId)
    }
}
```

### 4.3 服务发现流程

```go
// 查找Game节点
func findGameNode(userId int64) string {
    // 1. 获取所有Game节点
    nodes := discovery.GetNodes("game")
    
    // 2. 一致性哈希选择节点
    hash := crc32.ChecksumIEEE([]byte(fmt.Sprint(userId)))
    index := hash % uint32(len(nodes))
    
    return nodes[index].NodeId
}

// 路由消息到目标节点
func routeMessage(userId int64, msg Message) {
    nodeId := findGameNode(userId)
    cherry.CallRemote(nodeId, msg.Route, msg.Data)
}
```

## 五、组件化架构

### 5.1 内置组件

**HTTP Server**:
```go
import "github.com/cherry-game/components/gin"

app.Register(gin.New(
    gin.WithAddr(":8080"),
    gin.WithMode("release"),
))
```

**WebSocket Connector**:
```go
import "github.com/cherry-game/cherry/net/connector"

app.Register(connector.NewWS(
    connector.WithAddr(":20010"),
    connector.WithPath("/ws"),
))
```

**Database (GORM)**:
```go
import "github.com/cherry-game/components/gorm"

app.Register(gorm.New(
    gorm.WithDSN("postgres://..."),
    gorm.WithMaxOpenConns(100),
))
```

### 5.2 自定义组件

```go
// 定义组件
type MetricsComponent struct {
    cherry.Component
    registry *prometheus.Registry
}

// 组件名称
func (c *MetricsComponent) Name() string {
    return "metrics"
}

// 初始化
func (c *MetricsComponent) Init() {
    c.registry = prometheus.NewRegistry()
    // 注册指标
}

// 启动
func (c *MetricsComponent) Start() {
    // 启动HTTP服务暴露指标
    http.Handle("/metrics", promhttp.HandlerFor(
        c.registry,
        promhttp.HandlerOpts{},
    ))
    go http.ListenAndServe(":9090", nil)
}

// 停止
func (c *MetricsComponent) Stop() {
    // 清理资源
}

// 注册组件
app.Register(&MetricsComponent{})
```

## 六、热更新支持

### 6.1 配置热更新

```go
// 配置管理器
type ConfigManager struct {
    configs map[string]interface{}
    version int64
    mu      sync.RWMutex
}

// 加载配置
func (m *ConfigManager) Load() {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // 从文件/数据库加载
    newConfigs := loadConfigFromFile()
    m.configs = newConfigs
    m.version++
}

// 获取配置
func (m *ConfigManager) Get(key string) interface{} {
    m.mu.RLock()
    defer m.mu.RUnlock()
    return m.configs[key]
}

// 热更新
func (m *ConfigManager) Reload() {
    m.Load()
    // 通知所有节点
    cherry.Broadcast("game", "config.reload", nil)
}
```

### 6.2 代码热更新(插件化)

```go
// 插件接口
type Plugin interface {
    Name() string
    Init()
    Execute(ctx context.Context) error
}

// 插件管理器
type PluginManager struct {
    plugins map[string]Plugin
}

// 加载插件
func (m *PluginManager) Load(name string) error {
    // 动态加载.so文件
    p, err := plugin.Open(fmt.Sprintf("plugins/%s.so", name))
    if err != nil {
        return err
    }
    
    // 查找符号
    symbol, err := p.Lookup("NewPlugin")
    if err != nil {
        return err
    }
    
    // 创建插件实例
    newPlugin := symbol.(func() Plugin)
    plugin := newPlugin()
    plugin.Init()
    
    m.plugins[name] = plugin
    return nil
}
```

## 七、多协议支持

### 7.1 Pomelo协议

**特点**: 二进制协议,高效压缩

```go
// Pomelo Agent
type PomeloAgent struct {
    conn net.Conn
    serializer pomelo.Serializer
}

// 发送消息
func (a *PomeloAgent) Send(route string, data interface{}) {
    // 序列化
    bytes, _ := a.serializer.Marshal(data)
    
    // 编码Pomelo包
    packet := pomelo.Encode(route, bytes)
    
    // 发送
    a.conn.Write(packet)
}

// 接收消息
func (a *PomeloAgent) Receive() (*Message, error) {
    // 读取包头
    header := make([]byte, 4)
    a.conn.Read(header)
    
    // 读取包体
    length := binary.BigEndian.Uint32(header)
    body := make([]byte, length)
    a.conn.Read(body)
    
    // 解码
    return pomelo.Decode(body)
}
```

### 7.2 Simple协议

**特点**: 简单文本协议,易于调试

```go
// Simple协议格式: route|data\n
func (a *SimpleAgent) Send(route string, data string) {
    msg := fmt.Sprintf("%s|%s\n", route, data)
    a.conn.Write([]byte(msg))
}
```

### 7.3 自定义协议

```go
// 实现Serializer接口
type CustomSerializer struct{}

func (s *CustomSerializer) Marshal(v interface{}) ([]byte, error) {
    // 自定义序列化逻辑
}

func (s *CustomSerializer) Unmarshal(data []byte, v interface{}) error {
    // 自定义反序列化逻辑
}

// 注册自定义序列化器
cherry.RegisterSerializer("custom", &CustomSerializer{})
```

## 八、Cherry vs 其他框架

### 8.1 对比表

| 特性 | Cherry | Leaf | Nano | KBEngine |
|------|--------|------|------|----------|
| 语言 | Go | Go | Go | Python/C++ |
| Actor模型 | ✅ | ❌ | ❌ | ❌ |
| 分布式 | ✅ | ✅ | ✅ | ✅ |
| 服务发现 | etcd/NATS | 自实现 | 无 | 自实现 |
| 热更新 | ✅ | ❌ | ❌ | ✅ |
| 组件化 | ✅ | ❌ | ❌ | ✅ |
| 学习曲线 | 中 | 低 | 低 | 高 |
| 社区活跃度 | 中 | 高 | 中 | 高 |

### 8.2 Cherry优势

1. **Actor模型**: 天然支持高并发,无锁设计
2. **组件化**: 易于扩展和维护
3. **服务发现**: 支持多种注册中心
4. **热更新**: 配置实时生效
5. **轻量级**: 性能开销小
6. **游戏优化**: 专为游戏场景设计

### 8.3 适用场景

**适合**:
- 高并发游戏服务器
- 分布式系统
- 需要Actor模型的场景
- 需要快速开发的项目

**不适合**:
- 超大规模MMO(需要更复杂的架构)
- 对延迟要求极高的场景(< 10ms)
- 需要成熟生态的项目

---

**下一篇**: [实战场景篇](./04-实战场景篇.md)
