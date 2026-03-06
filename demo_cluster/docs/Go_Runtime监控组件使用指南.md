# Go Runtime 监控组件使用指南

## 概述

Go Runtime 监控组件是一个专为游戏服务器设计的运行时监控系统，提供 Goroutine、GC、内存、线程等关键指标的实时监控和告警功能。

## 核心功能

### 1. 四大监控维度

#### Goroutine 监控
- **目的**: 检测 Goroutine 泄露（Go 最常见的内存泄露原因）
- **指标**:
  - 当前数量、基线值、峰值
  - 增长率和趋势分析
  - 泄露检测告警
- **告警阈值**:
  - 增长率 > 20% 且持续增长 → CRITICAL
  - 超过基线 2 倍 → WARNING

#### GC 监控
- **目的**: 确保 GC 停顿不影响游戏帧率
- **指标**:
  - 停顿时间 P50/P90/P95/P99/Max
  - GC 频率（次/秒）
  - GC 次数累计
- **告警阈值**:
  - P99 停顿 > 50ms → CRITICAL（严重影响玩家体验）
  - P99 停顿 > 10ms → WARNING（建议优化）
- **游戏帧率要求**: 20-60 FPS（每帧 16-50ms）

#### 内存监控
- **目的**: 检测内存泄露和优化内存使用
- **指标**:
  - HeapAlloc/HeapInuse/HeapIdle
  - 堆对象数量
  - 内存分配速率（MB/s）
  - 内存增长率
- **告警阈值**:
  - 5分钟增长 > 30% → CRITICAL
  - HeapInuse > 1GB → WARNING
  - 对象数 > 1000万 → WARNING

#### 线程监控
- **目的**: 检测异常的系统线程创建
- **指标**:
  - CPU 核心数
  - GOMAXPROCS 设置
  - CGO 调用统计
- **告警阈值**:
  - CGO 调用 > 1000次/秒 → WARNING
- **常见原因**: 大量 CGO 调用、系统级 I/O 阻塞

## 快速开始

### 1. 在节点中集成

```go
// demo_cluster/nodes/game/game.go
package main

import (
    cherry "github.com/cherry-game/cherry"
    "github.com/cherry-game/examples/demo_cluster/internal/component/runtime_monitor"
)

func Run(profileFilePath, nodeID string) {
    app := cherry.Configure(profileFilePath, nodeID, true, cherry.Cluster)
    
    // 注册 runtime monitor 组件
    runtimeMonitor := runtime_monitor.New()
    app.Register(runtimeMonitor)
    runtime_monitor.SetGlobal(runtimeMonitor)
    
    // 注册其他组件...
    
    app.Startup()
}
```

### 2. 自定义配置

```go
config := &runtime_monitor.Config{
    Enabled:         true,
    CollectInterval: 5 * time.Second,  // 采集间隔
    PrintInterval:   60 * time.Second, // 打印间隔
    HistorySize:     120,               // 历史记录大小（10分钟）
    MetricsPath:     "/metrics",        // Prometheus 路径
    MetricsPort:     9090,              // HTTP 服务端口
    EnableAlert:     true,              // 启用告警
}

runtimeMonitor := runtime_monitor.NewWithConfig(config)
```

### 3. 在代码中使用

```go
import "github.com/cherry-game/examples/demo_cluster/internal/component/runtime_monitor"

// 获取 Goroutine 统计
goroutineStats := runtime_monitor.GetGoroutineStats()
if goroutineStats.Trend == "increasing" {
    log.Warnf("Goroutine 持续增长: %d", goroutineStats.Current)
}

// 获取 GC 统计
gcStats := runtime_monitor.GetGCStats()
if float64(gcStats.PauseP99Ns)/1e6 > 10 {
    log.Warnf("GC P99 停顿过长: %.2fms", float64(gcStats.PauseP99Ns)/1e6)
}

// 获取内存统计
memStats := runtime_monitor.GetMemoryStats()
log.Infof("内存使用: %.2fMB, 增长率: %.1f%%", 
    memStats.HeapInuseMB, memStats.GrowthRate*100)
```

## 监控输出

### 日志输出示例

```
[RuntimeMonitor] ========== game-1 Runtime Stats ==========
  [Goroutine] Current: 1234 | Baseline: 1000 | Peak: 1500 | Growth: 5.2% | Trend: stable
  [GC] Count: 156 | Frequency: 2.50/s | P50: 1.23ms | P90: 3.45ms | P99: 8.67ms | Max: 15.23ms
  [Memory] HeapAlloc: 123.45MB | HeapInuse: 234.56MB | HeapIdle: 45.67MB | Sys: 345.67MB
  [Memory] Objects: 1234567 | LiveObjects: 1234500 | AllocRate: 12.34MB/s | Growth: 2.3%
  [Thread] NumCPU: 8 | GOMAXPROCS: 8 | CgoCalls: 0
[RuntimeMonitor] ==========================================
```

### 告警日志示例

```
[RuntimeMonitor Alert] [CRITICAL] goroutine_leak: Goroutine 数量持续增长: 当前=2500, 基线=1000, 增长率=150.0%, 峰值=2500
[RuntimeMonitor Alert] [WARNING] gc_pause_warning: GC P99 停顿时间较长: 12.34ms (建议: <10ms), 频率=3.50次/秒
[RuntimeMonitor Alert] [CRITICAL] memory_leak: 内存持续增长: HeapInuse=512.00MB, 增长率=35.2%, 分配速率=25.67MB/s
```

## Prometheus + Grafana 集成

### 1. 配置 Prometheus

```yaml
# prometheus.yml
global:
  scrape_interval: 5s

scrape_configs:
  - job_name: 'game-server'
    static_configs:
      - targets: 
          - 'game-node-1:9090'
          - 'game-node-2:9090'
          - 'gate-node-1:9090'
    metrics_path: '/metrics'
```

### 2. 启动 Prometheus

```bash
prometheus --config.file=prometheus.yml
```

### 3. 访问 Prometheus UI

```
http://localhost:9090
```

### 4. 关键 PromQL 查询

```promql
# Goroutine 数量
go_runtime_goroutines

# Goroutine 增长率（5分钟）
rate(go_runtime_goroutines[5m])

# GC P99 停顿时间（毫秒）
go_runtime_gc_pause_p99_seconds * 1000

# GC 频率
go_runtime_gc_frequency_per_second

# 内存使用（MB）
go_runtime_heap_inuse_bytes / 1024 / 1024

# 内存增长率
rate(go_runtime_heap_inuse_bytes[5m])

# 内存分配速率（MB/s）
go_runtime_alloc_rate_mb_per_second
```

### 5. Grafana Dashboard 配置

创建 Dashboard，添加以下面板：

#### 面板 1: Goroutine 监控
```json
{
  "title": "Goroutine 数量",
  "targets": [
    {
      "expr": "go_runtime_goroutines",
      "legendFormat": "{{instance}}"
    }
  ]
}
```

#### 面板 2: GC 停顿时间
```json
{
  "title": "GC 停顿时间",
  "targets": [
    {
      "expr": "go_runtime_gc_pause_p50_seconds * 1000",
      "legendFormat": "P50"
    },
    {
      "expr": "go_runtime_gc_pause_p99_seconds * 1000",
      "legendFormat": "P99"
    }
  ]
}
```

#### 面板 3: 内存使用
```json
{
  "title": "内存使用",
  "targets": [
    {
      "expr": "go_runtime_heap_alloc_bytes / 1024 / 1024",
      "legendFormat": "HeapAlloc"
    },
    {
      "expr": "go_runtime_heap_inuse_bytes / 1024 / 1024",
      "legendFormat": "HeapInuse"
    }
  ]
}
```

## HTTP API 使用

### 启用 HTTP 服务

```go
config := &runtime_monitor.Config{
    MetricsPort: 9090, // 启用 HTTP 服务
}
```

### API 端点

#### 1. 获取完整统计

```bash
curl http://localhost:9090/api/runtime/stats | jq
```

响应示例：
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
    "pause_p99_ns": 8670000,
    "gc_frequency": 2.5
  },
  "memory": {
    "heap_alloc_mb": 123.45,
    "heap_inuse_mb": 234.56,
    "alloc_rate": 12.34,
    "growth_rate": 0.023
  }
}
```

#### 2. 获取 Goroutine 统计

```bash
curl http://localhost:9090/api/runtime/goroutine
```

#### 3. 获取 GC 统计

```bash
curl http://localhost:9090/api/runtime/gc
```

#### 4. 获取内存统计

```bash
curl http://localhost:9090/api/runtime/memory
```

#### 5. 获取告警信息

```bash
curl http://localhost:9090/api/runtime/alerts
```

#### 6. Prometheus Metrics

```bash
curl http://localhost:9090/metrics
```

## 告警配置

### 内置告警规则

| 规则名称 | 级别 | 触发条件 | 冷却期 |
|---------|------|---------|--------|
| goroutine_leak | CRITICAL | 增长率 > 20% 且持续增长 | 5分钟 |
| goroutine_high | WARNING | 超过基线 2 倍 | 10分钟 |
| gc_pause_high | CRITICAL | P99 停顿 > 50ms | 5分钟 |
| gc_pause_warning | WARNING | P99 停顿 > 10ms | 10分钟 |
| memory_leak | CRITICAL | 5分钟增长 > 30% | 5分钟 |
| memory_high | WARNING | HeapInuse > 1GB | 10分钟 |
| objects_high | WARNING | 对象数 > 1000万 | 10分钟 |
| cgo_calls_high | WARNING | CGO 调用 > 1000次/秒 | 10分钟 |

### 自定义告警规则

```go
if monitor := runtime_monitor.Global(); monitor != nil {
    alertEngine := monitor.GetAlertEngine()
    
    alertEngine.AddRule(&runtime_monitor.AlertRule{
        Name:        "custom_memory_alert",
        Description: "自定义内存告警",
        Level:       runtime_monitor.AlertLevelCritical,
        Cooldown:    5 * time.Minute,
        CheckFunc: func(c *runtime_monitor.Collector) (bool, string) {
            memStats := c.GetMemoryStats()
            if memStats.HeapInuseMB > 2048 {
                return true, fmt.Sprintf("内存超过 2GB: %.2fMB", memStats.HeapInuseMB)
            }
            return false, ""
        },
    })
}
```

### Prometheus Alertmanager 集成

```yaml
# alertmanager.yml
groups:
  - name: go_runtime_alerts
    rules:
      - alert: GoroutineLeaking
        expr: go_runtime_goroutine_growth_rate > 0.2
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "Goroutine 泄露检测"
          description: "节点 {{ $labels.instance }} Goroutine 增长率超过 20%"

      - alert: GCPauseHigh
        expr: go_runtime_gc_pause_p99_seconds > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "GC 停顿时间过长"
          description: "节点 {{ $labels.instance }} GC P99 停顿超过 50ms"

      - alert: MemoryLeak
        expr: go_runtime_memory_growth_rate > 0.3
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "内存泄露检测"
          description: "节点 {{ $labels.instance }} 内存增长率超过 30%"
```

## 故障排查

### 1. Goroutine 泄露排查

#### 步骤 1: 确认泄露
```bash
# 查看 Goroutine 统计
curl http://localhost:9090/api/runtime/goroutine
```

#### 步骤 2: 获取 Goroutine Profile
```bash
# 需要启用 pprof
curl http://localhost:6060/debug/pprof/goroutine?debug=2 > goroutine.txt
```

#### 步骤 3: 分析泄露点
```bash
# 查找数量最多的 Goroutine
grep -A 10 "goroutine" goroutine.txt | sort | uniq -c | sort -rn
```

#### 常见泄露原因
1. Channel 未关闭导致接收方永久阻塞
2. Context 未取消导致子协程无法退出
3. Timer/Ticker 未 Stop
4. 死锁或永久等待

### 2. GC 停顿优化

#### 步骤 1: 确认问题
```bash
# 查看 GC 统计
curl http://localhost:9090/api/runtime/gc
```

#### 步骤 2: 启用 GC Trace
```bash
GODEBUG=gctrace=1 ./game-server
```

#### 步骤 3: 分析 GC 日志
```
gc 1 @0.001s 0%: 0.018+1.2+0.017 ms clock, 0.14+0/1.2/3.6+0.13 ms cpu, 4->4->3 MB, 5 MB goal, 8 P
```

#### 优化方案
1. 使用 sync.Pool 减少临时对象分配
2. 调整 GOGC 参数: `debug.SetGCPercent(200)`
3. 减少大对象分配
4. 优化数据结构，减少指针

### 3. 内存泄露排查

#### 步骤 1: 确认泄露
```bash
# 查看内存统计
curl http://localhost:9090/api/runtime/memory
```

#### 步骤 2: 获取 Heap Profile
```bash
curl http://localhost:6060/debug/pprof/heap > heap.prof
```

#### 步骤 3: 分析内存分配
```bash
go tool pprof heap.prof
(pprof) top10
(pprof) list <function_name>
```

#### 常见泄露原因
1. 全局 map 不断增长未清理
2. 闭包引用导致对象无法释放
3. Goroutine 泄露导致内存泄露
4. 缓存未设置过期时间

## 性能影响

### 组件开销

| 项目 | 开销 | 说明 |
|------|------|------|
| 采集频率 | < 1ms | 每 5 秒一次 |
| 内存占用 | ~1MB | 保存 10 分钟历史 |
| CPU 占用 | < 0.1% | 正常情况下 |
| STW 影响 | < 1ms | ReadMemStats 调用 |

### 优化建议

1. **采集间隔**: 建议 ≥ 5 秒，避免频繁调用 ReadMemStats
2. **历史大小**: 根据需求调整，默认 120 个数据点（10分钟）
3. **打印间隔**: 建议 ≥ 60 秒，避免日志过多
4. **HTTP 端口**: 生产环境建议使用独立端口，避免与业务端口冲突

## 最佳实践

### 1. 监控指标解读

#### Goroutine 数量
- **正常**: 基线值 + 在线玩家数 × 每玩家协程数（2-5个）
- **异常**: 持续上升不回落
- **处理**: 使用 pprof 定位泄露点

#### GC 停顿时间
- **优秀**: P99 < 10ms
- **良好**: P99 < 50ms
- **需优化**: P99 > 50ms
- **处理**: 使用 sync.Pool、减少分配、调整 GOGC

#### 内存使用
- **正常**: 稳定或缓慢增长
- **异常**: 快速增长不回落
- **处理**: 使用 pprof heap 分析

### 2. 告警响应流程

1. **收到告警** → 查看告警详情
2. **确认问题** → 使用 API 或 Grafana 查看趋势
3. **定位原因** → 使用 pprof 分析
4. **临时处理** → 重启服务或限流
5. **根本解决** → 修复代码并发布

### 3. 日常运维

1. **每日检查**: 查看 Grafana Dashboard
2. **每周分析**: 分析趋势，提前发现问题
3. **压测验证**: 压测时重点关注 GC 和内存
4. **版本对比**: 新版本上线后对比监控数据

## 总结

Go Runtime 监控组件提供了完整的运行时监控能力，帮助你：

1. **及时发现问题**: 通过告警快速发现 Goroutine 泄露、GC 停顿、内存泄露等问题
2. **分析性能瓶颈**: 通过详细的指标分析系统性能
3. **优化系统**: 基于监控数据进行针对性优化
4. **保障稳定性**: 确保游戏服务器稳定运行

建议在所有生产环境节点上启用此组件，配合 Prometheus + Grafana 实现完整的监控体系。
