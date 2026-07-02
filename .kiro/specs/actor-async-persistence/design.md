# Design Document: Actor 异步持久化系统

## 1. 系统概述

Actor 异步持久化系统是一个集成到 Cherry Actor 框架的中间件层,提供准实时(<100ms)、高性能的状态持久化能力。系统采用异步队列机制将 Actor 内存状态同步到 Redis,支持懒加载恢复、批量刷新和增量更新优化。核心设计理念是利用 Actor 模型的单线程特性自然避免并发冲突,无需手动加锁即可保证数据一致性。

### 1.1 核心特性

- **准实时持久化**: 状态变更在 100ms 内异步推送到 Redis,不阻塞消息处理
- **混合协议存储**: JSON 元数据(类型、版本、时间戳) + Protobuf 二进制状态
- **懒加载恢复**: Actor 首次访问时按需从 Redis 恢复状态,避免启动瓶颈
- **Actor 模型并发安全**: 单线程消息处理自然保证并发安全,无需锁
- **批量刷新优化**: 定时器批量处理持久化请求,使用 Redis Pipeline 减少网络开销
- **增量更新**: 仅持久化变更字段,减少传输和存储开销
- **错误处理与重试**: 指数退避重试、失败日志记录、手动重放工具

### 1.2 技术架构分层

```
┌─────────────────────────────────────────────────────────────┐
│                      Application Layer                       │
│  (Player Actor, Spin Actor, 其他业务 Actor)                  │
└─────────────────────┬───────────────────────────────────────┘
                      │ implements PersistenceMarker
┌─────────────────────┴───────────────────────────────────────┐
│                   Persistence Middleware                     │
│  (拦截消息处理,触发持久化检查,生命周期管理)                  │
└─────────────────────┬───────────────────────────────────────┘
                      │ enqueue
┌─────────────────────┴───────────────────────────────────────┐
│                    Persistence Queue                         │
│  (每 Actor 独立队列,合并同周期变更,异步缓冲)                 │
└─────────────────────┬───────────────────────────────────────┘
                      │ batch flush (50ms timer)
┌─────────────────────┴───────────────────────────────────────┐
│                     Redis Store Layer                        │
│  (Pipeline 批量写入, 键格式: actor:{type}:{id})              │
└─────────────────────────────────────────────────────────────┘
                      │
                      ▼
              Redis (Persistence)
```


## 2. 核心组件设计

### 2.1 PersistenceMarker 接口

Actor 实现此接口以声明持久化支持。接口定义了序列化、反序列化和键生成的核心方法。

```go
package persistence

// PersistenceMarker Actor 持久化标记接口
// Actor 实现此接口即可自动获得持久化支持
type PersistenceMarker interface {
    // GetPersistenceKey 返回 Redis 键名
    // 格式: actor:{ActorType}:{ActorID}
    // 例如: actor:player:10001, actor:spin:room_123
    GetPersistenceKey() string
    
    // SerializeState 序列化 Actor 状态为 Protobuf 字节
    // 返回: protobuf 编码的状态数据, 错误信息
    SerializeState() ([]byte, error)
    
    // DeserializeState 从 Protobuf 字节反序列化状态
    // 参数: protobuf 编码的状态数据
    // 返回: 错误信息 (成功则为 nil)
    DeserializeState(data []byte) error
    
    // GetSchemaVersion 返回 Protobuf Schema 版本号
    // 用于版本兼容性检查和数据迁移
    GetSchemaVersion() int32
}
```

**设计要点:**
- 接口方法在 Actor 线程内调用,保证线程安全
- SerializeState 返回完整状态快照或增量 Delta
- GetPersistenceKey 支持自定义键命名策略
- GetSchemaVersion 支持数据版本演进


### 2.2 PersistenceMiddleware 中间件

集成到 Cherry Actor System 的中间件,拦截消息处理流程并触发持久化检查。

```go
package persistence

import (
    cfacade "github.com/cherry-game/cherry/facade"
)

// PersistenceMiddleware Actor 持久化中间件
type PersistenceMiddleware struct {
    queue       *PersistenceQueue       // 持久化队列
    recovery    *RecoveryManager        // 恢复管理器
    config      *PersistenceConfig      // 配置
    metrics     *MetricsCollector       // 指标收集器
}

// OnActorInit Actor 初始化钩子
// 在 Actor OnInit 之前调用,执行懒加载恢复
func (m *PersistenceMiddleware) OnActorInit(actor cfacade.IActor) error {
    // 检查 Actor 是否实现 PersistenceMarker
    marker, ok := actor.(PersistenceMarker)
    if !ok {
        return nil // 不支持持久化,跳过
    }
    
    // 执行懒加载恢复
    return m.recovery.RecoverState(actor, marker)
}

// OnMessageProcessed 消息处理后钩子
// 在 Actor 消息处理完成后调用,检查状态变更并推送队列
func (m *PersistenceMiddleware) OnMessageProcessed(
    actor cfacade.IActor, 
    message *cfacade.Message,
) error {
    marker, ok := actor.(PersistenceMarker)
    if !ok {
        return nil
    }
    
    // 检查状态是否变更 (通过脏标记机制)
    if !m.isStateDirty(actor) {
        return nil
    }
    
    // 推送到持久化队列
    return m.queue.Enqueue(actor, marker)
}

// OnActorStop Actor 停止钩子
// 在 Actor 销毁前执行最终持久化
func (m *PersistenceMiddleware) OnActorStop(actor cfacade.IActor) error {
    marker, ok := actor.(PersistenceMarker)
    if !ok {
        return nil
    }
    
    // 强制执行完整快照持久化
    return m.queue.EnqueueFinalSnapshot(actor, marker)
}
```

**设计要点:**
- 通过类型断言检测 Actor 是否实现 PersistenceMarker
- 在 Actor 线程内执行所有持久化检查,避免竞态
- OnActorStop 确保最终状态被持久化
- 支持配置是否启用中间件


### 2.3 PersistenceQueue 异步队列

为每个 Actor 维护独立队列,缓冲待持久化的状态变更,支持批量刷新和变更合并。

```go
package persistence

import (
    "sync"
    "time"
)

// DeltaUpdate 增量更新数据结构
type DeltaUpdate struct {
    ActorID       string            // Actor ID
    ActorType     string            // Actor 类型
    Key           string            // Redis 键
    State         []byte            // Protobuf 状态数据
    ChangedFields map[string]bool   // 变更字段标记
    Timestamp     int64             // 变更时间戳
    SchemaVersion int32             // Schema 版本号
    IsFull        bool              // 是否完整快照
}

// PersistenceQueue 持久化队列
type PersistenceQueue struct {
    queues      map[string]*actorQueue // 每 Actor 独立队列
    mu          sync.RWMutex            // 保护 queues map
    flushTimer  *time.Ticker            // 批量刷新定时器
    store       *RedisStore             // Redis 存储
    config      *PersistenceConfig      // 配置
    metrics     *MetricsCollector       // 指标收集器
    stopCh      chan struct{}           // 停止信号
}

// actorQueue 单个 Actor 的队列
type actorQueue struct {
    updates chan *DeltaUpdate // 缓冲通道
    pending []*DeltaUpdate    // 待刷新的更新
    mu      sync.Mutex        // 保护 pending
}

// Enqueue 推送状态变更到队列
func (q *PersistenceQueue) Enqueue(
    actor cfacade.IActor, 
    marker PersistenceMarker,
) error {
    // 序列化状态
    stateBytes, err := marker.SerializeState()
    if err != nil {
        return err
    }
    
    // 构建 DeltaUpdate
    update := &DeltaUpdate{
        ActorID:       actor.ActorID(),
        ActorType:     getActorType(actor),
        Key:           marker.GetPersistenceKey(),
        State:         stateBytes,
        Timestamp:     time.Now().UnixMilli(),
        SchemaVersion: marker.GetSchemaVersion(),
        IsFull:        false, // 默认增量更新
    }
    
    // 推送到对应 Actor 队列
    q.getOrCreateQueue(actor.ActorID()).Push(update)
    
    q.metrics.RecordEnqueue(update)
    return nil
}

// StartFlushTimer 启动批量刷新定时器
func (q *PersistenceQueue) StartFlushTimer() {
    interval := time.Duration(q.config.FlushIntervalMs) * time.Millisecond
    q.flushTimer = time.NewTicker(interval)
    
    go func() {
        for {
            select {
            case <-q.flushTimer.C:
                q.FlushAll()
            case <-q.stopCh:
                return
            }
        }
    }()
}

// FlushAll 批量刷新所有队列
func (q *PersistenceQueue) FlushAll() error {
    startTime := time.Now()
    
    // 收集所有待刷新的更新
    allUpdates := q.collectAllPending()
    
    if len(allUpdates) == 0 {
        return nil
    }
    
    // 分批执行 (每批最多 1000 条)
    batchSize := q.config.BatchSize
    for i := 0; i < len(allUpdates); i += batchSize {
        end := i + batchSize
        if end > len(allUpdates) {
            end = len(allUpdates)
        }
        
        batch := allUpdates[i:end]
        if err := q.store.BatchWrite(batch); err != nil {
            q.metrics.RecordFlushError(err)
            return err
        }
    }
    
    elapsed := time.Since(startTime).Milliseconds()
    q.metrics.RecordFlushLatency(elapsed)
    q.metrics.RecordFlushSuccess(len(allUpdates))
    
    return nil
}
```

**设计要点:**
- 每个 Actor 独立队列保证消息顺序性
- 定时器默认 50ms 触发批量刷新
- 批量大小限制为 1000 条避免 Redis 阻塞
- 支持强制刷新和优雅关闭


### 2.4 RedisStore Redis 存储层

封装 Redis 客户端,提供批量写入、懒加载读取和错误重试能力。

```go
package persistence

import (
    "context"
    "encoding/json"
    "fmt"
    "time"
    
)

// JSONMetadata Redis 存储的 JSON 元数据
type JSONMetadata struct {
    ActorType     string `json:"actorType"`     // Actor 类型名称
    ActorID       string `json:"actorId"`       // Actor ID
    SchemaVersion int32  `json:"schemaVersion"` // Protobuf Schema 版本
    UpdatedAt     int64  `json:"updatedAt"`     // 最后更新时间戳 (毫秒)
    StateSize     int    `json:"stateSize"`     // State 字节大小
}

// RedisValue Redis 存储值结构
type RedisValue struct {
    Metadata JSONMetadata `json:"metadata"` // JSON 元数据
    State    []byte       `json:"state"`    // Protobuf 状态数据 (base64 编码)
}

// RedisStore Redis 持久化存储
type RedisStore struct {
    client  *redis.Client       // Redis 客户端
    config  *RedisConfig        // Redis 配置
    metrics *MetricsCollector   // 指标收集器
}

// BatchWrite 批量写入 Redis (使用 Pipeline)
func (s *RedisStore) BatchWrite(updates []*DeltaUpdate) error {
    ctx := context.Background()
    pipe := s.client.Pipeline()
    
    for _, update := range updates {
        // 构建混合格式数据
        value := RedisValue{
            Metadata: JSONMetadata{
                ActorType:     update.ActorType,
                ActorID:       update.ActorID,
                SchemaVersion: update.SchemaVersion,
                UpdatedAt:     update.Timestamp,
                StateSize:     len(update.State),
            },
            State: update.State,
        }
        
        // 序列化为 JSON
        valueBytes, err := json.Marshal(value)
        if err != nil {
            return fmt.Errorf("marshal value failed: %w", err)
        }
        
        // 添加到 Pipeline
        pipe.Set(ctx, update.Key, valueBytes, s.getTTL())
    }
    
    // 执行 Pipeline
    _, err := pipe.Exec(ctx)
    if err != nil {
        return s.handleWriteError(err, updates)
    }
    
    s.metrics.RecordBatchWriteSuccess(len(updates))
    return nil
}

// Load 懒加载单个 Actor 状态
func (s *RedisStore) Load(key string) (*RedisValue, error) {
    ctx := context.Background()
    
    // 从 Redis 读取
    valueBytes, err := s.client.Get(ctx, key).Bytes()
    if err == redis.Nil {
        return nil, ErrStateNotFound // 状态不存在
    }
    if err != nil {
        return nil, s.handleReadError(err)
    }
    
    // 反序列化 JSON
    var value RedisValue
    if err := json.Unmarshal(valueBytes, &value); err != nil {
        return nil, fmt.Errorf("unmarshal value failed: %w", err)
    }
    
    s.metrics.RecordLoadSuccess()
    return &value, nil
}

// handleWriteError 处理写入错误 (重试逻辑)
func (s *RedisStore) handleWriteError(
    err error, 
    updates []*DeltaUpdate,
) error {
    maxRetries := s.config.MaxRetries
    
    for retry := 0; retry < maxRetries; retry++ {
        // 指数退避
        backoff := time.Duration(1<<retry) * 100 * time.Millisecond
        time.Sleep(backoff)
        
        // 重试写入
        if retryErr := s.retryBatchWrite(updates); retryErr == nil {
            s.metrics.RecordRetrySuccess(retry + 1)
            return nil
        }
        
        s.metrics.RecordRetry()
    }
    
    // 重试耗尽,记录失败日志
    s.logFailedUpdates(updates)
    s.metrics.RecordRetryExhausted()
    return fmt.Errorf("batch write failed after %d retries: %w", maxRetries, err)
}

// getTTL 返回 Redis 键 TTL
func (s *RedisStore) getTTL() time.Duration {
    return time.Duration(s.config.TTLDays) * 24 * time.Hour
}
```

**设计要点:**
- 混合协议存储: JSON 元数据 + Protobuf 状态
- Redis Pipeline 批量写入减少网络往返
- 指数退避重试机制 (最多 3 次)
- 失败日志记录到本地文件供手动重放
- TTL 默认 30 天自动过期


### 2.5 RecoveryManager 恢复管理器

负责 Actor 启动时从 Redis 懒加载恢复状态,支持版本兼容性检查和数据迁移。

```go
package persistence

import (
    "fmt"
    "time"
    
    cfacade "github.com/cherry-game/cherry/facade"
    clog "github.com/cherry-game/cherry/logger"
)

// MigrationFunc 版本迁移函数类型
type MigrationFunc func(oldVersion int32, data []byte) ([]byte, error)

// RecoveryManager 状态恢复管理器
type RecoveryManager struct {
    store      *RedisStore                // Redis 存储
    config     *PersistenceConfig         // 配置
    migrations map[int32]MigrationFunc    // 版本迁移函数
    metrics    *MetricsCollector          // 指标收集器
}

// RecoverState 从 Redis 恢复 Actor 状态
func (m *RecoveryManager) RecoverState(
    actor cfacade.IActor,
    marker PersistenceMarker,
) error {
    startTime := time.Now()
    
    // 检查 Redis 中是否存在状态
    key := marker.GetPersistenceKey()
    value, err := m.store.Load(key)
    
    if err == ErrStateNotFound {
        // 状态不存在,使用默认初始状态
        clog.Infof("[Recovery] No saved state found for %s, using default", key)
        m.metrics.RecordRecoverySkipped()
        return nil
    }
    
    if err != nil {
        // 恢复失败,允许使用默认状态启动
        clog.Errorf("[Recovery] Failed to load state for %s: %v", key, err)
        m.metrics.RecordRecoveryError()
        return nil // 不阻止 Actor 启动
    }
    
    // 检查版本兼容性
    currentVersion := marker.GetSchemaVersion()
    savedVersion := value.Metadata.SchemaVersion
    
    stateData := value.State
    if savedVersion != currentVersion {
        // 执行版本迁移
        migratedData, err := m.migrateData(savedVersion, currentVersion, value.State)
        if err != nil {
            clog.Errorf("[Recovery] Migration failed for %s: %v", key, err)
            m.metrics.RecordMigrationError()
            return fmt.Errorf("migration failed: %w", err)
        }
        stateData = migratedData
        m.metrics.RecordMigrationSuccess(savedVersion, currentVersion)
    }
    
    // 反序列化状态
    if err := marker.DeserializeState(stateData); err != nil {
        clog.Errorf("[Recovery] Deserialize failed for %s: %v", key, err)
        m.metrics.RecordRecoveryError()
        return nil // 允许使用默认状态
    }
    
    elapsed := time.Since(startTime).Milliseconds()
    m.metrics.RecordRecoverySuccess(elapsed)
    
    clog.Infof("[Recovery] Successfully recovered state for %s (version %d, %dms)",
        key, savedVersion, elapsed)
    
    return nil
}

// RegisterMigration 注册版本迁移函数
func (m *RecoveryManager) RegisterMigration(
    fromVersion int32,
    migrationFunc MigrationFunc,
) {
    m.migrations[fromVersion] = migrationFunc
}

// migrateData 执行数据迁移
func (m *RecoveryManager) migrateData(
    fromVersion int32,
    toVersion int32,
    data []byte,
) ([]byte, error) {
    // 逐版本迁移 (支持跨多个版本升级)
    currentData := data
    for v := fromVersion; v < toVersion; v++ {
        migrationFunc, exists := m.migrations[v]
        if !exists {
            return nil, fmt.Errorf("migration function not found for version %d", v)
        }
        
        migratedData, err := migrationFunc(v, currentData)
        if err != nil {
            return nil, fmt.Errorf("migration from v%d failed: %w", v, err)
        }
        
        currentData = migratedData
    }
    
    return currentData, nil
}

// BatchPreload 批量预加载多个 Actor 状态
func (m *RecoveryManager) BatchPreload(keys []string) (map[string]*RedisValue, error) {
    // 使用 Redis MGET 批量读取
    results := make(map[string]*RedisValue)
    
    for _, key := range keys {
        value, err := m.store.Load(key)
        if err == nil {
            results[key] = value
        }
    }
    
    m.metrics.RecordBatchPreload(len(results))
    return results, nil
}
```

**设计要点:**
- 懒加载策略: Actor 初始化时按需恢复
- 版本兼容性检查: 对比 Schema 版本号
- 版本迁移支持: 注册自定义迁移函数,逐版本升级
- 容错设计: 恢复失败不阻止 Actor 启动
- 批量预加载: 可选择性预加载高频 Actor


### 2.6 配置管理

```go
package persistence

// PersistenceConfig 持久化系统配置
type PersistenceConfig struct {
    // 基础配置
    Enabled           bool   `json:"enabled"`            // 是否启用持久化
    FlushIntervalMs   int    `json:"flushIntervalMs"`    // 刷新间隔 (毫秒,默认 50)
    DelayThresholdMs  int    `json:"delayThresholdMs"`   // 延迟阈值 (毫秒,默认 100)
    BatchSize         int    `json:"batchSize"`          // 批量大小 (默认 1000)
    
    // 功能特性开关
    EnableDeltaUpdate bool   `json:"enableDeltaUpdate"`  // 是否启用增量更新
    EnableLazyLoad    bool   `json:"enableLazyLoad"`     // 是否启用懒加载
    EnableBatchFlush  bool   `json:"enableBatchFlush"`   // 是否启用批量刷新
    
    // Redis 配置
    Redis RedisConfig `json:"redis"`
}

// RedisConfig Redis 连接配置
type RedisConfig struct {
    Address    string `json:"address"`    // Redis 地址 (例: localhost:6379)
    Password   string `json:"password"`   // Redis 密码
    DB         int    `json:"db"`         // 数据库索引 (默认 0)
    MaxRetries int    `json:"maxRetries"` // 最大重试次数 (默认 3)
    TTLDays    int    `json:"ttlDays"`    // 键 TTL 天数 (默认 30)
    PoolSize   int    `json:"poolSize"`   // 连接池大小 (默认 10)
}

// LoadConfig 从配置文件加载配置
func LoadConfig(configPath string) (*PersistenceConfig, error) {
    // 读取 JSON 配置文件
    // 设置默认值
    // 验证配置参数
    return &PersistenceConfig{}, nil
}

// ValidateConfig 验证配置参数合法性
func ValidateConfig(config *PersistenceConfig) error {
    if config.FlushIntervalMs < 10 {
        return fmt.Errorf("flushIntervalMs must >= 10ms")
    }
    if config.BatchSize < 1 || config.BatchSize > 10000 {
        return fmt.Errorf("batchSize must between 1 and 10000")
    }
    if config.Redis.MaxRetries < 0 {
        return fmt.Errorf("maxRetries must >= 0")
    }
    return nil
}
```

**配置文件示例 (JSON):**

```json
{
  "persistence": {
    "enabled": true,
    "flushIntervalMs": 50,
    "delayThresholdMs": 100,
    "batchSize": 1000,
    "enableDeltaUpdate": true,
    "enableLazyLoad": true,
    "enableBatchFlush": true,
    "redis": {
      "address": "localhost:6379",
      "password": "",
      "db": 0,
      "maxRetries": 3,
      "ttlDays": 30,
      "poolSize": 10
    }
  }
}
```


### 2.7 监控与指标

```go
package persistence

import (
    "sync/atomic"
    "time"
)

// MetricsCollector 指标收集器
type MetricsCollector struct {
    // 延迟指标
    persistenceLatencies []int64  // 持久化延迟采样 (毫秒)
    recoveryLatencies    []int64  // 恢复延迟采样 (毫秒)
    
    // 计数指标
    enqueueCount      atomic.Int64  // 入队计数
    flushSuccessCount atomic.Int64  // 刷新成功计数
    flushErrorCount   atomic.Int64  // 刷新失败计数
    retryCount        atomic.Int64  // 重试计数
    recoveryCount     atomic.Int64  // 恢复计数
    migrationCount    atomic.Int64  // 迁移计数
    
    // 队列指标
    queueLengthGauge  atomic.Int64  // 当前队列长度
}

// RecordPersistenceLatency 记录持久化延迟
func (m *MetricsCollector) RecordPersistenceLatency(latencyMs int64) {
    m.persistenceLatencies = append(m.persistenceLatencies, latencyMs)
}

// GetLatencyPercentiles 计算延迟百分位值
func (m *MetricsCollector) GetLatencyPercentiles() (p50, p95, p99 int64) {
    // 对延迟数据排序并计算 P50, P95, P99
    return 0, 0, 0
}

// GetMetricsSnapshot 获取指标快照 (Prometheus 格式)
func (m *MetricsCollector) GetMetricsSnapshot() map[string]interface{} {
    p50, p95, p99 := m.GetLatencyPercentiles()
    
    return map[string]interface{}{
        "persistence_enqueue_total":       m.enqueueCount.Load(),
        "persistence_flush_success_total": m.flushSuccessCount.Load(),
        "persistence_flush_error_total":   m.flushErrorCount.Load(),
        "persistence_retry_total":         m.retryCount.Load(),
        "persistence_recovery_total":      m.recoveryCount.Load(),
        "persistence_latency_p50_ms":      p50,
        "persistence_latency_p95_ms":      p95,
        "persistence_latency_p99_ms":      p99,
        "persistence_queue_length":        m.queueLengthGauge.Load(),
    }
}

// ExposeHTTPMetrics 暴露 HTTP 指标端点
func (m *MetricsCollector) ExposeHTTPMetrics(port int) {
    // 启动 HTTP 服务器,提供 /metrics 端点
    // 返回 Prometheus 格式指标
}
```

**暴露的 Prometheus 指标:**

```
# TYPE persistence_enqueue_total counter
persistence_enqueue_total 12345

# TYPE persistence_flush_success_total counter
persistence_flush_success_total 234

# TYPE persistence_flush_error_total counter
persistence_flush_error_total 12

# TYPE persistence_retry_total counter
persistence_retry_total 36

# TYPE persistence_latency_p50_ms gauge
persistence_latency_p50_ms 15

# TYPE persistence_latency_p95_ms gauge
persistence_latency_p95_ms 48

# TYPE persistence_latency_p99_ms gauge
persistence_latency_p99_ms 89

# TYPE persistence_queue_length gauge
persistence_queue_length 23
```


## 3. 核心流程设计

### 3.1 状态变更持久化流程

```
┌─────────────────────────────────────────────────────────────────┐
│  1. Actor 处理消息,修改内部状态                                   │
│     - 业务逻辑执行 (例: player.UpdateMoney(100))                 │
│     - 状态字段变更 (例: playerData.Money += 100)                 │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  2. 消息处理完成,触发 PersistenceMiddleware.OnMessageProcessed │
│     - 检查 Actor 是否实现 PersistenceMarker                      │
│     - 检查状态是否变更 (脏标记机制)                              │
└─────────────────┬───────────────────────────────────────────────┘
                  │ if dirty
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  3. 调用 marker.SerializeState() 序列化状态                      │
│     - 在 Actor 线程内执行,保证线程安全                           │
│     - 返回 Protobuf 字节数据                                     │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  4. 构建 DeltaUpdate 并推送到 PersistenceQueue                   │
│     - 包含 ActorID, ActorType, State, Timestamp, Version        │
│     - 推送到对应 Actor 的独立队列                                │
│     - 异步操作,不阻塞 Actor 消息处理                             │
└─────────────────┬───────────────────────────────────────────────┘
                  │ async
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  5. Flush_Timer 定时触发 (默认 50ms)                             │
│     - 收集所有队列中的 DeltaUpdate                               │
│     - 合并同一 Actor 的多次变更                                  │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  6. RedisStore.BatchWrite() 批量写入                             │
│     - 使用 Redis Pipeline 批量执行 SET 命令                      │
│     - 每批最多 1000 条,超出则分批执行                            │
│     - 错误重试: 指数退避,最多 3 次                               │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  7. Redis 持久化完成                                             │
│     - 数据存储格式: JSON 元数据 + Protobuf State                 │
│     - 键格式: actor:{ActorType}:{ActorID}                        │
│     - TTL: 30 天自动过期                                         │
└─────────────────────────────────────────────────────────────────┘
```

**关键时序要求:**
- 步骤 1-4 在 Actor 线程内同步执行,耗时 < 1ms
- 步骤 4 推送队列为异步操作,不阻塞消息处理
- 步骤 5 定时器周期为 50ms,确保批量效率
- 步骤 6 Redis 写入耗时 < 50ms (批量 Pipeline)
- 端到端延迟 < 100ms (从状态变更到 Redis 写入完成)


### 3.2 懒加载恢复流程

```
┌─────────────────────────────────────────────────────────────────┐
│  1. Actor 初始化 (首次创建或重启后)                              │
│     - Actor System 创建 Actor 实例                               │
│     - 触发 PersistenceMiddleware.OnActorInit()                  │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  2. 检查 Actor 是否实现 PersistenceMarker                        │
│     - 类型断言: marker, ok := actor.(PersistenceMarker)         │
│     - 不实现则跳过恢复流程                                       │
└─────────────────┬───────────────────────────────────────────────┘
                  │ if ok
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  3. RecoveryManager.RecoverState() 尝试恢复                      │
│     - 获取 Redis 键: key = marker.GetPersistenceKey()           │
│     - 查询 Redis: value = redisStore.Load(key)                  │
└─────────────────┬───────────────────────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
        ▼                   ▼
   状态存在             状态不存在
        │                   │
        │                   ▼
        │         ┌──────────────────────┐
        │         │ 使用默认初始状态启动  │
        │         │ 记录日志: No saved   │
        │         │ state found          │
        │         └──────────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│  4. 解析 JSON 元数据,检查版本兼容性                              │
│     - 对比 SchemaVersion: saved vs current                       │
└─────────────────┬───────────────────────────────────────────────┘
                  │
        ┌─────────┴─────────┐
        │                   │
        ▼                   ▼
   版本一致            版本不一致
        │                   │
        │                   ▼
        │         ┌──────────────────────┐
        │         │ 执行版本迁移          │
        │         │ migrateData(from, to)│
        │         │ - 调用注册的迁移函数  │
        │         │ - 逐版本升级数据      │
        │         └─────────┬────────────┘
        │                   │ migrated data
        ├───────────────────┘
        │
        ▼
┌─────────────────────────────────────────────────────────────────┐
│  5. 反序列化 Protobuf 状态                                       │
│     - 调用 marker.DeserializeState(stateBytes)                  │
│     - 在 Actor 线程内执行,加载数据到内存                         │
└─────────────────┬───────────────────────────────────────────────┘
                  │
                  ▼
┌─────────────────────────────────────────────────────────────────┐
│  6. 恢复完成,Actor 继续正常初始化                                │
│     - 触发 Actor.OnInit() 业务逻辑                               │
│     - 记录恢复成功日志和延迟指标                                 │
└─────────────────────────────────────────────────────────────────┘
```

**容错设计:**
- Redis 查询失败: 允许 Actor 使用默认状态启动,不阻塞初始化
- 版本迁移失败: 记录错误并拒绝恢复,防止数据损坏
- 反序列化失败: 允许使用默认状态,记录错误日志
- 恢复超时 (>200ms): 记录警告,但不中断流程


### 3.3 增量更新优化流程

当启用增量更新配置时,系统仅持久化变更的字段,减少网络传输和存储开销。

```go
// 脏标记机制示例
type PlayerActorState struct {
    Money    int64  `protobuf:"1"`
    Diamond  int64  `protobuf:"2"`
    Level    int32  `protobuf:"3"`
    Exp      int64  `protobuf:"4"`
    
    // 脏标记 (不持久化)
    dirtyFields map[string]bool `protobuf:"-"`
}

// MarkDirty 标记字段为已变更
func (s *PlayerActorState) MarkDirty(fieldName string) {
    if s.dirtyFields == nil {
        s.dirtyFields = make(map[string]bool)
    }
    s.dirtyFields[fieldName] = true
}

// UpdateMoney 更新金币并标记脏字段
func (p *PlayerActor) UpdateMoney(delta int64) {
    p.state.Money += delta
    p.state.MarkDirty("Money")
}

// SerializeState 序列化状态 (支持增量)
func (p *PlayerActor) SerializeState() ([]byte, error) {
    // 检查变更字段比例
    totalFields := 4
    dirtyCount := len(p.state.dirtyFields)
    
    if dirtyCount > totalFields/2 {
        // 超过 50% 变更,执行完整快照
        return proto.Marshal(p.state)
    }
    
    // 执行增量序列化 (仅包含变更字段)
    deltaState := &PlayerActorState{}
    if p.state.dirtyFields["Money"] {
        deltaState.Money = p.state.Money
    }
    if p.state.dirtyFields["Diamond"] {
        deltaState.Diamond = p.state.Diamond
    }
    // ... 其他字段
    
    return proto.Marshal(deltaState)
}
```

**增量更新策略:**
1. 每个状态字段对应一个脏标记位
2. 状态更新方法调用时自动标记脏字段
3. 序列化时检查脏字段比例:
   - < 50%: 仅序列化变更字段 (增量 Delta)
   - >= 50%: 序列化完整状态 (完整快照)
4. 持久化完成后清空脏标记
5. Actor 销毁时强制执行完整快照


### 3.4 错误处理与重试机制

系统采用分层错误处理策略,确保持久化失败不影响 Actor 正常运行。

```
持久化失败
    │
    ▼
┌─────────────────────────┐
│ 1. 识别错误类型          │
│ - 连接失败 (可重试)      │
│ - 超时错误 (可重试)      │
│ - 数据错误 (不可重试)    │
│ - 权限错误 (不可重试)    │
└────────┬────────────────┘
         │
         ▼
    可重试?
         │
    ┌────┴────┐
    │         │
   YES       NO
    │         │
    │         ▼
    │    ┌────────────────┐
    │    │ 记录错误日志    │
    │    │ 跳过本次持久化  │
    │    └────────────────┘
    │
    ▼
┌─────────────────────────┐
│ 2. 指数退避重试          │
│ - Retry 1: 100ms 后     │
│ - Retry 2: 200ms 后     │
│ - Retry 3: 400ms 后     │
└────────┬────────────────┘
         │
         ▼
    重试成功?
         │
    ┌────┴────┐
    │         │
   YES       NO
    │         │
    │         ▼
    │    ┌────────────────────────┐
    │    │ 3. 重试耗尽             │
    │    │ - 记录到失败日志文件    │
    │    │   /var/log/persistence/ │
    │    │   failed_updates.log    │
    │    │ - 暴露失败指标          │
    │    │ - 触发告警 (可选)       │
    │    └────────────────────────┘
    │
    ▼
持久化完成
```

**失败日志格式 (JSON Lines):**

```json
{"timestamp":1704067200000,"actorId":"player:10001","actorType":"Player","key":"actor:player:10001","error":"connection refused","retries":3,"stateSize":1024}
{"timestamp":1704067205000,"actorId":"spin:room_123","actorType":"Spin","key":"actor:spin:room_123","error":"timeout","retries":3,"stateSize":2048}
```

**手动重放工具:**

```bash
# 读取失败日志并重放持久化操作
./replay_failed_persistence.sh /var/log/persistence/failed_updates.log

# 支持时间范围过滤
./replay_failed_persistence.sh --from="2024-01-01 00:00:00" --to="2024-01-02 00:00:00"
```


## 4. 集成现有 Cherry Actor 框架

### 4.1 中间件集成点

持久化系统以中间件形式集成到 Cherry Actor System,拦截 Actor 生命周期的关键钩子:

```go
// 在 Cherry Actor System 初始化时注册中间件
func InitActorSystem(app cfacade.IApplication) *cherryActor.System {
    system := cherryActor.NewSystem(app)
    
    // 加载持久化配置
    config, err := persistence.LoadConfig("./config/persistence.json")
    if err != nil {
        clog.Fatalf("Load persistence config failed: %v", err)
    }
    
    if config.Enabled {
        // 初始化持久化中间件
        middleware := persistence.NewMiddleware(config)
        
        // 注册到 Actor System
        system.RegisterMiddleware(middleware)
        
        clog.Infof("Persistence middleware registered")
    }
    
    return system
}
```

**中间件钩子拦截点:**

1. **OnActorInit (Actor 初始化前)**
   - 执行懒加载恢复
   - 检查 Redis 状态快照
   - 反序列化并加载状态

2. **OnMessageProcessed (消息处理后)**
   - 检查状态是否变更
   - 序列化变更状态
   - 推送到持久化队列

3. **OnActorStop (Actor 停止前)**
   - 执行最终持久化
   - 确保最新状态被保存
   - 清理队列资源


### 4.2 改造现有 Player Actor 示例

以下展示如何改造现有 `actorPlayer` 以支持持久化:

```go
package player

import (
    "github.com/cherry-game/cherry/net/parser/pomelo"
    "github.com/cherry-game/examples/persistence"
    "google.golang.org/protobuf/proto"
)

// actorPlayer 实现 PersistenceMarker 接口
type actorPlayer struct {
    pomelo.ActorBase
    isOnline   bool
    userId     int64
    uid        int64
    playerData *PlayerData
}

// ============ 实现 PersistenceMarker 接口 ============

// GetPersistenceKey 返回 Redis 键名
func (p *actorPlayer) GetPersistenceKey() string {
    return fmt.Sprintf("actor:player:%d", p.userId)
}

// SerializeState 序列化玩家状态
func (p *actorPlayer) SerializeState() ([]byte, error) {
    if p.playerData == nil {
        return nil, fmt.Errorf("player data is nil")
    }
    
    // 转换为 Protobuf 消息
    pbState := &pb.PlayerState{
        UserId:   p.playerData.UserId,
        Money:    p.playerData.Money,
        Diamond:  p.playerData.Diamond,
        Level:    p.playerData.Level,
        Exp:      p.playerData.Exp,
        // ... 其他字段
    }
    
    return proto.Marshal(pbState)
}

// DeserializeState 反序列化玩家状态
func (p *actorPlayer) DeserializeState(data []byte) error {
    pbState := &pb.PlayerState{}
    if err := proto.Unmarshal(data, pbState); err != nil {
        return fmt.Errorf("unmarshal failed: %w", err)
    }
    
    // 加载到内存
    p.playerData = &PlayerData{
        UserId:  pbState.UserId,
        Money:   pbState.Money,
        Diamond: pbState.Diamond,
        Level:   pbState.Level,
        Exp:     pbState.Exp,
        // ... 其他字段
    }
    
    p.userId = pbState.UserId
    return nil
}

// GetSchemaVersion 返回 Schema 版本号
func (p *actorPlayer) GetSchemaVersion() int32 {
    return 1 // 当前版本
}

// ============ 业务逻辑 (保持不变) ============

// UpdateMoney 更新金币
func (p *actorPlayer) UpdateMoney(ctx context.Context, req *pb.UpdateMoneyRequest) {
    // 业务逻辑
    p.playerData.Money += req.Delta
    
    // 持久化系统会自动在消息处理完成后触发持久化
    // 无需手动调用,保持业务逻辑简洁
}
```

**关键改造点:**
1. 实现 `PersistenceMarker` 接口的 4 个方法
2. 定义 Protobuf 消息 `PlayerState` 用于序列化
3. 业务逻辑代码无需修改,持久化自动触发
4. 状态加载由中间件在 Actor 初始化时自动完成


### 4.3 Protobuf 定义示例

```protobuf
syntax = "proto3";

package pb;

option go_package = "./pb";

// PlayerState 玩家状态数据 (用于持久化)
message PlayerState {
    int64 user_id = 1;       // 玩家 ID
    int64 money = 2;         // 金币
    int64 diamond = 3;       // 钻石
    int32 level = 4;         // 等级
    int64 exp = 5;           // 经验值
    string name = 6;         // 昵称
    int32 gender = 7;        // 性别
    int64 create_time = 8;   // 创建时间
    int64 last_login = 9;    // 最后登录时间
}

// SpinState 转轮状态数据 (用于持久化)
message SpinState {
    string room_id = 1;      // 房间 ID
    int32 level = 2;         // 难度等级
    int64 total_bet = 3;     // 累计下注
    int64 total_win = 4;     // 累计赢取
    int32 spin_count = 5;    // 转轮次数
    bytes progress_data = 6; // 进度数据 (嵌套 Protobuf)
}
```


## 5. 数据模型与存储格式

### 5.1 Redis 键命名规范

```
格式: actor:{ActorType}:{ActorID}

示例:
- actor:player:10001         // 玩家 Actor (ID=10001)
- actor:player:10002         // 玩家 Actor (ID=10002)
- actor:spin:room_123        // 转轮 Actor (房间 ID=room_123)
- actor:spin:room_456        // 转轮 Actor (房间 ID=room_456)
```

### 5.2 Redis 值存储格式 (混合协议)

```json
{
  "metadata": {
    "actorType": "player",
    "actorId": "10001",
    "schemaVersion": 1,
    "updatedAt": 1704067200000,
    "stateSize": 1024
  },
  "state": "<base64 encoded protobuf bytes>"
}
```

**字段说明:**
- `metadata.actorType`: Actor 类型名称 (可读,便于调试)
- `metadata.actorId`: Actor ID (可读)
- `metadata.schemaVersion`: Protobuf Schema 版本号 (版本兼容性检查)
- `metadata.updatedAt`: 最后更新时间戳 (毫秒,用于监控和排查)
- `metadata.stateSize`: State 字节大小 (用于监控和容量规划)
- `state`: Base64 编码的 Protobuf 二进制数据 (高效存储)

### 5.3 数据版本演进示例

假设 PlayerState 从 v1 升级到 v2,新增 `vip_level` 字段:

```protobuf
// Version 1
message PlayerState {
    int64 user_id = 1;
    int64 money = 2;
    int32 level = 3;
}

// Version 2 (新增字段)
message PlayerState {
    int64 user_id = 1;
    int64 money = 2;
    int32 level = 3;
    int32 vip_level = 4;  // 新增 VIP 等级
}
```

**版本迁移函数:**

```go
// 注册 v1 -> v2 迁移函数
recovery.RegisterMigration(1, func(oldVersion int32, data []byte) ([]byte, error) {
    // 反序列化 v1 数据
    v1State := &pb.PlayerStateV1{}
    if err := proto.Unmarshal(data, v1State); err != nil {
        return nil, err
    }
    
    // 转换为 v2 数据
    v2State := &pb.PlayerState{
        UserId:   v1State.UserId,
        Money:    v1State.Money,
        Level:    v1State.Level,
        VipLevel: 0, // 默认值
    }
    
    // 序列化 v2 数据
    return proto.Marshal(v2State)
})
```


## 6. Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: 状态变更及时入队

*For any* Actor 状态变更,变更应在 100 毫秒内被推送到 Persistence_Queue

**Validates: Requirements 1.1**

### Property 2: 持久化不阻塞消息处理

*For any* Actor 消息处理操作,持久化机制不应显著增加消息处理延迟 (延迟增量 < 5ms)

**Validates: Requirements 1.2**

### Property 3: 批量写入及时完成

*For any* 包含待处理数据的 Persistence_Queue,批量写入应在 50 毫秒内完成

**Validates: Requirements 1.3**

### Property 4: 失败重试有界

*For any* 持久化失败操作,重试次数应恰好为 3 次,并在重试耗尽后记录失败日志

**Validates: Requirements 1.4**

### Property 5: 混合格式完整性

*For any* 持久化的 Actor 状态,存储数据应同时包含 JSON 元数据和 Protobuf 状态,且元数据包含类型、版本、时间戳字段

**Validates: Requirements 2.1, 2.2**

### Property 6: 序列化往返一致性

*For any* Actor 状态,执行序列化后再反序列化应产生等价的状态数据

**Validates: Requirements 2.3**

### Property 7: 键格式一致性

*For any* Actor 类型和 ID 组合,生成的 Redis 键应匹配格式 `actor:{ActorType}:{ActorID}`

**Validates: Requirements 2.4**

### Property 8: 元数据优先解析

*For any* 持久化数据加载操作,JSON 元数据应在 Protobuf 状态之前被解析和验证

**Validates: Requirements 2.5**

### Property 9: 懒加载触发检查

*For any* Actor 初始化操作,Recovery_Manager 应检查 Redis 中是否存在对应状态快照

**Validates: Requirements 3.1**

### Property 10: 存在快照则恢复

*For any* 在 Redis 中存在状态快照的 Actor,Recovery_Manager 应加载并反序列化该快照

**Validates: Requirements 3.2**

### Property 11: 无快照则默认初始化

*For any* 在 Redis 中不存在状态快照的 Actor,应使用默认初始状态启动

**Validates: Requirements 3.3**

### Property 12: 恢复操作及时完成

*For any* Actor 状态恢复操作,应在 200 毫秒内完成

**Validates: Requirements 3.4**

### Property 13: 恢复失败不阻塞启动

*For any* 恢复失败的 Actor,应记录错误并允许使用默认状态启动

**Validates: Requirements 3.5**

### Property 14: Actor 线程内标记

*For any* 状态变更标记操作,应在 Actor 消息处理线程内执行

**Validates: Requirements 4.1**

### Property 15: 队列独立性

*For any* Actor 实例,Persistence_Queue 应为其维护独立的队列

**Validates: Requirements 4.2**

### Property 16: 同周期变更合并

*For any* 在同一消息处理周期内发生的多个状态变更,应合并为单次 Delta_Update

**Validates: Requirements 4.3**

### Property 17: Actor 线程内版本更新

*For any* 持久化确认操作,持久化版本号更新应在 Actor 线程内执行

**Validates: Requirements 4.5**

### Property 18: 接口实现自动注册

*For any* 实现 Persistence_Marker 接口的 Actor,持久化系统应自动为其注册持久化支持

**Validates: Requirements 5.2, 11.4**

### Property 19: Actor 销毁最终持久化

*For any* 销毁的 Actor,持久化系统应执行最终持久化操作保存最新状态

**Validates: Requirements 5.6**

### Property 20: 定时器周期触发

*For any* 时间周期,Flush_Timer 应以 50 毫秒间隔触发批量持久化

**Validates: Requirements 6.1**

### Property 21: 批量收集完整性

*For any* Flush_Timer 触发,应收集所有 Persistence_Queue 中的 Delta_Update

**Validates: Requirements 6.2**

### Property 22: 大批量分批执行

*For any* 单次批量写入数据量超过 1000 条的情况,应分批执行避免 Redis 阻塞

**Validates: Requirements 6.4**

### Property 23: 性能指标记录

*For any* 持久化操作,系统应记录延迟、吞吐量和失败率指标

**Validates: Requirements 6.5**

### Property 24: 字段变更跟踪

*For any* 状态字段修改操作,应标记对应字段为已变更

**Validates: Requirements 7.1, 7.2**

### Property 25: Delta 仅含变更字段

*For any* 增量更新,生成的 Delta_Update 应仅包含已变更的字段

**Validates: Requirements 7.3**

### Property 26: 变更比例阈值切换

*For any* 状态更新,当变更字段超过总字段 50% 时,应切换为完整 State_Snapshot 持久化

**Validates: Requirements 7.4**

### Property 27: 指数退避重试

*For any* Redis 连接失败,应执行指数退避重试,最大重试 3 次

**Validates: Requirements 8.1**

### Property 28: 重试耗尽日志记录

*For any* 重试次数耗尽的 Delta_Update,应记录到本地日志文件

**Validates: Requirements 8.2**

### Property 29: 错误类型决策

*For any* Redis 返回的错误,应根据错误类型选择重试或跳过

**Validates: Requirements 8.4**

### Property 30: 失败指标暴露

*For any* 持久化失败和重试事件,应增加相应的监控指标计数

**Validates: Requirements 8.5**

### Property 31: 延迟百分位计算

*For any* 持久化操作集合,应计算 P50、P95、P99 延迟百分位值

**Validates: Requirements 9.1**

### Property 32: 成功率统计

*For any* 时间窗口,应统计该窗口内的持久化成功率和失败率

**Validates: Requirements 9.2**

### Property 33: 队列长度实时暴露

*For any* 时刻,Persistence_Queue 的当前长度应作为实时指标暴露

**Validates: Requirements 9.3**

### Property 34: 超阈值告警

*For any* 持久化延迟超过配置阈值的操作,应记录警告日志

**Validates: Requirements 9.4**

### Property 35: 配置热重载有效

*For any* 支持热重载的配置参数变更,变更应在不重启服务的情况下生效

**Validates: Requirements 10.4**

### Property 36: 配置验证完整

*For any* 启动时加载的配置,应验证参数合法性并记录验证结果

**Validates: Requirements 10.5**

### Property 37: 消息处理后持久化检查

*For any* Actor 消息处理完成后,Persistence_Middleware 应检查是否需要触发持久化

**Validates: Requirements 11.2**

### Property 38: Schema 版本记录

*For any* 持久化数据,JSON 元数据应包含 Protobuf Schema 版本号

**Validates: Requirements 12.1**

### Property 39: 版本兼容性检查

*For any* 加载持久化数据操作,Recovery_Manager 应检查版本号与当前代码版本的兼容性

**Validates: Requirements 12.2**

### Property 40: 版本不兼容触发迁移

*For any* 版本不兼容的持久化数据,Recovery_Manager 应执行数据迁移逻辑

**Validates: Requirements 12.3**

### Property 41: 迁移失败拒绝恢复

*For any* 版本迁移失败的情况,Recovery_Manager 应记录错误并拒绝恢复数据

**Validates: Requirements 12.5**


## 7. 测试策略

### 7.1 单元测试

**测试工具: Go testing + testify**

重点测试组件:
- `PersistenceMarker` 接口实现
- `RedisStore` 序列化/反序列化
- `RecoveryManager` 版本迁移逻辑
- `PersistenceQueue` 队列操作
- 配置加载和验证

**Mock 实现:**

```go
// MockRedisStore 内存模拟 Redis 存储
type MockRedisStore struct {
    data map[string]*RedisValue
    mu   sync.RWMutex
}

func (m *MockRedisStore) BatchWrite(updates []*DeltaUpdate) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    for _, update := range updates {
        value := RedisValue{
            Metadata: JSONMetadata{
                ActorType:     update.ActorType,
                ActorID:       update.ActorID,
                SchemaVersion: update.SchemaVersion,
                UpdatedAt:     update.Timestamp,
            },
            State: update.State,
        }
        m.data[update.Key] = &value
    }
    return nil
}

func (m *MockRedisStore) Load(key string) (*RedisValue, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    value, ok := m.data[key]
    if !ok {
        return nil, ErrStateNotFound
    }
    return value, nil
}
```

### 7.2 属性测试 (Property-Based Testing)

**测试工具: gopter (Go port of QuickCheck)**

属性测试覆盖核心正确性属性,每个测试最少 100 次迭代:

```go
func TestProperty_SerializationRoundTrip(t *testing.T) {
    properties := gopter.NewProperties(nil)
    
    properties.Property("序列化往返一致性", prop.ForAll(
        func(state *PlayerState) bool {
            // 序列化
            data, err := proto.Marshal(state)
            if err != nil {
                return false
            }
            
            // 反序列化
            recovered := &PlayerState{}
            err = proto.Unmarshal(data, recovered)
            if err != nil {
                return false
            }
            
            // 验证等价性
            return proto.Equal(state, recovered)
        },
        genPlayerState(), // 生成器
    ))
    
    properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// 属性生成器
func genPlayerState() gopter.Gen {
    return gopter.CombineGens(
        gen.Int64Range(1, 1000000),  // user_id
        gen.Int64Range(0, 999999999), // money
        gen.Int64Range(0, 999999),    // diamond
        gen.Int32Range(1, 100),       // level
    ).Map(func(values []interface{}) *PlayerState {
        return &PlayerState{
            UserId:  values[0].(int64),
            Money:   values[1].(int64),
            Diamond: values[2].(int64),
            Level:   values[3].(int32),
        }
    })
}
```

**核心属性测试用例:**

| 属性编号 | 测试内容 | 生成器 | 迭代次数 |
|---------|---------|--------|---------|
| Property 6 | 序列化往返一致性 | 随机 Actor 状态 | 100 |
| Property 7 | Redis 键格式 | 随机类型/ID 组合 | 100 |
| Property 16 | 同周期变更合并 | 随机多次状态变更 | 100 |
| Property 25 | Delta 仅含变更字段 | 随机字段修改 | 100 |
| Property 27 | 指数退避重试 | 随机连接失败 | 100 |


### 7.3 集成测试

测试完整的持久化与恢复流程,使用真实 Redis 实例。

```go
func TestIntegration_PersistenceAndRecovery(t *testing.T) {
    // 1. 启动测试 Redis
    redisContainer := startTestRedis(t)
    defer redisContainer.Stop()
    
    // 2. 初始化持久化系统
    config := &PersistenceConfig{
        Enabled:          true,
        FlushIntervalMs:  50,
        Redis: RedisConfig{
            Address: redisContainer.Address(),
        },
    }
    
    middleware := persistence.NewMiddleware(config)
    
    // 3. 创建并操作 Player Actor
    player := &actorPlayer{
        userId: 10001,
        playerData: &PlayerData{
            UserId: 10001,
            Money:  1000,
        },
    }
    
    // 4. 触发持久化
    err := middleware.OnMessageProcessed(player, nil)
    assert.NoError(t, err)
    
    // 5. 等待批量刷新
    time.Sleep(100 * time.Millisecond)
    
    // 6. 创建新 Actor 实例并恢复
    player2 := &actorPlayer{userId: 10001}
    err = middleware.OnActorInit(player2)
    assert.NoError(t, err)
    
    // 7. 验证状态一致
    assert.Equal(t, int64(1000), player2.playerData.Money)
}
```

### 7.4 性能测试

测量不同负载下的持久化延迟和吞吐量。

```go
func BenchmarkPersistence_Throughput(b *testing.B) {
    middleware := setupTestMiddleware()
    
    // 生成 1000 个不同的 Actor
    actors := make([]PersistenceMarker, 1000)
    for i := range actors {
        actors[i] = &actorPlayer{
            userId: int64(i),
            playerData: &PlayerData{Money: 1000},
        }
    }
    
    b.ResetTimer()
    
    // 并发写入
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            actor := actors[rand.Intn(len(actors))]
            middleware.OnMessageProcessed(actor, nil)
        }
    })
    
    // 报告吞吐量
    opsPerSec := float64(b.N) / b.Elapsed().Seconds()
    b.ReportMetric(opsPerSec, "ops/s")
}

func BenchmarkPersistence_Latency(b *testing.B) {
    middleware := setupTestMiddleware()
    actor := &actorPlayer{
        userId: 10001,
        playerData: &PlayerData{Money: 1000},
    }
    
    latencies := make([]time.Duration, b.N)
    
    for i := 0; i < b.N; i++ {
        start := time.Now()
        middleware.OnMessageProcessed(actor, nil)
        latencies[i] = time.Since(start)
    }
    
    // 计算百分位
    sort.Slice(latencies, func(i, j int) bool {
        return latencies[i] < latencies[j]
    })
    
    p50 := latencies[b.N/2]
    p95 := latencies[int(float64(b.N)*0.95)]
    p99 := latencies[int(float64(b.N)*0.99)]
    
    b.Logf("P50: %v, P95: %v, P99: %v", p50, p95, p99)
}
```

**性能目标:**
- 吞吐量: > 10,000 ops/s (单节点)
- P50 延迟: < 15ms (入队到 Redis 写入完成)
- P95 延迟: < 50ms
- P99 延迟: < 100ms

### 7.5 故障注入测试

模拟 Redis 故障场景,验证错误处理和重试机制。

```go
func TestFaultInjection_RedisConnectionFailure(t *testing.T) {
    // 创建故障注入 Redis Store
    faultyStore := &FaultyRedisStore{
        realStore:     realRedisStore,
        failureRate:   0.3, // 30% 失败率
        failureType:   ConnectionError,
    }
    
    middleware := persistence.NewMiddleware(config)
    middleware.store = faultyStore
    
    // 执行 100 次持久化操作
    successCount := 0
    for i := 0; i < 100; i++ {
        err := middleware.OnMessageProcessed(actor, nil)
        if err == nil {
            successCount++
        }
    }
    
    // 验证重试机制有效 (最终成功率应 > 95%)
    assert.Greater(t, successCount, 95)
    
    // 验证失败日志记录
    assert.FileExists(t, "/var/log/persistence/failed_updates.log")
}
```


## 8. 部署与运维

### 8.1 Redis 部署建议

**生产环境配置:**

```redis
# Redis 配置文件 (redis.conf)

# 内存优化
maxmemory 4gb
maxmemory-policy allkeys-lru

# 持久化策略 (同时启用 RDB + AOF)
save 900 1
save 300 10
save 60 10000

appendonly yes
appendfsync everysec

# 性能优化
tcp-backlog 511
timeout 300
tcp-keepalive 60

# 慢查询日志
slowlog-log-slower-than 10000
slowlog-max-len 128
```

**Redis 集群方案 (高可用):**

- **主从复制 + Sentinel**: 自动故障转移,适合中小规模
- **Redis Cluster**: 分片存储,适合大规模和高吞吐场景
- **云托管 Redis**: AWS ElastiCache, 阿里云 Redis 等

### 8.2 监控告警配置

**Prometheus 告警规则:**

```yaml
groups:
  - name: persistence_alerts
    rules:
      # 持久化失败率告警
      - alert: PersistenceHighFailureRate
        expr: rate(persistence_flush_error_total[5m]) > 0.05
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "持久化失败率过高 (> 5%)"
          description: "节点 {{ $labels.instance }} 的持久化失败率为 {{ $value }}"
      
      # 持久化延迟告警
      - alert: PersistenceHighLatency
        expr: persistence_latency_p95_ms > 100
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "持久化延迟过高 (P95 > 100ms)"
          description: "节点 {{ $labels.instance }} 的 P95 延迟为 {{ $value }}ms"
      
      # 队列积压告警
      - alert: PersistenceQueueBacklog
        expr: persistence_queue_length > 1000
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "持久化队列积压严重 (> 1000)"
          description: "节点 {{ $labels.instance }} 的队列长度为 {{ $value }}"
      
      # Redis 连接失败告警
      - alert: RedisConnectionFailure
        expr: rate(persistence_retry_total[5m]) > 0.1
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Redis 连接频繁失败"
          description: "节点 {{ $labels.instance }} Redis 重试率为 {{ $value }}"
```

### 8.3 日志配置

**日志级别策略:**

- **INFO**: 系统启动、配置加载、恢复成功
- **WARN**: 延迟超阈值、重试、队列积压
- **ERROR**: 持久化失败、恢复失败、版本迁移失败

**日志轮转配置 (logrotate):**

```
/var/log/persistence/*.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
    create 0644 app app
}
```

### 8.4 容量规划

**Redis 存储容量估算:**

```
单个 Actor 状态大小: ~1KB (平均)
在线玩家数: 10,000
总存储: 10,000 * 1KB = ~10MB

考虑其他 Actor 类型 (Spin, Room 等):
预估总存储: ~100MB

建议配置: 1GB Redis 内存 (留足缓冲)
```

**网络带宽估算:**

```
持久化频率: 每秒 1,000 次 (峰值)
单次数据量: 1KB
网络带宽: 1,000 * 1KB = ~1MB/s = 8Mbps

建议配置: 100Mbps 网络带宽
```

### 8.5 灾难恢复

**备份策略:**

1. **Redis RDB 快照**: 每小时一次,保留 24 小时
2. **Redis AOF 日志**: 实时追加,每秒 fsync
3. **失败日志备份**: 每天归档到对象存储 (S3/OSS)

**恢复流程:**

1. 从最新 RDB 快照恢复 Redis
2. 重放 AOF 日志到当前时间
3. 重启游戏服务器,触发懒加载
4. 手动重放失败日志 (如有必要)

```bash
# 恢复 Redis 数据
redis-cli --rdb /backup/dump.rdb

# 重放失败日志
./replay_failed_persistence.sh /backup/failed_updates.log
```


## 9. 性能优化建议

### 9.1 序列化优化

- **使用 Protobuf**: 比 JSON 小 3-10 倍,解析速度快 20-100 倍
- **字段裁剪**: 仅持久化必要字段,移除可计算字段
- **压缩大对象**: 对 > 10KB 的状态启用 gzip 压缩

### 9.2 批量优化

- **动态调整批量大小**: 根据队列长度动态调整 (100-1000)
- **Pipeline 优化**: 批量命令使用 Redis Pipeline 减少网络往返
- **异步确认**: 持久化完成异步通知 Actor,不阻塞刷新流程

### 9.3 网络优化

- **连接池**: Redis 客户端使用连接池 (推荐 10-20 连接)
- **本地缓存**: 高频读取的数据启用本地 LRU 缓存
- **就近部署**: Redis 与游戏服务器部署在同一可用区

### 9.4 内存优化

- **增量更新**: 启用增量更新减少序列化开销
- **TTL 策略**: 设置合理 TTL 自动清理过期数据
- **定期清理**: 定期清理离线超过 30 天的 Actor 状态

## 10. 安全考虑

### 10.1 数据加密

- **传输加密**: Redis 连接启用 TLS (Redis 6.0+)
- **存储加密**: 敏感字段 (如货币) 可选择性加密存储
- **密钥管理**: 使用 Vault 等工具管理 Redis 密码

### 10.2 访问控制

- **Redis ACL**: 限制持久化系统的 Redis 权限 (仅 SET/GET)
- **网络隔离**: Redis 仅允许游戏服务器访问,禁止公网暴露
- **审计日志**: 记录所有持久化操作到审计日志

### 10.3 数据完整性

- **CRC 校验**: 可选择性为状态数据添加 CRC 校验码
- **版本号校验**: 恢复时验证版本号,防止数据错乱
- **幂等性保证**: 重复持久化操作不影响数据正确性

## 11. 总结

Actor 异步持久化系统通过中间件形式无缝集成到 Cherry Actor 框架,提供准实时、高性能的状态持久化能力。系统设计充分利用 Actor 模型的单线程特性保证并发安全,采用异步队列和批量刷新机制优化性能,支持懒加载恢复和增量更新减少资源开销。

**核心优势:**
- **低侵入性**: 通过接口标记启用,业务逻辑无需修改
- **高性能**: 端到端延迟 < 100ms,吞吐量 > 10,000 ops/s
- **高可用**: 错误重试、失败日志、灾难恢复机制完善
- **易维护**: 配置驱动、监控完善、工具齐全

**适用场景:**
- 游戏玩家状态持久化
- 游戏房间状态持久化
- 其他需要准实时持久化的 Actor 状态

