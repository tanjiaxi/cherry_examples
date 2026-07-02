# Implementation Plan: Actor 异步持久化系统

## Overview

本实现计划将分阶段构建 Actor 异步持久化系统,从核心接口和数据结构开始,逐步实现中间件、队列、存储层、恢复管理器和监控指标,最终完成与现有 Cherry Actor 框架的集成。实现采用 Go 语言,利用 Cherry 框架的 Actor 模型特性,通过中间件模式实现低侵入性集成。

## Tasks

- [ ] 1. 建立项目结构和核心接口定义
  - 创建 `persistence` 包目录结构
  - 定义 `PersistenceMarker` 接口 (GetPersistenceKey, SerializeState, DeserializeState, GetSchemaVersion)
  - 定义数据结构类型: `DeltaUpdate`, `JSONMetadata`, `RedisValue`
  - 定义配置结构: `PersistenceConfig`, `RedisConfig`
  - 定义错误类型: `ErrStateNotFound`, `ErrSerializationFailed`, `ErrVersionMismatch`
  - _Requirements: 2.1, 2.2, 5.1, 5.3, 5.4, 5.5, 10.1, 12.1_

- [ ] 2. 实现 Redis 存储层 (RedisStore)
  - [ ] 2.1 实现 Redis 客户端初始化和连接池管理
    - 使用 go-redis/redis 库初始化客户端
    - 实现连接池配置 (PoolSize, MaxRetries)
    - 实现 Redis 健康检查方法
    - _Requirements: 10.1_

  - [ ] 2.2 实现混合格式序列化与反序列化
    - 实现 `marshalRedisValue()` 将 DeltaUpdate 转换为 JSON+Protobuf 混合格式
    - 实现 `unmarshalRedisValue()` 解析 Redis 存储的混合格式数据
    - 验证 JSON 元数据包含 actorType, actorId, schemaVersion, updatedAt, stateSize
    - _Requirements: 2.1, 2.2, 2.3, 2.5_

  - [ ] 2.3 实现批量写入方法 (BatchWrite)
    - 使用 Redis Pipeline 批量执行 SET 命令
    - 支持单批最多 1000 条记录,超出则分批
    - 实现 TTL 设置 (默认 30 天)
    - 实现键格式 `actor:{ActorType}:{ActorID}`
    - _Requirements: 1.3, 2.4, 5.7, 6.3, 6.4_

  - [ ] 2.4 实现单条加载方法 (Load)
    - 实现 Redis GET 操作封装
    - 处理键不存在情况 (返回 ErrStateNotFound)
    - 解析 JSON 元数据并验证版本兼容性
    - _Requirements: 2.5, 3.1, 3.2_

  - [ ]* 2.5 编写 RedisStore 单元测试
    - 测试连接初始化和健康检查
    - 测试混合格式序列化往返一致性
    - 测试批量写入和 Pipeline 分批逻辑
    - 测试键格式生成和 TTL 设置
    - 使用 Mock Redis (miniredis) 进行测试

- [ ] 3. 实现持久化队列 (PersistenceQueue)
  - [ ] 3.1 实现队列核心数据结构
    - 实现 `actorQueue` 结构 (带缓冲 channel + pending 列表)
    - 实现 `PersistenceQueue` 结构 (维护 map[actorID]*actorQueue)
    - 使用 sync.RWMutex 保护队列 map
    - _Requirements: 4.2_

  - [ ] 3.2 实现 Enqueue 方法
    - 序列化 Actor 状态为 DeltaUpdate
    - 获取或创建对应 Actor 的队列
    - 推送 DeltaUpdate 到 channel (非阻塞)
    - 合并同周期内的多次变更
    - _Requirements: 1.1, 4.3_

  - [ ] 3.3 实现定时刷新机制 (StartFlushTimer & FlushAll)
    - 启动 50ms 间隔的定时器
    - 实现 `collectAllPending()` 收集所有队列的待刷新数据
    - 调用 RedisStore.BatchWrite 批量写入
    - 实现优雅关闭机制 (stopCh)
    - _Requirements: 1.3, 6.1, 6.2_

  - [ ] 3.4 实现最终持久化方法 (EnqueueFinalSnapshot)
    - 强制执行完整状态快照
    - 跳过增量更新优化
    - 立即刷新,不等待定时器
    - _Requirements: 5.6_

  - [ ]* 3.5 编写 PersistenceQueue 单元测试
    - 测试队列独立性和并发安全
    - 测试同周期变更合并逻辑
    - 测试定时器触发和批量收集
    - 测试最终持久化和优雅关闭

- [ ] 4. 实现恢复管理器 (RecoveryManager)
  - [ ] 4.1 实现状态恢复方法 (RecoverState)
    - 调用 RedisStore.Load 查询状态快照
    - 处理状态不存在情况 (允许默认初始化)
    - 检查 Schema 版本兼容性
    - 调用 Actor 的 DeserializeState 恢复状态
    - 实现 200ms 超时控制
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [ ] 4.2 实现版本迁移机制
    - 定义 `MigrationFunc` 函数类型
    - 实现 `RegisterMigration()` 注册迁移函数
    - 实现 `migrateData()` 执行逐版本迁移
    - 处理迁移失败情况 (拒绝恢复)
    - _Requirements: 12.1, 12.2, 12.3, 12.4, 12.5_

  - [ ] 4.3 实现批量预加载方法 (BatchPreload)
    - 使用 Redis MGET 批量读取多个键
    - 返回 map[key]*RedisValue
    - 支持可选的预加载 Actor 列表
    - _Requirements: 3.6_

  - [ ]* 4.4 编写 RecoveryManager 单元测试
    - 测试状态存在和不存在两种场景
    - 测试版本兼容性检查和迁移逻辑
    - 测试恢复失败容错机制
    - 测试批量预加载功能

- [ ] 5. 实现持久化中间件 (PersistenceMiddleware)
  - [ ] 5.1 实现中间件核心结构
    - 定义 `PersistenceMiddleware` 结构
    - 持有 PersistenceQueue, RecoveryManager, Config, Metrics 引用
    - 实现初始化方法 `NewMiddleware()`
    - _Requirements: 11.1_

  - [ ] 5.2 实现 OnActorInit 钩子
    - 检查 Actor 是否实现 PersistenceMarker (类型断言)
    - 调用 RecoveryManager.RecoverState 执行懒加载
    - 记录恢复成功/失败日志
    - _Requirements: 3.1, 11.4_

  - [ ] 5.3 实现 OnMessageProcessed 钩子
    - 检查 Actor 是否实现 PersistenceMarker
    - 检查状态是否变更 (脏标记机制)
    - 调用 PersistenceQueue.Enqueue 推送持久化请求
    - 保证在 Actor 线程内执行,不阻塞消息处理
    - _Requirements: 1.1, 1.2, 4.1, 11.2_

  - [ ] 5.4 实现 OnActorStop 钩子
    - 调用 PersistenceQueue.EnqueueFinalSnapshot 执行最终持久化
    - 等待最终持久化完成 (带超时保护)
    - 记录最终持久化结果
    - _Requirements: 5.6_

  - [ ] 5.5 实现脏标记机制 (isStateDirty)
    - 通过 Actor 状态字段的脏标记位判断
    - 支持配置是否启用脏标记优化
    - _Requirements: 4.1, 7.1_

  - [ ]* 5.6 编写 PersistenceMiddleware 单元测试
    - 测试中间件钩子注册和触发
    - 测试 Actor 类型检测 (PersistenceMarker)
    - 测试懒加载恢复流程
    - 测试脏标记检测和持久化触发

- [ ] 6. 实现监控指标收集器 (MetricsCollector)
  - [ ] 6.1 实现指标数据结构和收集方法
    - 定义 `MetricsCollector` 结构 (包含计数器和延迟采样)
    - 实现 `RecordEnqueue()`, `RecordFlushSuccess()`, `RecordFlushError()`
    - 实现 `RecordRetry()`, `RecordRecoverySuccess()`, `RecordRecoveryError()`
    - 使用 atomic.Int64 保证并发安全
    - _Requirements: 6.5, 8.5, 9.2, 9.4_

  - [ ] 6.2 实现延迟百分位计算
    - 实现 `RecordPersistenceLatency()` 记录延迟采样
    - 实现 `GetLatencyPercentiles()` 计算 P50, P95, P99
    - 使用滑动窗口限制采样数量 (避免内存泄漏)
    - _Requirements: 9.1_

  - [ ] 6.3 实现 Prometheus 指标暴露
    - 实现 `GetMetricsSnapshot()` 返回指标快照
    - 实现 `ExposeHTTPMetrics()` 启动 HTTP 服务器
    - 提供 `/metrics` 端点返回 Prometheus 格式
    - 暴露指标: enqueue_total, flush_success_total, flush_error_total, retry_total, latency_pXX_ms, queue_length
    - _Requirements: 9.3, 9.5_

  - [ ]* 6.4 编写 MetricsCollector 单元测试
    - 测试计数器递增和原子性
    - 测试延迟百分位计算准确性
    - 测试 HTTP 端点和 Prometheus 格式输出

- [ ] 7. 实现增量更新优化
  - [ ] 7.1 实现脏标记数据结构
    - 在状态结构中添加 `dirtyFields map[string]bool`
    - 实现 `MarkDirty(fieldName)` 标记字段变更
    - 实现 `ClearDirty()` 清空脏标记
    - _Requirements: 7.1, 7.2_

  - [ ] 7.2 实现增量序列化逻辑
    - 在 SerializeState 中检查脏字段比例
    - 若 < 50% 变更,仅序列化脏字段 (Delta)
    - 若 >= 50% 变更,序列化完整状态 (Snapshot)
    - 在 DeltaUpdate 中标记 IsFull 字段
    - _Requirements: 7.3, 7.4, 7.5_

  - [ ]* 7.3 编写增量更新单元测试
    - 测试脏标记设置和清空
    - 测试增量序列化 (< 50% 变更)
    - 测试完整快照 (>= 50% 变更)
    - 测试字段变更跟踪准确性

- [ ] 8. 实现错误处理与重试机制
  - [ ] 8.1 实现指数退避重试逻辑
    - 在 RedisStore.handleWriteError 中实现重试
    - 使用指数退避: 100ms, 200ms, 400ms
    - 最大重试 3 次
    - 记录每次重试到日志
    - _Requirements: 1.4, 8.1_

  - [ ] 8.2 实现失败日志记录
    - 创建失败日志文件 `/var/log/persistence/failed_updates.log`
    - 使用 JSON Lines 格式记录失败的 DeltaUpdate
    - 包含字段: timestamp, actorId, actorType, key, error, retries, stateSize
    - 实现日志轮转 (防止文件过大)
    - _Requirements: 8.2_

  - [ ] 8.3 实现错误类型判断
    - 区分可重试错误 (连接失败, 超时) 和不可重试错误 (权限错误, 数据错误)
    - 根据错误类型选择重试或跳过
    - _Requirements: 8.4_

  - [ ] 8.4 实现手动重放工具脚本
    - 编写 `replay_failed_persistence.sh` 读取失败日志
    - 解析 JSON Lines 并重新执行持久化
    - 支持时间范围过滤 (--from, --to)
    - _Requirements: 8.3_

  - [ ]* 8.5 编写错误处理单元测试
    - 测试指数退避计算
    - 测试重试次数限制
    - 测试失败日志写入
    - 测试错误类型判断逻辑

- [ ] 9. Checkpoint - 核心功能验证
  - 启动测试 Redis 实例 (使用 Docker 或 miniredis)
  - 运行所有核心组件单元测试,确保通过
  - 验证 RedisStore 批量写入和加载功能
  - 验证 PersistenceQueue 定时刷新机制
  - 验证 RecoveryManager 懒加载和版本迁移
  - 记录任何发现的问题,及时修复

- [ ] 10. 集成 Cherry Actor 框架
  - [ ] 10.1 定义 Protobuf 消息结构
    - 在 `internal/protocol/cherry_rpc.proto` 添加持久化相关消息
    - 定义 `PlayerState` 消息 (userId, money, diamond, level, exp, ...)
    - 定义 `SpinState` 消息 (roomId, level, totalBet, totalWin, spinCount, ...)
    - 运行 `build_proto.sh` 生成 Go 代码
    - _Requirements: 2.3, 5.3, 5.4, 5.5_

  - [ ] 10.2 改造 actorPlayer 实现 PersistenceMarker
    - 在 `demo_cluster/nodes/game/module/player/actor_player.go` 实现接口
    - 实现 GetPersistenceKey() 返回 `actor:player:{userId}`
    - 实现 SerializeState() 序列化 playerData 为 Protobuf
    - 实现 DeserializeState() 从 Protobuf 恢复 playerData
    - 实现 GetSchemaVersion() 返回版本号 1
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 5.5, 11.3, 11.4, 11.5_

  - [ ] 10.3 改造 actorSpinData 实现 PersistenceMarker
    - 在 `demo_cluster/nodes/center/module/spin/actor_spin_data.go` 实现接口
    - 实现 GetPersistenceKey() 返回 `actor:spin:{roomId}`
    - 实现 SerializeState() 序列化 spinData 为 Protobuf
    - 实现 DeserializeState() 从 Protobuf 恢复 spinData
    - 实现 GetSchemaVersion() 返回版本号 1
    - _Requirements: 5.1, 5.2, 11.5_

  - [ ] 10.4 注册持久化中间件到 Actor System
    - 在 `demo_cluster/nodes/game/game.go` 和 `nodes/center/center.go` 初始化中间件
    - 加载 `config/persistence.json` 配置文件
    - 调用 `actorSystem.RegisterMiddleware(persistenceMiddleware)`
    - 验证中间件钩子正确触发
    - _Requirements: 11.1, 11.2_

  - [ ]* 10.5 编写集成测试
    - 测试完整的持久化与恢复流程 (Player Actor)
    - 测试 Actor 销毁时的最终持久化
    - 测试服务器重启后的懒加载恢复
    - 测试批量 Actor 并发持久化场景

- [ ] 11. 实现配置管理
  - [ ] 11.1 创建配置文件 persistence.json
    - 定义 JSON 配置结构,包含所有可配置参数
    - 设置默认值: flushIntervalMs=50, batchSize=1000, delayThresholdMs=100
    - 配置 Redis 连接信息: address, password, db, maxRetries, ttlDays, poolSize
    - 配置功能开关: enableDeltaUpdate, enableLazyLoad, enableBatchFlush
    - _Requirements: 10.1, 10.2, 10.3_

  - [ ] 11.2 实现配置加载和验证
    - 实现 `LoadConfig()` 从文件读取 JSON 配置
    - 实现 `ValidateConfig()` 验证参数合法性
    - 验证规则: flushIntervalMs >= 10, batchSize 1-10000, maxRetries >= 0
    - 记录配置加载结果和验证错误
    - _Requirements: 10.5_

  - [ ] 11.3 实现配置热重载 (可选)
    - 监听配置文件变更 (使用 fsnotify)
    - 重新加载和验证配置
    - 动态更新支持热重载的参数 (如 flushIntervalMs)
    - 记录热重载事件
    - _Requirements: 10.4_

  - [ ]* 11.4 编写配置管理单元测试
    - 测试配置文件解析
    - 测试参数验证逻辑
    - 测试默认值设置
    - 测试热重载机制

- [ ] 12. 实现测试工具和 Mock
  - [ ] 12.1 实现 MockRedisStore
    - 使用内存 map 模拟 Redis 存储
    - 实现 BatchWrite, Load, Delete 方法
    - 支持故障注入 (FaultyRedisStore)
    - _Requirements: 13.1, 13.3_

  - [ ] 12.2 实现测试辅助函数
    - 实现 `AssertStatePersisted()` 验证状态是否持久化
    - 实现 `SimulateRedisFailure()` 模拟 Redis 故障
    - 实现 `CreateTestActor()` 创建测试用 Actor
    - _Requirements: 13.2, 13.3_

  - [ ]* 12.3 编写性能测试
    - 测试吞吐量 (ops/s)
    - 测试延迟百分位 (P50, P95, P99)
    - 测试不同负载下的队列积压情况
    - 目标: 吞吐量 > 10,000 ops/s, P99 < 100ms
    - _Requirements: 13.4_

  - [ ]* 12.4 编写故障注入测试
    - 测试 Redis 连接失败场景
    - 测试 Redis 超时场景
    - 测试重试耗尽和失败日志记录
    - 验证最终成功率 > 95%
    - _Requirements: 13.3_

- [ ] 13. Checkpoint - 集成测试验证
  - 启动完整的 demo_cluster 游戏服务器
  - 创建多个 Player Actor 并触发状态变更
  - 验证状态在 100ms 内持久化到 Redis
  - 重启服务器,验证懒加载恢复成功
  - 使用 Redis CLI 检查存储格式 (JSON + Protobuf)
  - 检查 `/metrics` 端点,验证监控指标正常暴露
  - 记录性能指标 (延迟、吞吐量、队列长度)

- [ ] 14. 编写文档和示例
  - [ ] 14.1 编写 README.md
    - 系统概述和核心特性
    - 快速开始指南
    - 配置参数说明
    - 常见问题 FAQ
    - _Requirements: 11.5_

  - [ ] 14.2 编写使用示例代码
    - 示例 1: 改造 Actor 实现 PersistenceMarker
    - 示例 2: 注册持久化中间件到 Actor System
    - 示例 3: 配置和启动持久化系统
    - 示例 4: 版本迁移函数注册
    - _Requirements: 11.5_

  - [ ] 14.3 编写运维指南
    - Redis 部署建议
    - 监控告警配置 (Prometheus 规则)
    - 日志配置和轮转
    - 容量规划和性能调优
    - 灾难恢复流程

  - [ ] 14.4 编写 API 文档
    - PersistenceMarker 接口文档
    - PersistenceMiddleware 方法文档
    - 配置参数详细说明
    - 监控指标说明
    - 使用 godoc 生成文档

- [ ] 15. 最终集成和验证
  - 运行完整的单元测试套件 (go test -v ./persistence/...)
  - 运行集成测试 (包含真实 Redis)
  - 运行性能测试并记录基准数据
  - 验证所有 41 个 Correctness Properties
  - 代码审查和优化
  - 更新版本号和 CHANGELOG

## Notes

- 标记 `*` 的任务为可选测试任务,可以跳过以加快 MVP 交付
- 每个任务都引用了对应的需求编号,便于追溯
- Checkpoint 任务用于阶段性验证,确保增量进展
- 所有核心实现任务都需要完成,测试任务可根据时间灵活调整
- 使用 Go 1.23+ 和 Cherry 框架现有依赖,无需引入新的外部库 (除了 go-redis)
- 代码风格遵循 Cherry 框架现有约定,使用 `clog` 进行日志记录
- 持久化系统作为独立包 `persistence` 实现,便于后续迁移到 Cherry 框架核心

## Task Dependency Graph

```json
{
  "waves": [
    {
      "id": 0,
      "tasks": ["1.1"]
    },
    {
      "id": 1,
      "tasks": ["2.1", "11.1"]
    },
    {
      "id": 2,
      "tasks": ["2.2", "3.1", "11.2", "12.1"]
    },
    {
      "id": 3,
      "tasks": ["2.3", "3.2", "4.1", "6.1", "7.1", "8.1", "11.3", "12.2"]
    },
    {
      "id": 4,
      "tasks": ["2.4", "2.5", "3.3", "4.2", "6.2", "7.2", "8.2"]
    },
    {
      "id": 5,
      "tasks": ["3.4", "3.5", "4.3", "4.4", "6.3", "7.3", "8.3"]
    },
    {
      "id": 6,
      "tasks": ["5.1", "6.4", "8.4", "8.5", "11.4"]
    },
    {
      "id": 7,
      "tasks": ["5.2", "5.3", "5.4", "5.5", "10.1"]
    },
    {
      "id": 8,
      "tasks": ["5.6", "10.2", "10.3"]
    },
    {
      "id": 9,
      "tasks": ["9.1", "10.4"]
    },
    {
      "id": 10,
      "tasks": ["10.5", "12.3", "12.4"]
    },
    {
      "id": 11,
      "tasks": ["13.1"]
    },
    {
      "id": 12,
      "tasks": ["14.1", "14.2", "14.3", "14.4"]
    },
    {
      "id": 13,
      "tasks": ["15.1"]
    }
  ]
}
```
