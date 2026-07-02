# Requirements Document

## Introduction

本文档定义了基于 Cherry Actor 框架的异步持久化系统需求。该系统为游戏服务器中的 Actor 实例提供准实时、高性能的状态持久化能力,通过异步机制将内存状态同步到 Redis,在服务器重启或崩溃后支持懒加载恢复。系统采用 JSON + Protobuf bytes 混合协议,利用 Actor 模型的单线程特性自然避免并发冲突,实现低延迟(<100ms)的数据持久化。

## Glossary

- **Persistence_System**: 异步持久化系统,负责将 Actor 状态持久化到 Redis 的核心模块
- **Actor**: Cherry 框架中的并发实体,遵循单线程消息处理模型
- **Player_Actor**: 玩家 Actor,管理单个玩家的游戏状态和数据
- **Spin_Actor**: 游戏转轮 Actor,管理转轮游戏的状态数据
- **Persistence_Queue**: 持久化队列,缓冲待持久化的数据变更
- **Redis_Store**: Redis 持久化存储,实际存储 Actor 状态数据的 Redis 实例
- **State_Snapshot**: 状态快照,Actor 当前完整状态的序列化表示
- **Delta_Update**: 增量更新,仅包含变更字段的状态更新
- **Lazy_Loading**: 懒加载机制,Actor 启动时按需从 Redis 恢复状态
- **Persistence_Marker**: 持久化标记接口,标识 Actor 需要持久化支持
- **Flush_Timer**: 刷新定时器,触发批量持久化操作的定时器
- **JSON_Metadata**: JSON 元数据字段,存储 Actor 类型、版本等可读信息
- **Protobuf_State**: Protobuf 状态数据,存储 Actor 的二进制状态数据
- **Recovery_Manager**: 恢复管理器,负责 Actor 状态恢复的组件
- **Persistence_Middleware**: 持久化中间件,拦截 Actor 消息处理并触发持久化

## Requirements

### Requirement 1: 准实时异步持久化

**User Story:** 作为游戏开发者,我希望 Actor 状态变更能在 100ms 内异步持久化到 Redis,以确保数据实时性同时不阻塞游戏逻辑

#### Acceptance Criteria

1. WHEN Actor 状态发生变更, THE Persistence_System SHALL 在 100 毫秒内将变更推送到 Persistence_Queue
2. THE Persistence_System SHALL 使用异步机制执行持久化操作,不阻塞 Actor 消息处理
3. WHEN Persistence_Queue 中存在待处理数据, THE Persistence_System SHALL 在 50 毫秒内完成批量写入 Redis_Store
4. IF 持久化操作失败, THEN THE Persistence_System SHALL 记录错误日志并重试最多 3 次
5. THE Persistence_System SHALL 支持配置持久化延迟阈值,默认值为 100 毫秒

### Requirement 2: 混合协议存储格式

**User Story:** 作为运维人员,我希望持久化数据既包含人类可读的 JSON 元数据又包含高效的 Protobuf 状态数据,便于调试和性能优化

#### Acceptance Criteria

1. WHEN Actor 状态持久化, THE Persistence_System SHALL 将数据存储为包含 JSON_Metadata 和 Protobuf_State 的混合格式
2. THE JSON_Metadata SHALL 包含 Actor 类型名称、版本号、最后更新时间戳字段
3. THE Protobuf_State SHALL 存储完整的 Actor 业务状态数据
4. THE Persistence_System SHALL 在 Redis_Store 中使用键格式 `actor:{ActorType}:{ActorID}` 存储数据
5. WHEN 读取持久化数据, THE Persistence_System SHALL 先解析 JSON_Metadata 验证数据版本兼容性

### Requirement 3: 懒加载恢复策略

**User Story:** 作为系统架构师,我希望 Actor 在首次访问时按需从 Redis 恢复状态,避免启动时加载全部数据导致的性能问题

#### Acceptance Criteria

1. WHEN Actor 首次初始化, THE Recovery_Manager SHALL 检查 Redis_Store 中是否存在对应的 State_Snapshot
2. IF State_Snapshot 存在, THEN THE Recovery_Manager SHALL 从 Redis_Store 加载并反序列化 Protobuf_State
3. IF State_Snapshot 不存在, THEN THE Actor SHALL 使用默认初始状态启动
4. THE Recovery_Manager SHALL 在 200 毫秒内完成单个 Actor 状态恢复操作
5. WHEN 恢复操作失败, THE Recovery_Manager SHALL 记录错误并允许 Actor 使用默认状态启动
6. THE Persistence_System SHALL 支持批量预加载功能,可选择性预加载高频访问的 Actor 状态

### Requirement 4: Actor 模型并发安全

**User Story:** 作为游戏开发者,我希望利用 Actor 单线程特性自然避免并发冲突,无需手动加锁即可保证数据一致性

#### Acceptance Criteria

1. THE Persistence_System SHALL 在 Actor 消息处理线程内标记状态变更,避免跨线程数据竞争
2. THE Persistence_Queue SHALL 为每个 Actor 实例维护独立的队列,保证消息顺序性
3. WHEN 多个状态变更在同一消息处理周期内发生, THE Persistence_System SHALL 合并为单次 Delta_Update
4. THE Persistence_System SHALL 不使用互斥锁或其他同步原语保护 Actor 状态访问
5. WHEN Actor 接收到持久化确认, THE Persistence_System SHALL 在 Actor 线程内更新持久化版本号

### Requirement 5: 持久化标记与生命周期

**User Story:** 作为框架使用者,我希望通过简单的接口标记来启用 Actor 持久化功能,并自动管理持久化生命周期

#### Acceptance Criteria

1. THE Persistence_System SHALL 提供 Persistence_Marker 接口供 Actor 实现
2. WHEN Actor 实现 Persistence_Marker 接口, THE Persistence_System SHALL 自动注册持久化支持
3. THE Persistence_Marker SHALL 定义 `GetPersistenceKey()` 方法返回 Redis 键名
4. THE Persistence_Marker SHALL 定义 `SerializeState()` 方法返回 Protobuf_State
5. THE Persistence_Marker SHALL 定义 `DeserializeState(data []byte)` 方法从 Protobuf_State 恢复
6. WHEN Actor 销毁, THE Persistence_System SHALL 执行最终持久化操作保存最新状态
7. THE Persistence_System SHALL 支持配置 Actor 持久化 TTL,默认为 30 天

### Requirement 6: 批量刷新与定时器

**User Story:** 作为性能工程师,我希望系统能批量处理持久化请求,减少 Redis 网络往返次数,提升吞吐量

#### Acceptance Criteria

1. THE Persistence_System SHALL 启动 Flush_Timer 定期触发批量持久化,默认间隔为 50 毫秒
2. WHEN Flush_Timer 触发, THE Persistence_System SHALL 收集所有 Persistence_Queue 中的 Delta_Update
3. THE Persistence_System SHALL 使用 Redis Pipeline 执行批量写入操作
4. WHEN 单次批量写入数据量超过 1000 条, THE Persistence_System SHALL 分批执行避免 Redis 阻塞
5. THE Persistence_System SHALL 记录批量持久化的性能指标,包括延迟、吞吐量和失败率

### Requirement 7: 增量更新优化

**User Story:** 作为游戏开发者,我希望系统仅持久化变更的字段,减少网络传输和存储开销

#### Acceptance Criteria

1. WHERE 启用增量更新配置, THE Persistence_System SHALL 跟踪 Actor 状态字段的变更标记
2. WHEN Actor 调用状态更新方法, THE Persistence_System SHALL 标记对应字段为已变更
3. THE Persistence_System SHALL 生成仅包含变更字段的 Delta_Update
4. WHEN 累计变更字段超过总字段 50%, THE Persistence_System SHALL 切换为完整 State_Snapshot 持久化
5. THE Persistence_System SHALL 支持强制执行完整快照持久化的手动触发接口

### Requirement 8: 错误处理与重试机制

**User Story:** 作为运维人员,我希望系统能自动处理持久化失败情况,通过重试和降级策略保证数据最终一致性

#### Acceptance Criteria

1. IF Redis_Store 连接失败, THEN THE Persistence_System SHALL 执行指数退避重试,最大重试 3 次
2. WHEN 重试次数耗尽, THE Persistence_System SHALL 将失败的 Delta_Update 记录到本地日志文件
3. THE Persistence_System SHALL 提供手动重放失败持久化操作的工具脚本
4. IF Redis_Store 返回错误码, THEN THE Persistence_System SHALL 根据错误类型选择重试或跳过
5. THE Persistence_System SHALL 暴露持久化失败次数和重试次数的监控指标

### Requirement 9: 监控与可观测性

**User Story:** 作为运维人员,我希望监控持久化系统的健康状态,包括延迟、成功率和队列长度等关键指标

#### Acceptance Criteria

1. THE Persistence_System SHALL 记录每次持久化操作的延迟,并计算 P50、P95、P99 百分位值
2. THE Persistence_System SHALL 统计持久化成功率和失败率,按分钟聚合
3. THE Persistence_System SHALL 暴露 Persistence_Queue 当前长度的实时指标
4. WHEN 持久化延迟超过阈值, THE Persistence_System SHALL 记录警告日志
5. THE Persistence_System SHALL 支持通过 HTTP 端点暴露 Prometheus 格式的监控指标

### Requirement 10: 配置管理与灵活性

**User Story:** 作为配置管理员,我希望通过配置文件灵活调整持久化系统的行为,无需修改代码

#### Acceptance Criteria

1. THE Persistence_System SHALL 从配置文件加载 Redis 连接信息,包括地址、密码、数据库索引
2. THE Persistence_System SHALL 支持配置持久化延迟阈值、批量大小、重试次数等参数
3. THE Persistence_System SHALL 支持配置是否启用增量更新、懒加载、批量刷新等功能特性
4. WHEN 配置文件变更, THE Persistence_System SHALL 支持热重载部分配置参数,无需重启服务
5. THE Persistence_System SHALL 在启动时验证配置参数的合法性,并记录验证结果

### Requirement 11: 集成现有 Actor 框架

**User Story:** 作为框架集成者,我希望持久化系统能无缝集成到现有 Cherry Actor 框架中,兼容现有 Player_Actor 和 Spin_Actor 实现

#### Acceptance Criteria

1. THE Persistence_System SHALL 以 Actor 中间件形式集成,自动拦截 Actor 消息处理流程
2. THE Persistence_Middleware SHALL 在 Actor 消息处理完成后检查是否需要触发持久化
3. THE Persistence_System SHALL 兼容现有 `pomelo.ActorBase` 和 `cactor.Base` 父类
4. WHEN Actor 实现 Persistence_Marker 接口, THE Persistence_System SHALL 自动为其注册持久化钩子
5. THE Persistence_System SHALL 提供示例代码,展示如何改造现有 Player_Actor 和 Spin_Actor 以支持持久化

### Requirement 12: 数据版本兼容性

**User Story:** 作为版本管理员,我希望持久化系统支持数据版本演进,在 Protobuf 定义变更后仍能正常恢复旧版本数据

#### Acceptance Criteria

1. THE Persistence_System SHALL 在 JSON_Metadata 中记录 Protobuf Schema 版本号
2. WHEN 加载持久化数据, THE Recovery_Manager SHALL 检查版本号与当前代码版本的兼容性
3. IF 版本不兼容, THEN THE Recovery_Manager SHALL 执行数据迁移逻辑转换为新版本格式
4. THE Persistence_System SHALL 支持注册自定义版本迁移函数
5. WHEN 版本迁移失败, THE Recovery_Manager SHALL 记录错误并拒绝恢复数据

### Requirement 13: 测试与验证支持

**User Story:** 作为测试工程师,我希望系统提供测试工具和 Mock 实现,方便编写单元测试和集成测试

#### Acceptance Criteria

1. THE Persistence_System SHALL 提供 Mock Redis_Store 实现,支持内存模拟持久化操作
2. THE Persistence_System SHALL 提供测试工具函数,验证 Actor 状态是否正确持久化
3. THE Persistence_System SHALL 提供测试工具函数,模拟 Redis 故障场景
4. THE Persistence_System SHALL 提供性能测试工具,测量不同负载下的持久化延迟和吞吐量
5. THE Persistence_System SHALL 提供集成测试示例,覆盖完整的持久化与恢复流程
