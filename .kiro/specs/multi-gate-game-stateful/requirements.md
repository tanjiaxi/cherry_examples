<!--
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-19 17:58:09
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-19 17:58:39
 * @FilePath: /examples/.kiro/specs/multi-gate-game-stateful/requirements.md
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
-->
# Requirements Document

## Introduction

本功能实现多Gate、多Game节点的有状态服务架构。玩家登录时通过负载均衡分配Gate和Game节点，玩家的游戏状态（如SpinData）绑定到特定的Game节点。支持断线重连时路由到原Game节点，以及节点故障时的状态迁移。

## Glossary

- **Gate节点**: 网关节点，负责客户端连接管理和消息转发
- **Game节点**: 游戏逻辑节点，处理玩家游戏状态（如SpinData）
- **Center节点**: 中心节点，负责玩家位置注册、节点分配和全局协调
- **PlayerLocation**: 玩家位置信息，记录玩家当前所在的Gate和Game节点
- **负载均衡**: 根据节点负载情况分配玩家到合适的节点
- **有状态服务**: 玩家游戏数据（SpinData等）缓存在特定Game节点内存中
- **断线重连**: 玩家断线后重新连接，路由到原来的Game节点恢复状态

## Requirements

### Requirement 1

**User Story:** As a player, I want to be assigned to an appropriate Gate and Game node when logging in, so that I can have a smooth gaming experience with balanced server load.

#### Acceptance Criteria

1. WHEN a player initiates login THEN the Center node SHALL query available Gate nodes and return the least loaded Gate address
2. WHEN a player completes Gate authentication THEN the Center node SHALL allocate a Game node using load balancing algorithm
3. WHEN allocating a Game node THEN the system SHALL consider node online player count and CPU/memory metrics
4. WHEN a player is allocated to nodes THEN the Center node SHALL persist the PlayerLocation record with playerId, gateNodeId, gameNodeId, and loginTime
5. WHEN the PlayerLocation is persisted THEN the system SHALL store it in both memory cache and database for durability

### Requirement 2

**User Story:** As a player, I want my game state to be maintained on a specific Game node, so that my SpinData and other progress are preserved during my session.

#### Acceptance Criteria

1. WHEN a player enters a game room THEN the Game node SHALL load and cache the player's SpinData in memory
2. WHEN a player performs a spin operation THEN the system SHALL route the request to the player's assigned Game node
3. WHEN routing game requests THEN the Gate node SHALL query PlayerLocation from Center to determine the target Game node
4. WHEN SpinData is modified THEN the Game node SHALL mark it dirty and persist to database at appropriate intervals
5. WHEN a player logs out normally THEN the Game node SHALL persist all dirty data and clear the memory cache

### Requirement 3

**User Story:** As a player, I want to reconnect to my previous Game node after a disconnection, so that I can continue my game without losing progress.

#### Acceptance Criteria

1. WHEN a player reconnects within the session timeout THEN the Center node SHALL return the existing PlayerLocation
2. WHEN reconnecting THEN the Gate node SHALL route the player to the original Game node
3. WHEN the original Game node receives a reconnection THEN the system SHALL restore the player's cached state
4. WHEN the session timeout expires THEN the Center node SHALL clear the PlayerLocation and allow fresh allocation
5. WHEN a player reconnects after timeout THEN the system SHALL allocate new nodes and load state from database

### Requirement 4

**User Story:** As a system administrator, I want the system to handle Game node failures gracefully, so that players can continue playing with minimal disruption.

#### Acceptance Criteria

1. WHEN a Game node becomes unavailable THEN the Center node SHALL detect it within 10 seconds via heartbeat monitoring
2. WHEN a Game node failure is detected THEN the Center node SHALL mark affected PlayerLocations as requiring migration
3. WHEN a player on a failed node sends a request THEN the system SHALL allocate a new Game node and load state from database
4. WHEN migrating a player THEN the system SHALL update the PlayerLocation record with the new Game node
5. WHEN a Game node recovers THEN the system SHALL not automatically migrate players back to avoid disruption

### Requirement 5

**User Story:** As a developer, I want clear configuration for multi-Gate and multi-Game deployment, so that I can easily scale the system.

#### Acceptance Criteria

1. WHEN configuring the cluster THEN the system SHALL support multiple Gate nodes with distinct addresses in areaConfig
2. WHEN configuring Game nodes THEN the system SHALL support dynamic node registration via etcd discovery
3. WHEN a new Gate or Game node starts THEN the system SHALL automatically register it with the Center node
4. WHEN a node shuts down gracefully THEN the system SHALL deregister it and migrate affected players
5. WHEN querying available nodes THEN the Center node SHALL filter out disabled or unhealthy nodes

### Requirement 6

**User Story:** As a developer, I want the PlayerLocation data to be serialized and deserialized correctly, so that player routing information is reliably stored and retrieved.

#### Acceptance Criteria

1. WHEN serializing PlayerLocation THEN the system SHALL encode it to JSON format for database storage
2. WHEN deserializing PlayerLocation THEN the system SHALL parse JSON and reconstruct the PlayerLocation struct
3. WHEN serializing and then deserializing PlayerLocation THEN the system SHALL produce an equivalent object (round-trip consistency)
