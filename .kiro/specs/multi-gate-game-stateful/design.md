# Design Document: Multi-Gate Multi-Game Stateful Architecture

## Overview

本设计实现多Gate、多Game节点的有状态服务架构。核心功能：
1. Center节点维护玩家位置信息（PlayerLocation）
2. 使用在线人数负载均衡分配Game节点
3. 游戏请求路由到玩家绑定的Game节点
4. 断线重连时恢复到原节点
5. Game节点故障时自动迁移玩家

**设计原则**：
- 使用在线人数作为负载指标（简单有效）
- 内存缓存 + 数据库持久化
- 通过etcd心跳检测节点健康状态

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                         Client                                   │
└─────────────────────────────────────────────────────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌──────────┐   ┌──────────┐   ┌──────────┐
        │ Gate-1   │   │ Gate-2   │   │ Gate-N   │
        │ :10010   │   │ :10011   │   │ :1001N   │
        └──────────┘   └──────────┘   └──────────┘
              │               │               │
              └───────────────┼───────────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │     Center       │
                    │ (PlayerLocation) │
                    │ (在线人数统计)    │
                    └──────────────────┘
                              │
              ┌───────────────┼───────────────┐
              ▼               ▼               ▼
        ┌──────────┐   ┌──────────┐   ┌──────────┐
        │ Game-1   │   │ Game-2   │   │ Game-N   │
        │(SpinData)│   │(SpinData)│   │(SpinData)│
        └──────────┘   └──────────┘   └──────────┘
                              │
                              ▼
                    ┌──────────────────┐
                    │    PostgreSQL    │
                    └──────────────────┘
```

## Components and Interfaces

### 1. PlayerLocation (Center节点 - 内存+DB)

```go
// PlayerLocation 玩家位置信息
type PlayerLocation struct {
    UserId    int64  `json:"player_id" gorm:"primaryKey"`
    GateNodeId  string `json:"gate_node_id"`
    GameNodeId  string `json:"game_node_id"`
    LoginTime   int64  `json:"login_time"`
    Status      int32  `json:"status"` // 1=online, 0=offline
}

// PlayerLocationManager 玩家位置管理器
type PlayerLocationManager struct {
    cache map[int64]*PlayerLocation // 内存缓存
    mu    sync.RWMutex
}

// 核心方法
func (m *PlayerLocationManager) AllocateNodes(playerId int64) (*PlayerLocation, error)
func (m *PlayerLocationManager) GetLocation(playerId int64) (*PlayerLocation, bool)
func (m *PlayerLocationManager) RemoveLocation(playerId int64) error
```

### 2. NodeOnlineCounter (Center节点 - 简单负载均衡)

```go
// NodeOnlineCounter 节点在线人数统计
type NodeOnlineCounter struct {
    counts map[string]int32 // nodeId -> onlineCount
    mu     sync.RWMutex
}

// 核心方法
func (c *NodeOnlineCounter) Increment(nodeId string)
func (c *NodeOnlineCounter) Decrement(nodeId string)
func (c *NodeOnlineCounter) GetLeastLoadedNode(nodeType string, nodes []string) string
```

### 3. 路由修改 (Gate节点)

修改现有的 `route.go`，在路由游戏请求时：
1. 从session获取playerId
2. 调用Center获取PlayerLocation
3. 路由到对应的GameNodeId

### 4. 节点健康检测 (Center节点)

```go
// NodeHealthChecker 节点健康检测
type NodeHealthChecker struct {
    nodeStatus map[string]int64 // nodeId -> lastHeartbeat
    mu         sync.RWMutex
    timeout    int64 // 心跳超时时间（秒）
}

// 核心方法
func (c *NodeHealthChecker) UpdateHeartbeat(nodeId string)
func (c *NodeHealthChecker) IsHealthy(nodeId string) bool
func (c *NodeHealthChecker) GetUnhealthyNodes() []string
func (c *NodeHealthChecker) StartHealthCheck() // 定时检查，触发迁移
```

### 5. 故障迁移 (Center节点)

当检测到Game节点故障时：
1. 获取该节点上的所有PlayerLocation
2. 为每个玩家重新分配健康的Game节点
3. 更新PlayerLocation记录
4. 下次玩家请求时会路由到新节点，从DB加载状态

## Data Models

### PlayerLocation 表结构

```sql
CREATE TABLE player_location (
    player_id    BIGINT PRIMARY KEY,
    gate_node_id VARCHAR(64) NOT NULL,
    game_node_id VARCHAR(64) NOT NULL,
    login_time   BIGINT NOT NULL,
    status       INT DEFAULT 1,
    created_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at   TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_player_location_game ON player_location(game_node_id);
```

### 配置文件修改

**areaConfig.json** - 支持多Gate:
```json
[
  {"areaId": 1, "areaName": "1区", "gate": "127.0.0.1:10010"},
  {"areaId": 2, "areaName": "2区", "gate": "127.0.0.1:10011"}
]
```

**demo-cluster.json** - 多Game节点:
```json
"game": [
  {"node_id": "10001", "enable": true},
  {"node_id": "10002", "enable": true}
]
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Load Balancing - Least Loaded Selection
*For any* set of nodes with different online counts, GetLeastLoadedNode SHALL return the node with the minimum online count.
**Validates: Requirements 1.1, 1.2, 1.3**

### Property 2: PlayerLocation Completeness
*For any* player allocation, the PlayerLocation SHALL contain non-empty playerId, gateNodeId, gameNodeId, and valid loginTime > 0.
**Validates: Requirements 1.4, 1.5**

### Property 3: Routing Consistency
*For any* player with an existing PlayerLocation, GetLocation SHALL return the same gameNodeId that was allocated.
**Validates: Requirements 2.2, 2.3**

### Property 4: Reconnection Preserves Location
*For any* player with status=online, calling GetLocation SHALL return the existing location without modification.
**Validates: Requirements 3.1, 3.2, 3.3**

### Property 5: Node Failure Migration
*For any* unhealthy node, all PlayerLocations with that gameNodeId SHALL be updated to a healthy node.
**Validates: Requirements 4.2, 4.3, 4.4**

### Property 6: PlayerLocation Round-Trip Serialization
*For any* valid PlayerLocation object, serializing to JSON and then deserializing SHALL produce an equivalent object.
**Validates: Requirements 6.1, 6.2, 6.3**

## Error Handling

### 1. 节点分配失败
- 如果没有可用的Game节点，返回错误码 `NoAvailableGame`
- 客户端收到错误后应提示"服务器繁忙，请稍后重试"

### 2. 路由失败
- 如果PlayerLocation不存在，触发重新分配
- 如果目标Game节点不可达，触发故障迁移

### 3. 节点故障
- 心跳超时10秒后标记为unhealthy
- 触发该节点上所有玩家的迁移

## Testing Strategy

### 单元测试
- 测试负载均衡算法的正确性
- 测试PlayerLocation的CRUD操作
- 测试故障迁移逻辑

### 属性测试 (Property-Based Testing)
使用 `github.com/leanovate/gopter` 库进行属性测试：

1. **负载均衡属性测试**: 生成随机节点在线人数，验证选择最少人数节点
2. **序列化属性测试**: 生成随机PlayerLocation，验证round-trip一致性
3. **故障迁移属性测试**: 验证故障节点上的玩家都被迁移到健康节点

每个属性测试配置运行100次迭代。

## 实现流程

### 登录流程
```
1. Client -> Gate: 连接
2. Gate -> Center: AllocateNodes(playerId)
3. Center: 
   - 检查是否已有PlayerLocation（断线重连）
   - 如果没有，选择在线人数最少的Game节点
   - 保存PlayerLocation到内存和DB
   - 返回 {gateNodeId, gameNodeId}
4. Gate: 保存gameNodeId到session
5. 后续请求路由到对应的Game节点
```

### 断线重连流程
```
1. Client -> Gate: 重新连接
2. Gate -> Center: GetLocation(playerId)
3. Center: 返回已存在的PlayerLocation
4. Gate: 路由到原来的Game节点
5. Game: 从内存缓存恢复SpinData
```

### 登出流程
```
1. Gate -> Center: RemoveLocation(playerId)
2. Center: 
   - 从内存删除PlayerLocation
   - 更新DB状态为offline
   - 减少对应Game节点的在线人数
3. Game: 持久化SpinData，清理内存缓存
```

### 故障迁移流程
```
1. Center: 定时检查节点心跳
2. 发现节点心跳超时 -> 标记为unhealthy
3. 获取该节点上的所有PlayerLocation
4. 为每个玩家分配新的健康Game节点
5. 更新PlayerLocation记录
6. 玩家下次请求时路由到新节点，从DB加载状态
```
