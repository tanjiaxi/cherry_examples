# Design Document: 简化版负载测试工具

## Overview

简化版负载测试工具，专注于核心目标：找出服务器最大承载能力，监控满负载时的表现。

直接修改现有 `robot_client/main.go`，添加必要的指标收集和输出功能。

## Architecture

简单的单文件架构，在现有 robot_client 基础上扩展：

```
main.go
├── 配置变量（并发数、批次大小等）
├── 指标收集（原子计数器）
├── 批量启动机器人
├── 实时状态打印
└── 最终汇总输出
```

## Components and Interfaces

### 1. 配置变量

```go
var (
    maxRobotNum   = 1000              // 最大机器人数
    batchSize     = 10                // 每批启动数量
    batchInterval = 2 * time.Second   // 批次间隔
    errorThreshold = 0.1              // 错误率阈值 (10%)
    printInterval = 5 * time.Second   // 状态打印间隔
)
```

### 2. 指标收集（使用 atomic）

```go
var (
    onlineCount    int64  // 当前在线数
    totalRequests  int64  // 总请求数
    successCount   int64  // 成功数
    errorCount     int64  // 错误数
    totalLatencyMs int64  // 总延迟(毫秒)
    maxLatencyMs   int64  // 最大延迟
)
```

### 3. 核心函数

```go
// 批量启动机器人，直到错误率超过阈值
func RunLoadTest()

// 启动单个机器人并记录指标
func RunRobotWithMetrics(...)

// 定时打印当前状态
func PrintStatus()

// 打印最终汇总
func PrintSummary()
```

## Data Models

### 控制台输出格式

实时状态（每5秒）：
```
[10:00:05] Online: 50 | Latency: 45ms | Errors: 2 (0.5%) | Requests: 400
```

最终汇总：
```
========== Load Test Summary ==========
Max Online Users: 150
Total Requests: 3000
Success Rate: 95.5%
Avg Latency: 52ms
Max Latency: 1200ms
Total Errors: 135
========================================
```

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system-essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Error rate calculation is accurate
*For any* error count E and total count T where T > 0, the error rate SHALL equal E / T.
**Validates: Requirements 3.2**

### Property 2: Average latency calculation is correct
*For any* collection of latency values, the average SHALL equal total latency sum divided by count.
**Validates: Requirements 2.2**

## Error Handling

- 连接失败：记录错误，不重试，继续下一个机器人
- 请求超时：记录为错误
- 错误率超过阈值：停止启动新机器人，输出汇总

## Testing Strategy

由于是简化版工具，主要通过实际运行测试验证。

核心逻辑（错误率计算、平均延迟计算）可以通过简单的单元测试验证。

