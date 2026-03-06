// Package metrics 提供服务端 QPS 和延迟统计功能
// 使用原子操作和分片锁优化高并发性能
package metrics

import (
	"sort"
	"sync"
	"sync/atomic"
	"time"

	cherryFacade "github.com/cherry-game/cherry/facade"
	cherryLogger "github.com/cherry-game/cherry/logger"
)

// RouteMetrics 单个路由的统计数据 (优化版：减少锁竞争)
type RouteMetrics struct {
	// 累计统计 (全部使用原子操作，无锁)
	RequestCount  int64 // 收到的请求数
	ResponseCount int64 // 发送的响应数
	ErrorCount    int64 // 错误数
	TotalLatency  int64 // 总延迟(纳秒)
	MaxLatency    int64 // 最大延迟(纳秒)

	// 滑动窗口 QPS - 请求 (使用原子操作数组，无锁写入)
	windowCounts [60]int64 // 每秒请求数 (原子操作)
	lastClearSec int64     // 上次清理的秒数

	// 滑动窗口 QPS - 响应/处理完成 (用于衡量服务器实际处理能力)
	respWindowCounts [60]int64 // 每秒响应数 (原子操作)

	// 延迟百分位统计 (使用环形缓冲区 + 分片锁)
	latencyRing   [10000]int64 // 环形缓冲区
	latencyIdx    int64        // 当前写入位置 (原子操作)
	latencyCount  int64        // 已写入数量 (原子操作)
	latencyReadMu sync.Mutex   // 仅读取时加锁
}

// NewRouteMetrics 创建路由统计
func NewRouteMetrics() *RouteMetrics {
	return &RouteMetrics{
		lastClearSec: time.Now().Unix(),
	}
}

// RecordRequest 记录请求开始 (无锁，高性能)
func (m *RouteMetrics) RecordRequest() {
	// atomic.AddInt64(&m.RequestCount, 1)

	// // 更新滑动窗口 (无锁)
	// nowSec := time.Now().Unix()
	// idx := nowSec % 60

	// // 清理旧数据 (CAS 操作，避免重复清理)
	// lastClear := atomic.LoadInt64(&m.lastClearSec)
	// if nowSec > lastClear {
	// 	if atomic.CompareAndSwapInt64(&m.lastClearSec, lastClear, nowSec) {
	// 		// 清理过期的槽位
	// 		for sec := lastClear + 1; sec <= nowSec; sec++ {
	// 			clearIdx := sec % 60
	// 			atomic.StoreInt64(&m.windowCounts[clearIdx], 0)
	// 		}
	// 	}
	// }

	// atomic.AddInt64(&m.windowCounts[idx], 1)

	atomic.AddInt64(&m.RequestCount, 1)

	// 直接计算下标并增加，完全不管数据是不是脏的
	// 清理工作完全交给后台协程，相信它
	idx := time.Now().Unix() % 60
	atomic.AddInt64(&m.windowCounts[idx], 1)
}

// RecordResponse 记录响应完成 (无锁写入，高性能)
func (m *RouteMetrics) RecordResponse(latencyNs int64, isError bool) {
	atomic.AddInt64(&m.ResponseCount, 1)
	atomic.AddInt64(&m.TotalLatency, latencyNs)

	if isError {
		atomic.AddInt64(&m.ErrorCount, 1)
	}

	// 更新响应滑动窗口 (用于计算处理完成 QPS)
	idx := time.Now().Unix() % 60
	atomic.AddInt64(&m.respWindowCounts[idx], 1)

	// 更新最大延迟 (CAS)
	for {
		cur := atomic.LoadInt64(&m.MaxLatency)
		if latencyNs <= cur || atomic.CompareAndSwapInt64(&m.MaxLatency, cur, latencyNs) {
			break
		}
	}

	// 记录延迟样本到环形缓冲区 (无锁写入)
	idx = atomic.AddInt64(&m.latencyIdx, 1) - 1
	ringIdx := idx % int64(len(m.latencyRing))
	m.latencyRing[ringIdx] = latencyNs
	atomic.AddInt64(&m.latencyCount, 1)
}

// GetRealtimeQPS 获取实时 QPS (最近 N 秒平均，不含当前秒)
func (m *RouteMetrics) GetRealtimeQPS(windowSize int) float64 {
	if windowSize > 60 {
		windowSize = 60
	}

	nowSec := time.Now().Unix()
	var total int64

	for i := 1; i <= windowSize; i++ {
		sec := nowSec - int64(i)
		idx := sec % 60
		total += atomic.LoadInt64(&m.windowCounts[idx])
	}

	return float64(total) / float64(windowSize)
}

// GetLastSecondQPS 获取上一秒的 QPS
func (m *RouteMetrics) GetLastSecondQPS() float64 {
	lastSec := time.Now().Unix() - 1
	idx := lastSec % 60
	return float64(atomic.LoadInt64(&m.windowCounts[idx]))
}

// GetResponseRealtimeQPS 获取响应/处理完成的实时 QPS (最近 N 秒平均)
func (m *RouteMetrics) GetResponseRealtimeQPS(windowSize int) float64 {
	if windowSize > 60 {
		windowSize = 60
	}

	nowSec := time.Now().Unix()
	var total int64

	for i := 1; i <= windowSize; i++ {
		sec := nowSec - int64(i)
		idx := sec % 60
		total += atomic.LoadInt64(&m.respWindowCounts[idx])
	}

	return float64(total) / float64(windowSize)
}

// GetLastSecondResponseQPS 获取上一秒的响应/处理完成 QPS
func (m *RouteMetrics) GetLastSecondResponseQPS() float64 {
	lastSec := time.Now().Unix() - 1
	idx := lastSec % 60
	return float64(atomic.LoadInt64(&m.respWindowCounts[idx]))
}

// GetPercentiles 获取延迟百分位 (P50, P90, P95, P99, Max)
// 仅在读取时加锁，不影响写入性能
func (m *RouteMetrics) GetPercentiles() (p50, p90, p95, p99, max int64) {
	m.latencyReadMu.Lock()
	defer m.latencyReadMu.Unlock()

	count := atomic.LoadInt64(&m.latencyCount)
	if count == 0 {
		return
	}

	// 确定要读取的样本数量
	ringSize := int64(len(m.latencyRing))
	sampleCount := count
	if sampleCount > ringSize {
		sampleCount = ringSize
	}

	// 复制样本
	samples := make([]int64, sampleCount)
	startIdx := atomic.LoadInt64(&m.latencyIdx) - sampleCount
	if startIdx < 0 {
		startIdx = 0
	}

	for i := int64(0); i < sampleCount; i++ {
		ringIdx := (startIdx + i) % ringSize
		samples[i] = m.latencyRing[ringIdx]
	}

	// 排序计算百分位
	sort.Slice(samples, func(i, j int) bool { return samples[i] < samples[j] })

	n := len(samples)
	if n == 0 {
		return
	}

	p50 = samples[int(float64(n-1)*0.50)]
	p90 = samples[int(float64(n-1)*0.90)]
	p95 = samples[int(float64(n-1)*0.95)]
	p99 = samples[int(float64(n-1)*0.99)]
	max = samples[n-1]
	return
}

// GetStats 获取基础统计 (全部原子读取，无锁)
func (m *RouteMetrics) GetStats() (requests, responses, errors int64, avgLatencyMs, maxLatencyMs float64) {
	requests = atomic.LoadInt64(&m.RequestCount)
	responses = atomic.LoadInt64(&m.ResponseCount)
	errors = atomic.LoadInt64(&m.ErrorCount)
	maxLatencyMs = float64(atomic.LoadInt64(&m.MaxLatency)) / 1e6

	if responses > 0 {
		avgLatencyMs = float64(atomic.LoadInt64(&m.TotalLatency)) / float64(responses) / 1e6
	}
	return
}

// Component 服务端 QPS 统计组件
type Component struct {
	cherryFacade.Component

	// 使用 sync.Map 替代 map + RWMutex，减少锁竞争
	routeMetrics sync.Map // map[string]*RouteMetrics

	printInterval time.Duration
	stopChan      chan struct{}
	nodeName      string
}

// New 创建 Metrics 组件
func New() *Component {
	return &Component{
		printInterval: 5 * time.Second,
		stopChan:      make(chan struct{}),
	}
}

// NewWithInterval 创建带自定义打印间隔的 Metrics 组件
func NewWithInterval(interval time.Duration) *Component {
	c := New()
	c.printInterval = interval
	return c
}

func (c *Component) Name() string {
	return "server_metrics_component"
}

func (c *Component) OnAfterInit() {
	c.nodeName = c.App().NodeID()
	go c.printLoop()
	// 【新增】启动清理协程
	go c.cleanerLoop()
	cherryLogger.Warnf("[Metrics] Component started for node: %s, print interval: %v (lock-free optimized)", c.nodeName, c.printInterval)
}

func (c *Component) OnStop() {
	close(c.stopChan)
	cherryLogger.Warnf("[Metrics] Component stopped for node: %s", c.nodeName)
}

// GetRouteMetrics 获取或创建路由统计 (使用 sync.Map，高并发友好)
func (c *Component) GetRouteMetrics(route string) *RouteMetrics {
	if m, ok := c.routeMetrics.Load(route); ok {
		return m.(*RouteMetrics)
	}

	// 使用 LoadOrStore 避免重复创建
	newMetrics := NewRouteMetrics()
	actual, _ := c.routeMetrics.LoadOrStore(route, newMetrics)
	return actual.(*RouteMetrics)
}

// RecordRequest 记录请求 (在处理开始时调用)
func (c *Component) RecordRequest(route string) {
	c.GetRouteMetrics(route).RecordRequest()
}

// RecordResponse 记录响应 (在处理结束时调用)
func (c *Component) RecordResponse(route string, startTime time.Time, isError bool) {
	latencyNs := time.Since(startTime).Nanoseconds()
	c.GetRouteMetrics(route).RecordResponse(latencyNs, isError)
}

// printLoop 定时打印统计
func (c *Component) printLoop() {
	ticker := time.NewTicker(c.printInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.PrintMetrics()
		}
	}
}

// PrintMetrics 打印所有路由统计
func (c *Component) PrintMetrics() {
	// 收集所有路由
	routes := make([]string, 0)
	c.routeMetrics.Range(func(key, value any) bool {
		routes = append(routes, key.(string))
		return true
	})

	if len(routes) == 0 {
		return
	}

	sort.Strings(routes)

	cherryLogger.Warnf("[Metrics] ========== %s Server QPS ==========", c.nodeName)

	var totalReq, totalResp, totalErr int64
	var totalReqQPS, totalRespQPS float64

	for _, route := range routes {
		if m, ok := c.routeMetrics.Load(route); ok {
			rm := m.(*RouteMetrics)
			req, resp, errs, avgMs, maxMs := rm.GetStats()
			if req == 0 {
				continue
			}

			// 请求 QPS
			lastSecQPS := rm.GetLastSecondQPS()
			avg10sQPS := rm.GetRealtimeQPS(10)

			// 响应/处理完成 QPS
			lastSecRespQPS := rm.GetLastSecondResponseQPS()
			avg10sRespQPS := rm.GetResponseRealtimeQPS(10)

			p50, p90, _, p99, _ := rm.GetPercentiles()

			totalReq += req
			totalResp += resp
			totalErr += errs
			totalReqQPS += avg10sQPS
			totalRespQPS += avg10sRespQPS

			cherryLogger.Warnf("  %-35s | ReqQPS: %5.0f/s (10s: %5.1f) | RespQPS: %5.0f/s (10s: %5.1f) | Avg: %5.1fms | P50: %5.1fms P90: %5.1fms P99: %5.1fms | Max: %5.1fms | Req: %6d Resp: %6d Err: %4d",
				route, lastSecQPS, avg10sQPS, lastSecRespQPS, avg10sRespQPS, avgMs,
				float64(p50)/1e6, float64(p90)/1e6, float64(p99)/1e6, maxMs,
				req, resp, errs)
		}
	}

	cherryLogger.Warnf("  [TOTAL] Requests: %d | Responses: %d | Errors: %d | ReqQPS: %.1f/s | RespQPS: %.1f/s",
		totalReq, totalResp, totalErr, totalReqQPS, totalRespQPS)
	cherryLogger.Warnf("[Metrics] ==========================================")
}

// GetAllStats 获取所有路由统计 (用于 API 导出)
func (c *Component) GetAllStats() map[string]RouteStats {
	result := make(map[string]RouteStats)

	c.routeMetrics.Range(func(key, value any) bool {
		route := key.(string)
		m := value.(*RouteMetrics)

		req, resp, errs, avgMs, maxMs := m.GetStats()
		p50, p90, p95, p99, _ := m.GetPercentiles()

		result[route] = RouteStats{
			Route:          route,
			Requests:       req,
			Responses:      resp,
			Errors:         errs,
			AvgLatencyMs:   avgMs,
			MaxLatencyMs:   maxMs,
			P50Ms:          float64(p50) / 1e6,
			P90Ms:          float64(p90) / 1e6,
			P95Ms:          float64(p95) / 1e6,
			P99Ms:          float64(p99) / 1e6,
			LastSecQPS:     m.GetLastSecondQPS(),
			Avg10sQPS:      m.GetRealtimeQPS(10),
			LastSecRespQPS: m.GetLastSecondResponseQPS(),
			Avg10sRespQPS:  m.GetResponseRealtimeQPS(10),
		}
		return true
	})

	return result
}

// RouteStats 路由统计数据结构
type RouteStats struct {
	Route          string  `json:"route"`
	Requests       int64   `json:"requests"`
	Responses      int64   `json:"responses"`
	Errors         int64   `json:"errors"`
	AvgLatencyMs   float64 `json:"avg_latency_ms"`
	MaxLatencyMs   float64 `json:"max_latency_ms"`
	P50Ms          float64 `json:"p50_ms"`
	P90Ms          float64 `json:"p90_ms"`
	P95Ms          float64 `json:"p95_ms"`
	P99Ms          float64 `json:"p99_ms"`
	LastSecQPS     float64 `json:"last_sec_qps"`      // 请求 QPS (上一秒)
	Avg10sQPS      float64 `json:"avg_10s_qps"`       // 请求 QPS (10秒平均)
	LastSecRespQPS float64 `json:"last_sec_resp_qps"` // 响应/处理完成 QPS (上一秒)
	Avg10sRespQPS  float64 `json:"avg_10s_resp_qps"`  // 响应/处理完成 QPS (10秒平均)
}

// Reset 重置所有统计
func (c *Component) Reset() {
	c.routeMetrics = sync.Map{}
	cherryLogger.Warnf("[Metrics] All metrics reset for node: %s", c.nodeName)
}

// cleanerLoop 负责预清理数据
func (c *Component) cleanerLoop() {
	// 设置为 500ms 检查一次，保证精度
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.cleanUpOldMetrics()
		}
	}
}

// cleanUpOldMetrics 执行清理
func (c *Component) cleanUpOldMetrics() {
	now := time.Now().Unix()

	// 我们预清理 "未来" 的槽位
	// 比如现在是第 5 秒，我们把第 6 秒、第 7 秒的数据先清空
	// 这样当时间走到第 6 秒时，那里已经是 0 了

	// 为什么要清理未来几秒？防止 ticker 有轻微延迟
	nextSec := now + 1

	c.routeMetrics.Range(func(key, value any) bool {
		m := value.(*RouteMetrics)

		// 清理下一秒的槽位 (对应的是 59 秒前的旧数据)
		idx := nextSec % 60

		// 直接强制置 0，因为那是下一秒才用的，现在没人写那个位置
		// 即使有极端的时间漂移，偶尔丢一个数也比 Race Condition 强
		atomic.StoreInt64(&m.windowCounts[idx], 0)
		atomic.StoreInt64(&m.respWindowCounts[idx], 0) // 同时清理响应窗口

		// 为了保险，可以多清理一个未来的槽位，比如 nextSec+1
		// 但一般 +1 就够了
		return true
	})
}
