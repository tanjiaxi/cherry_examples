# Implementation Plan

## 1. 创建PlayerLocation数据模型和管理器

- [x] 1.1 创建PlayerLocation数据模型
  - 在 `demo_cluster/internal/model/` 创建 `player_location.go`
  - 定义PlayerLocation结构体（UserId, GateNodeId, GameNodeId, LoginTime, Status）
  - 添加GORM标签用于数据库映射
  - _Requirements: 1.4, 1.5_

- [ ]* 1.2 编写Property测试 - PlayerLocation序列化round-trip
  - **Property 6: PlayerLocation Round-Trip Serialization**
  - **Validates: Requirements 6.1, 6.2, 6.3**

- [x] 1.3 创建PlayerLocationManager
  - 在 `demo_cluster/nodes/center/server/` 创建 `player_location_manager.go`
  - 实现内存缓存 map[int64]*PlayerLocation
  - 实现 AllocateNodes, GetLocation, UpdateLocation, RemoveLocation 方法
  - _Requirements: 1.4, 1.5, 2.2, 3.1_

- [ ]* 1.4 编写Property测试 - 位置完整性
  - **Property 2: PlayerLocation Completeness**
  - **Validates: Requirements 1.4, 1.5**

- [ ]* 1.5 编写Property测试 - 路由一致性
  - **Property 3: Routing Consistency**
  - **Validates: Requirements 2.2, 2.3**

- [ ]* 1.6 编写Property测试 - 重连保持
  - **Property 4: Reconnection Preserves Location**
  - **Validates: Requirements 3.1, 3.2, 3.3**

## 2. 实现负载均衡

- [x] 2.1 创建NodeOnlineCounter
  - 在 `demo_cluster/nodes/center/server/` 创建 `node_online_counter.go`
  - 实现在线人数统计 map[string]int32
  - 实现 Increment, Decrement, GetLeastLoadedNode 方法
  - _Requirements: 1.1, 1.2, 1.3_

- [ ]* 2.2 编写Property测试 - 负载均衡选择最少人数节点
  - **Property 1: Load Balancing - Least Loaded Selection**
  - **Validates: Requirements 1.1, 1.2, 1.3**

## 3. 实现节点健康检测和故障迁移

- [x] 3.1 创建NodeHealthChecker
  - 在 `demo_cluster/nodes/center/server/` 创建 `node_health_checker.go`
  - 实现心跳记录 map[string]int64
  - 实现 UpdateHeartbeat, IsHealthy, GetUnhealthyNodes 方法
  - _Requirements: 4.1_

- [x] 3.2 实现故障迁移逻辑
  - 在PlayerLocationManager中添加 MigratePlayersFromNode 方法
  - 获取故障节点上的所有玩家
  - 为每个玩家分配新的健康节点
  - 更新PlayerLocation记录
  - _Requirements: 4.2, 4.3, 4.4_

- [ ]* 3.3 编写Property测试 - 故障迁移
  - **Property 5: Node Failure Migration**
  - **Validates: Requirements 4.2, 4.3, 4.4**

- [x] 3.4 启动定时健康检查
  - 在Center节点启动时启动健康检查goroutine
  - 每5秒检查一次节点心跳
  - 发现故障节点时触发迁移
  - _Requirements: 4.1_

## 4. Checkpoint - 确保所有测试通过
- Ensure all tests pass, ask the user if questions arise.

## 5. 集成到Center节点

- [x] 5.1 创建Center节点的Location Actor
  - 在 `demo_cluster/nodes/center/module/` 创建 `location/actor_location.go`
  - 注册Remote方法: allocateNodes, getLocation, removeLocation, heartbeat
  - _Requirements: 1.4, 2.2, 3.1_

- [x] 5.2 在Center启动时初始化组件
  - 修改 `demo_cluster/nodes/center/center.go`
  - 初始化PlayerLocationManager, NodeOnlineCounter, NodeHealthChecker
  - 注册ActorLocation
  - _Requirements: 5.3_

## 6. 修改Gate节点路由

- [x] 6.1 创建RPC调用Center的方法
  - 在 `demo_cluster/internal/rpc/center/` 添加 location相关RPC方法
  - AllocateNodes, GetLocation, RemoveLocation
  - _Requirements: 2.3_

- [x] 6.2 修改Gate登录流程
  - 修改 `demo_cluster/nodes/gate/actor_agent.go` 的login方法
  - 登录成功后调用Center分配节点
  - 保存gameNodeId到session
  - _Requirements: 1.1, 1.2_

- [x] 6.3 修改Gate路由逻辑
  - 修改 `demo_cluster/nodes/gate/route.go`
  - 游戏请求路由时使用session中的gameNodeId
  - 节点离线时调用Center重新分配节点
  - _Requirements: 2.2, 2.3_

- [x] 6.4 修改Gate断开连接处理
  - 修改 `demo_cluster/nodes/gate/actor_agent.go` 的onSessionClose方法
  - 调用Center的RemoveLocation
  - _Requirements: 2.5_

## 7. 修改Game节点

- [x] 7.1 添加Game节点心跳上报
  - 修改 `demo_cluster/nodes/game/game.go`
  - 启动时向Center注册，定时发送心跳
  - _Requirements: 4.1, 5.3_

- [ ] 7.2 修改Game节点状态恢复
  - 确保玩家重连时能从内存或DB恢复SpinData
  - _Requirements: 3.3_

## 8. 修改Web节点serverList（方案A：Web节点负载均衡）

- [x] 8.1 修改serverList方法
  - 修改 `demo_cluster/nodes/web/controller/controller.go`
  - serverList调用Center获取最优Gate地址
  - 返回负载均衡后的Gate地址给客户端
  - _Requirements: 1.1_

- [x] 8.2 添加RPC方法获取最优Gate
  - 在 `demo_cluster/internal/rpc/center/` 添加 GetBestGate 方法
  - Center根据在线人数返回最优Gate
  - _Requirements: 1.1, 1.2_

## 9. 更新配置文件

- [x] 9.1 更新areaConfig.json支持多Gate
  - 修改 `config/data/areaConfig.json`
  - 配置多个Gate地址列表
  - _Requirements: 5.1_

- [ ] 9.2 更新demo-cluster.json支持多Game
  - 确保多个Game节点配置正确
  - _Requirements: 5.2_

## 10. Final Checkpoint - 确保所有测试通过
- Ensure all tests pass, ask the user if questions arise.
