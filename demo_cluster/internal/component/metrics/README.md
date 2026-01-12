# 服务端 QPS 统计组件

## 功能

- 每个路由独立统计 QPS、延迟、错误率
- 滑动窗口实时 QPS (10秒平均)
- 延迟百分位统计 (P50, P90, P95, P99)
- 定时打印统计日志
- 支持 JSON 导出

## 集成方式

### 1. 在节点中注册组件

```go
// demo_cluster/nodes/game/game.go
import (
    "github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
)

func Run(profileFilePath, nodeID string) {
    app := cherry.Configure(profileFilePath, nodeID, true, cherry.Cluster)
    
    // 注册 metrics 组件
    metricsComponent := metrics.New()
    app.Register(metricsComponent)
    metrics.SetGlobal(metricsComponent) // 设置全局访问
    
    // ... 其他组件
    app.Startup()
}
```

### 2. 在 Actor 中使用

#### 方式一：使用 TrackRequest (推荐)

```go
import "github.com/cherry-game/examples/demo_cluster/internal/component/metrics"

func (p *actorPlayer) playerEnter(session *cproto.Session, req *pb.Int64) {
    // 自动记录请求开始和结束
    done := metrics.TrackRequest("game.player.enter")
    defer func() {
        done(false) // false 表示成功，true 表示错误
    }()
    
    // ... 业务逻辑
}
```

#### 方式二：手动记录

```go
import "github.com/cherry-game/examples/demo_cluster/internal/component/metrics"

func (p *actorPlayer) playerEnter(session *cproto.Session, req *pb.Int64) {
    startTime := time.Now()
    metrics.RecordRequest("game.player.enter")
    
    // ... 业务逻辑
    
    isError := false // 根据实际情况设置
    metrics.RecordResponse("game.player.enter", startTime, isError)
}
```

### 3. 输出示例

```
[Metrics] ========== game-1 Server QPS ==========
  game.player.enter                   | QPS:    120/s (10s:  115.3) | Avg:   8.5ms | P50:   5.2ms P90:  12.3ms P99:  45.6ms | Max:  89.2ms | Req:  12000 Err:    5
  game.player.select                  | QPS:     80/s (10s:   78.2) | Avg:   3.2ms | P50:   2.1ms P90:   5.4ms P99:  12.3ms | Max:  34.5ms | Req:   8000 Err:    0
  game.slots.spin                     | QPS:    500/s (10s:  485.6) | Avg:  15.3ms | P50:  10.2ms P90:  25.4ms P99:  78.9ms | Max: 156.7ms | Req:  50000 Err:   12
  [TOTAL] Requests: 70000 | Responses: 70000 | Errors: 17 | TotalQPS: 679.1/s
[Metrics] ==========================================
```

### 4. API 导出

```go
// 获取所有统计数据 (用于 HTTP API 或监控)
if c := metrics.Global(); c != nil {
    stats := c.GetAllStats()
    // stats 是 map[string]RouteStats
    jsonBytes, _ := json.Marshal(stats)
}
```

### 5. 重置统计

```go
if c := metrics.Global(); c != nil {
    c.Reset()
}
```

## 配置

```go
// 自定义打印间隔 (默认 5 秒)
metricsComponent := metrics.NewWithInterval(10 * time.Second)
```

## 统计指标说明

| 指标 | 说明 |
|------|------|
| QPS | 上一秒的请求数 |
| 10s | 最近 10 秒的平均 QPS |
| Avg | 平均延迟 (毫秒) |
| P50/P90/P99 | 延迟百分位 |
| Max | 最大延迟 |
| Req | 总请求数 |
| Err | 错误数 |
