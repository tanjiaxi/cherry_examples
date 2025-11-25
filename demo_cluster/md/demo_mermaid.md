```mermaid
graph LR
    subgraph "检测层"
        A[NATS监控] --> D[节点状态管理器]
        B[Discovery监控] --> D
        C[应用层心跳] --> D
    end
    
    subgraph "处理层"
        D --> E[玩家识别器]
        E --> F[节点选择器]
        F --> G[状态迁移器]
        G --> H[通知管理器]
    end
    
    subgraph "恢复层"
        H --> I[玩家重连]
        I --> J[状态恢复]
        J --> K[游戏继续]
    end
    
    subgraph "存储层"
        L[Redis缓存] --> G
        M[数据库] --> G
        G --> L
        G --> M
    end
```#
 Cherry框架 Mermaid 图表集合

## 节点崩溃检测与处理流程

### 1. 节点崩溃检测序列图
*创建时间: 2025-01-27*
*描述: 展示从Game节点崩溃到Gate节点检测并处理的完整时序*

```mermaid
sequenceDiagram
    participant Client as 客户端
    participant Gate as Gate节点
    participant Game as Game节点
    participant NATS as NATS服务器
    participant Master as Master节点
    participant Discovery as Discovery服务

    Note over Game: Game节点正常运行
    Client->>Gate: 发送游戏消息
    Gate->>Game: 转发消息到Game节点
    Game->>Gate: 返回响应
    Gate->>Client: 转发响应

    Note over Game: Game节点崩溃
    Game--xNATS: 连接断开
    NATS->>Master: DisconnectHandler触发
    Master->>Discovery: 检测到节点离线
    Discovery->>Gate: 广播节点下线通知
    
    Client->>Gate: 继续发送消息
    Gate->>Game: 尝试转发消息
    Note over Gate: 消息发送失败
    Gate->>Client: 返回错误或重新路由
```

### 2. 节点检测机制架构图
*创建时间: 2025-01-27*
*描述: 三层检测体系的架构关系*

```mermaid
graph TB
    subgraph "NATS连接层检测"
        A[NATS Client] --> B[DisconnectErrHandler]
        A --> C[ReconnectHandler]
        A --> D[ClosedHandler]
        A --> E[ErrorHandler]
    end
    
    subgraph "Discovery服务层检测"
        F[Master Node] --> G[注册检测]
        F --> H[注销检测]
        F --> I[心跳检测]
        F --> J[节点广播]
    end
    
    subgraph "应用层检测"
        K[CheckCenter Component] --> L[启动检测]
        K --> M[定期Ping]
        K --> N[RPC超时检测]
    end
    
    B --> F
    C --> F
    D --> F
    E --> F
    
    G --> O[Gate节点]
    H --> O
    I --> O
    J --> O
    
    L --> P[Game节点]
    M --> P
    N --> P
```

### 3. 玩家重新路由流程图
*创建时间: 2025-01-27*
*描述: 检测到节点崩溃后的玩家处理流程*

```mermaid
flowchart TD
    A[检测到Game节点崩溃] --> B{获取受影响玩家}
    B --> C[遍历所有Agent]
    C --> D[检查session中的serverID]
    D --> E[收集绑定到崩溃节点的玩家]
    
    E --> F{选择新的Game节点}
    F -->|有可用节点| G[更新session中的serverID]
    F -->|无可用节点| H[踢掉玩家连接]
    
    G --> I[通知玩家重新连接]
    I --> J[可选: 恢复玩家状态]
    J --> K[完成玩家迁移]
    
    H --> L[发送服务器维护通知]
    L --> M[断开玩家连接]
```

### 4. 完整的崩溃处理时序图
*创建时间: 2025-01-27*
*描述: 包含玩家状态恢复的完整处理流程*

```mermaid
sequenceDiagram
    participant P as 玩家
    participant G as Gate节点
    participant Game1 as Game节点1(崩溃)
    participant Game2 as Game节点2(新)
    participant M as Master节点
    participant Redis as Redis/数据库

    Note over Game1: 节点正常运行
    P->>G: 游戏操作请求
    G->>Game1: 转发到Game节点1
    Game1->>G: 处理结果
    G->>P: 返回结果

    Note over Game1: 节点崩溃
    Game1--xM: 连接断开
    M->>G: 广播节点下线通知
    
    Note over G: 检测到节点崩溃
    G->>G: 识别受影响玩家
    G->>Game2: 检查节点可用性
    Game2->>G: 确认可用
    
    G->>Redis: 保存玩家状态(可选)
    G->>G: 更新session serverID
    G->>P: 发送重连通知
    
    P->>G: 重新连接/继续游戏
    G->>Game2: 转发到新Game节点
    Game2->>Redis: 加载玩家状态(可选)
    Game2->>G: 处理结果
    G->>P: 返回结果
```

### 5. 节点状态监控流程图
*创建时间: 2025-01-27*
*描述: 节点状态变化的状态机*

```mermaid
stateDiagram-v2
    [*] --> Online: 节点启动
    Online --> Monitoring: 开始监控
    
    state Monitoring {
        [*] --> HeartbeatCheck
        HeartbeatCheck --> NATSCheck: 每30秒
        NATSCheck --> DiscoveryCheck
        DiscoveryCheck --> HeartbeatCheck: 正常
        
        NATSCheck --> Suspected: NATS连接异常
        DiscoveryCheck --> Suspected: Discovery检测异常
        HeartbeatCheck --> Suspected: 心跳超时
    }
    
    Suspected --> Crashed: 确认崩溃
    Suspected --> Monitoring: 恢复正常
    
    Crashed --> PlayerMigration: 开始玩家迁移
    PlayerMigration --> Cleanup: 清理资源
    Cleanup --> [*]: 处理完成
```

### 6. 改进后的错误处理架构图
*创建时间: 2025-01-27*
*描述: 完整的错误处理系统架构*

```mermaid
graph LR
    subgraph "检测层"
        A[NATS监控] --> D[节点状态管理器]
        B[Discovery监控] --> D
        C[应用层心跳] --> D
    end
    
    subgraph "处理层"
        D --> E[玩家识别器]
        E --> F[节点选择器]
        F --> G[状态迁移器]
        G --> H[通知管理器]
    end
    
    subgraph "恢复层"
        H --> I[玩家重连]
        I --> J[状态恢复]
        J --> K[游戏继续]
    end
    
    subgraph "存储层"
        L[Redis缓存] --> G
        M[数据库] --> G
        G --> L
        G --> M
    end
```

---

*注意: 以上图表展示了Cherry框架中节点崩溃检测和处理的完整机制，可用于系统设计和故障排查参考。*

```mermaid
flowchart TD
    A[现有项目分析] --> B{项目类型判断}
    B -->|Web API服务| C[高度适合]
    B -->|实时游戏| D[完美适合]
    B -->|微服务架构| E[非常适合]
    B -->|单体应用| F[需要重构]
    
    C --> G[使用Web节点 + Gin组件]
    D --> H[使用完整集群架构]
    E --> I[使用Actor系统拆分]
    F --> J[逐步迁移策略]
    
    G --> K[移植建议]
    H --> K
    I --> K
    J --> K
```#
# Cherry框架配置系统

### 1. Config结构和Wrap函数关系图
*创建时间: 2025-01-27*
*描述: 展示Config结构如何通过Wrap函数统一不同数据源的访问*

```mermaid
flowchart TD
    A[任意Go数据类型] --> B[Wrap函数]
    B --> C[Config对象]
    C --> D[统一的配置访问接口]
    
    E[JSON字符串] --> B
    F[map[string]interface{}] --> B
    G[struct结构体] --> B
    H[slice切片] --> B
```

### 2. 配置数据流转架构
*创建时间: 2025-01-27*
*描述: 从配置文件到应用组件的完整数据流*

```mermaid
graph TB
    subgraph "数据来源"
        A[JSON文件] --> D[Wrap函数]
        B[YAML文件] --> D
        C[环境变量] --> D
        E[数据库配置] --> D
    end
    
    subgraph "Config对象"
        D --> F[统一的访问接口]
        F --> G[GetString]
        F --> H[GetInt]
        F --> I[GetBool]
        F --> J[GetConfig]
    end
    
    subgraph "应用层"
        G --> K[业务代码]
        H --> K
        I --> K
        J --> K
    end
```

### 3. 配置加载时序图
*创建时间: 2025-01-27*
*描述: Cherry框架中配置加载和使用的完整时序*

```mermaid
sequenceDiagram
    participant App as 应用启动
    participant Profile as Profile模块
    participant Config as Config对象
    participant Component as 组件

    App->>Profile: loadFile("config.json")
    Profile->>Profile: 读取JSON文件
    Profile->>Config: Wrap(jsonData)
    Config->>App: 返回Config对象
    
    App->>Component: 初始化组件
    Component->>Config: GetConfig("nats")
    Config->>Component: 返回NATS配置
    Component->>Component: 创建NATS连接
```#
# Cherry框架Agent系统

### 1. Agent架构层次图
*创建时间: 2025-01-27*
*描述: Agent在Cherry框架中的层次结构和职责分工*

```mermaid
graph TB
    subgraph "客户端层"
        A[游戏客户端1] --> D[TCP/WebSocket连接1]
        B[游戏客户端2] --> E[TCP/WebSocket连接2]  
        C[游戏客户端3] --> F[TCP/WebSocket连接3]
    end
    
    subgraph "Gate节点 - Agent层"
        D --> G[Agent1]
        E --> H[Agent2]
        F --> I[Agent3]
    end
    
    subgraph "业务Actor层"
        G --> J[ActorAgent1]
        H --> K[ActorAgent2]
        I --> L[ActorAgent3]
    end
    
    subgraph "Game节点"
        J --> M[Player Actor1]
        K --> N[Player Actor2]
        L --> O[Player Actor3]
    end
```

### 2. Agent生命周期状态图
*创建时间: 2025-01-27*
*描述: Agent从创建到销毁的完整生命周期*

```mermaid
stateDiagram-v2
    [*] --> AgentInit: 客户端连接
    AgentInit --> AgentWaitAck: 等待握手确认
    AgentWaitAck --> AgentWorking: 握手成功
    AgentWorking --> AgentClosed: 连接断开
    AgentClosed --> [*]: 清理资源
    
    AgentWorking --> AgentWorking: 处理消息
    AgentInit --> AgentClosed: 连接失败
    AgentWaitAck --> AgentClosed: 握手超时
```

### 3. Agent消息处理时序图
*创建时间: 2025-01-27*
*描述: 从客户端消息到业务处理的完整流程*

```mermaid
sequenceDiagram
    participant Client as 客户端
    participant Agent as Agent(网络层)
    participant ActorAgent as ActorAgent(业务层)
    participant GameActor as Game Actor

    Client->>Agent: TCP/WebSocket消息
    Agent->>Agent: 解析Pomelo协议
    Agent->>ActorAgent: 转发业务消息
    ActorAgent->>GameActor: RPC调用
    GameActor->>ActorAgent: 返回结果
    ActorAgent->>Agent: 响应消息
    Agent->>Client: 发送响应
```

### 4. Agent设计优势图
*创建时间: 2025-01-27*
*描述: Agent设计带来的核心优势和价值*

```mermaid
graph TB
    subgraph "Agent设计优势"
        A[连接抽象] --> B[统一接口]
        C[状态管理] --> D[生命周期控制]
        E[消息队列] --> F[异步处理]
        G[会话绑定] --> H[用户识别]
        I[协议解析] --> J[Pomelo协议支持]
    end
    
    B --> K[简化业务开发]
    D --> K
    F --> K
    H --> K
    J --> K
```