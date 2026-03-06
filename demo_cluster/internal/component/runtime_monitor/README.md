# Go Runtime 监控组件

## 功能特性

### 核心监控指标

1. **Goroutine 监控**
   - 当前 Goroutine 数量
   - 基线值、峰值统计
   - 增长率和趋势分析
   - Goroutine 泄露检测

2. **GC 监控**
   - GC 停顿时间 (P50/P90/P95/P99/Max)
   - GC 频率统计
   - GC 次数累计
   - 游戏帧率影响分析

3. **内存监控**
   - 堆内存分配 (HeapAlloc/HeapInuse/HeapIdle)
   - 堆对象数量
   - 内存分配速率
   - 内存增长率
   - 内存泄露检测

4. **线程监控**
   - CPU 核心数
   - GOMAXPROCS 设置
   - CGO 调用统计
   - 系统线程数监控

### 告警功能

内置 8 种告警规则：

| 告警规则 | 级别 | 触发条件 | 冷却期 |
|---------|------|---------|--------|
| goroutine_leak | CRITICAL | 增长率 > 20% 且持续增长 | 5分钟 |
| goroutine_high | WARNING | 超过基线 2 倍 | 10分钟 |
| gc_pause_high | CRITICAL | P99 停顿 > 50ms | 5分钟 |
| gc_pause_warning | WARNING | P99 停顿 > 10ms | 10分钟 |
| memory_leak | CRITICAL | 5分钟增长 > 30% | 5分钟 |
| memory_high | WARNING | HeapInuse > 1GB | 10分钟 |
| objects_high | WARNING | 对象数 > 1000万 | 10分钟 |
| cgo_calls_high | WARNING | CGO 调用 > 1000次/秒 | 10分钟 |

### 数据暴露

1. **Prometheus Metrics**
   - 标准 Prometheus 格式
   - 支持 Grafana 可视化
   - 完整的指标集合

2. **HTTP JSON API**
   - `/api/runtime/stats` - 完整统计
   - `/api/runtime/goroutine` - Goroutine 统计
   - `/api/runtime/gc` - GC 统计
   - `/api/runtime/memory` - 内存统计
   - `/api/runtime/alerts` - 告警信息

3. **日志输出**
   - 定时打印统计信息
   - 告警日志记录

## 快速开始

### 1. 基础集成

```go
// demo_cluster/nodes/game/game.go
import (
    "github.com/cherry-game/examples/demo_cluster/internal/component/runtime_monitor"
)

func Run(profileFilePath, nodeID string) {
    app := cherry.Configure(profileFilePath, nodeID, true, cherry.Cluster)
    
    // 注册 runtime monitor 组件
    runtimeMonitor := runtime_monitor.New()
    app.Register(runtimeMonitor)
    runtime_monitor.SetGlobal(runtimeMonitor) // 设置全局访问
    
    // ... 其他组件
    app.Startup()
}
```

### 2. 自定义配置

```go
config := &runtime_monitor.Config{
    Enabled:         true,
    CollectInterval: 5 * time.Second,  // 采集间隔
    PrintInterval:   60 * time.Second, // 打印间隔
    HistorySize:     120,               // 历史记录大小 (10分钟)
    MetricsPath:     "/metrics",        // Prometheus 路径
    MetricsPort:     9090,              // HTTP 服务端口
    EnableAlert:     true,              // 启用告警
}

runtimeMonitor := runtime_monitor.NewWithConfig(config)
```

### 3. 使用全局 API

```go
import "github.com/cherry-game/examples/demo_cluster/internal/component/runtime_monitor"

// 获取 Goroutine 统计
goroutineStats := runtime_monitor.GetGoroutineStats()
fmt.Printf("Current Goroutines: %d, Trend: %s\n", 
    goroutineStats.Current, goroutineStats.Trend)

// 获取 GC 统计
gcStats := runtime_monitor.GetGCStats()
fmt.Printf("GC P99: %.2fms, Frequency: %.2f/s\n",
    float64(gcStats.PauseP99Ns)/1e6, gcStats.GCFrequency)

// 获取内存统计
memStats := runtime_monitor.GetMemoryStats()
fmt.Printf("HeapInuse: %.2fMB, Growth: %.1f%%\n",
    memStats.HeapInuseMB, memStats.GrowthRate*100)
```

## 输出示例

### 日志输出

```
[RuntimeMonitor] ========== game-1 Runtime Stats ==========
  [Goroutine] Current: 1234 | Baseline: 1000 | Peak: 1500 | Growth: 5.2% | Trend: stable
  [GC] Count: 156 | Frequency: 2.50/s | P50: 1.23ms | P90: 3.45ms | P99: 8.67ms | Max: 15.23ms
  [Memory] HeapAlloc: 123.45MB | HeapInuse: 234.56MB | HeapIdle: 45.67MB | Sys: 345.67MB
  [Memory] Objects: 1234567 | LiveObjects: 1234500 | AllocRate: 12.34MB/s | Growth: 2.3%
  [Thread] NumCPU: 8 | GOMAXPROCS: 8 | CgoCalls: 0
[RuntimeMonitor] ==========================================
```

### 告警日志

```
[RuntimeMonitor Alert] [CRITICAL] goroutine_leak: Goroutine 数量持续增长: 当前=2500, 基线=1000, 增长率=150.0%, 峰值=2500
[RuntimeMonitor Alert] [WARNING] gc_pause_warning: GC P99 停顿时间较长: 12.34ms (建议: <10ms), 频率=3.50次/秒
[RuntimeMonitor Alert] [CRITICAL] memory_leak: 内存持续增长: HeapInuse=512.00MB, 增长率=35.2%, 分配速率=25.67MB/s
```

## Prometheus 集成

### 1. 配置 Prometheus

```yaml
# prometheus.yml
scrape_configs:
  - job_name: 'game-server'
    static_configs:
      - targets: ['localhost:9090']
    scrape_interval: 5s
```

### 2. 可用指标

```
# Goroutine
go_runtime_goroutines
go_runtime_goroutine_growth_rate

# GC
go_runtime_gc_count_total
go_runtime_gc_pause_p50_seconds
go_runtime_gc_pause_p90_seconds
go_runtime_gc_pause_p95_seconds
go_runtime_gc_pause_p99_seconds
go_runtime_gc_pause_max_seconds
go_runtime_gc_pause_avg_seconds
go_runtime_gc_frequency_per_second

# Memory
go_runtime_heap_alloc_bytes
go_runtime_heap_inuse_bytes
go_runtime_heap_idle_bytes
go_runtime_heap_objects
go_runtime_heap_sys_bytes
go_runtime_stack_inuse_bytes
go_runtime_stack_sys_bytes
go_runtime_sys_bytes
go_runtime_mallocs_total
go_runtime_frees_total
go_runtime_live_objects
go_runtime_memory_growth_rate
go_runtime_alloc_rate_mb_per_second

# Thread
go_runtime_num_cpu
go_runtime_gomaxprocs
go_runtime_cgo_calls_total
```

## Grafana Dashboard

### 推荐面板布局

1. **概览面板**
   - Goroutine 数量趋势
   - GC P99 停顿时间
   - 内存使用趋势
   - 告警状态

2. **Goroutine 面板**
   - 当前数量 vs 基线
   - 增长率曲线
   - 趋势分析

3. **GC 面板**
   - 停顿时间分布 (P50/P90/P95/P99)
   - GC 频率
   - GC 次数累计

4. **内存面板**
   - HeapAlloc/HeapInuse/HeapIdle
   - 对象数量
   - 分配速率
   - 增长率

5. **线程面板**
   - GOMAXPROCS
   - CGO 调用频率

### 示例 PromQL 查询

```promql
# Goroutine 增长率
rate(go_runtime_goroutines[5m])

# GC P99 停顿时间 (毫秒)
go_runtime_gc_pause_p99_seconds * 1000

# 内存增长率
rate(go_runtime_heap_inuse_bytes[5m])

# 分配速率 (MB/s)
rate(go_runtime_mallocs_total[1m]) / 1024 / 1024
```

## 告警规则配置

### Prometheus Alertmanager

```yaml
groups:
  - name: go_runtime
    rules:
      - alert: GoroutineLeaking
        expr: go_runtime_goroutine_growth_rate > 0.2
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Goroutine 泄露检测"
          description: "Goroutine 增长率超过 20%"

      - alert: GCPauseHigh
        expr: go_runtime_gc_pause_p99_seconds > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "GC 停顿时间过长"
          description: "GC P99 停顿超过 50ms"

      - alert: MemoryLeak
        expr: go_runtime_memory_growth_rate > 0.3
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "内存泄露检测"
          description: "内存增长率超过 30%"
```

## 自定义告警规则

```go
// 添加自定义告警规则
if monitor := runtime_monitor.Global(); monitor != nil {
    alertEngine := monitor.GetAlertEngine()
    
    alertEngine.AddRule(&runtime_monitor.AlertRule{
        Name:        "custom_goroutine_alert",
        Description: "自定义 Goroutine 告警",
        Level:       runtime_monitor.AlertLevelWarning,
        Cooldown:    5 * time.Minute,
        CheckFunc: func(c *runtime_monitor.Collector) (bool, string) {
            stats := c.GetGoroutineStats()
            if stats.Current > 5000 {
                return true, fmt.Sprintf("Goroutine 超过 5000: %d", stats.Current)
            }
            return false, ""
        },
    })
}
```

## HTTP API 使用

### 获取完整统计

```bash
curl http://localhost:9090/api/runtime/stats | jq
```

```json
{
  "timestamp": "2026-03-04T10:00:00Z",
  "goroutine": {
    "current": 1234,
    "baseline": 1000,
    "peak": 1500,
    "growth_rate": 0.052,
    "trend": "stable"
  },
  "gc": {
    "num_gc": 156,
    "pause_p50_ns": 1230000,
    "pause_p90_ns": 3450000,
    "pause_p99_ns": 8670000,
    "pause_max_ns": 15230000,
    "gc_frequency": 2.5
  },
  "memory": {
    "heap_alloc_mb": 123.45,
    "heap_inuse_mb": 234.56,
    "heap_objects": 1234567,
    "alloc_rate": 12.34,
    "growth_rate": 0.023
  }
}
```

### 获取告警信息

```bash
curl http://localhost:9090/api/runtime/alerts | jq
```

```json
[
  {
    "rule": "gc_pause_warning",
    "level": "WARNING",
    "message": "GC P99 停顿时间较长: 12.34ms",
    "timestamp": "2026-03-04T10:00:00Z"
  }
]
```

## 性能影响

- **采集开销**: < 1ms (每 5 秒一次)
- **内存开销**: ~1MB (保存 10 分钟历史)
- **CPU 开销**: < 0.1% (正常情况)

注意: `runtime.ReadMemStats()` 会触发 STW，建议采集间隔 ≥ 5 秒

## 最佳实践

1. **Goroutine 泄露排查**
   - 监控 Goroutine 增长趋势
   - 使用 pprof 定位泄露点
   - 检查 Channel 是否正确关闭
   - 检查 Context 是否正确取消

2. **GC 优化**
   - P99 停顿 < 10ms 为优秀
   - 使用 sync.Pool 减少分配
   - 调整 GOGC 参数
   - 减少大对象分配

3. **内存优化**
   - 监控内存增长率
   - 使用 pprof heap 分析
   - 优化对象池使用
   - 避免内存泄露

4. **告警配置**
   - 根据业务调整阈值
   - 设置合理的冷却期
   - 配置告警通知渠道

## 故障排查

### Goroutine 泄露

```bash
# 获取 Goroutine profile
curl http://localhost:6060/debug/pprof/goroutine?debug=2 > goroutine.txt

# 分析泄露点
grep -A 10 "goroutine" goroutine.txt
```

### 内存泄露

```bash
# 获取 heap profile
curl http://localhost:6060/debug/pprof/heap > heap.prof

# 使用 pprof 分析
go tool pprof heap.prof
```

### GC 停顿分析

```bash
# 启用 GC trace
GODEBUG=gctrace=1 ./game-server

# 分析 GC 日志
# gc 1 @0.001s 0%: 0.018+1.2+0.017 ms clock, 0.14+0/1.2/3.6+0.13 ms cpu, 4->4->3 MB, 5 MB goal, 8 P
```

## 参考资料

- [Go Runtime Metrics](https://golang.org/pkg/runtime/)
- [Prometheus Go Client](https://github.com/prometheus/client_golang)
- [Go GC Guide](https://tip.golang.org/doc/gc-guide)
- [pprof 使用指南](https://github.com/google/pprof)
