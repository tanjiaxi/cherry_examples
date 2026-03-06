// Package runtime_monitor 提供 Go Runtime 监控功能
// 监控 Goroutine、GC、内存、线程等关键指标
package runtime_monitor

import (
	"runtime"
	"sort"
	"sync"
	"time"
)

// RuntimeMetrics Go Runtime 指标快照
type RuntimeMetrics struct {
	Timestamp time.Time

	// Goroutine 指标
	NumGoroutine int

	// GC 指标
	NumGC        uint32
	PauseNs      []uint64 // 最近的 GC 停顿时间
	PauseTotalNs uint64
	LastGCTime   time.Time

	// 内存指标
	HeapAlloc   uint64 // 当前堆分配字节数
	HeapInuse   uint64 // 堆正在使用的字节数
	HeapIdle    uint64 // 堆空闲字节数
	HeapObjects uint64 // 堆对象数量
	HeapSys     uint64 // 堆系统字节数
	StackInuse  uint64 // 栈使用字节数
	StackSys    uint64 // 栈系统字节数
	MSpanInuse  uint64
	MSpanSys    uint64
	MCacheInuse uint64
	MCacheSys   uint64
	Sys         uint64 // 系统总内存
	Mallocs     uint64 // 累计分配次数
	Frees       uint64 // 累计释放次数
	LiveObjects uint64 // 当前存活对象数

	// 线程指标
	NumCPU     int
	GOMAXPROCS int
	NumCgoCall int64
}

// Collector Runtime 指标采集器
type Collector struct {
	mu            sync.RWMutex
	current       *RuntimeMetrics
	history       []*RuntimeMetrics // 环形缓冲区
	historySize   int
	historyIndex  int
	collectTicker *time.Ticker
	stopChan      chan struct{}
}

// NewCollector 创建采集器
func NewCollector(historySize int) *Collector {
	if historySize <= 0 {
		historySize = 120 // 默认保存 10 分钟历史 (5秒间隔)
	}

	return &Collector{
		history:     make([]*RuntimeMetrics, historySize),
		historySize: historySize,
		stopChan:    make(chan struct{}),
	}
}

// Start 启动采集
func (c *Collector) Start(interval time.Duration) {
	// 立即采集一次
	c.collect()

	// 启动定时采集
	c.collectTicker = time.NewTicker(interval)
	go c.collectLoop()
}

// Stop 停止采集
func (c *Collector) Stop() {
	if c.collectTicker != nil {
		c.collectTicker.Stop()
	}
	close(c.stopChan)
}

// collectLoop 采集循环
func (c *Collector) collectLoop() {
	for {
		select {
		case <-c.stopChan:
			return
		case <-c.collectTicker.C:
			c.collect()
		}
	}
}

// collect 执行一次采集
func (c *Collector) collect() {
	metrics := &RuntimeMetrics{
		Timestamp: time.Now(),
	}

	// 采集 Goroutine 数量
	metrics.NumGoroutine = runtime.NumGoroutine()

	// 采集内存统计
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	// GC 指标
	metrics.NumGC = ms.NumGC
	metrics.PauseTotalNs = ms.PauseTotalNs
	if ms.NumGC > 0 {
		metrics.LastGCTime = time.Unix(0, int64(ms.LastGC))
	}

	// 提取最近的 GC 停顿时间 (最多 256 个)
	numPauses := int(ms.NumGC)
	if numPauses > 256 {
		numPauses = 256
	}
	metrics.PauseNs = make([]uint64, numPauses)
	for i := 0; i < numPauses; i++ {
		metrics.PauseNs[i] = ms.PauseNs[i]
	}

	// 内存指标
	metrics.HeapAlloc = ms.HeapAlloc
	metrics.HeapInuse = ms.HeapInuse
	metrics.HeapIdle = ms.HeapIdle
	metrics.HeapObjects = ms.HeapObjects
	metrics.HeapSys = ms.HeapSys
	metrics.StackInuse = ms.StackInuse
	metrics.StackSys = ms.StackSys
	metrics.MSpanInuse = ms.MSpanInuse
	metrics.MSpanSys = ms.MSpanSys
	metrics.MCacheInuse = ms.MCacheInuse
	metrics.MCacheSys = ms.MCacheSys
	metrics.Sys = ms.Sys
	metrics.Mallocs = ms.Mallocs
	metrics.Frees = ms.Frees
	metrics.LiveObjects = ms.Mallocs - ms.Frees

	// 线程指标
	metrics.NumCPU = runtime.NumCPU()
	metrics.GOMAXPROCS = runtime.GOMAXPROCS(0)
	metrics.NumCgoCall = runtime.NumCgoCall()

	// 保存到历史记录
	c.mu.Lock()
	c.current = metrics
	c.history[c.historyIndex] = metrics
	c.historyIndex = (c.historyIndex + 1) % c.historySize
	c.mu.Unlock()
}

// GetCurrent 获取当前指标
func (c *Collector) GetCurrent() *RuntimeMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// GetHistory 获取历史记录 (按时间顺序)
func (c *Collector) GetHistory(count int) []*RuntimeMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if count <= 0 || count > c.historySize {
		count = c.historySize
	}

	result := make([]*RuntimeMetrics, 0, count)
	for i := 0; i < count; i++ {
		idx := (c.historyIndex - count + i + c.historySize) % c.historySize
		if c.history[idx] != nil {
			result = append(result, c.history[idx])
		}
	}

	return result
}

// GCStats GC 统计信息
type GCStats struct {
	NumGC        uint32
	PauseP50Ns   uint64
	PauseP90Ns   uint64
	PauseP95Ns   uint64
	PauseP99Ns   uint64
	PauseMaxNs   uint64
	PauseAvgNs   uint64
	PauseTotalNs uint64
	LastGCTime   time.Time
	GCFrequency  float64 // GC 频率 (次/秒)
}

// GetGCStats 计算 GC 统计信息
func (c *Collector) GetGCStats() *GCStats {
	current := c.GetCurrent()
	if current == nil || len(current.PauseNs) == 0 {
		return &GCStats{}
	}

	// 复制并排序停顿时间
	pauses := make([]uint64, len(current.PauseNs))
	copy(pauses, current.PauseNs)
	sort.Slice(pauses, func(i, j int) bool {
		return pauses[i] < pauses[j]
	})

	n := len(pauses)
	stats := &GCStats{
		NumGC:        current.NumGC,
		PauseTotalNs: current.PauseTotalNs,
		LastGCTime:   current.LastGCTime,
	}

	// 计算百分位
	if n > 0 {
		stats.PauseP50Ns = pauses[int(float64(n-1)*0.50)]
		stats.PauseP90Ns = pauses[int(float64(n-1)*0.90)]
		stats.PauseP95Ns = pauses[int(float64(n-1)*0.95)]
		stats.PauseP99Ns = pauses[int(float64(n-1)*0.99)]
		stats.PauseMaxNs = pauses[n-1]
		stats.PauseAvgNs = current.PauseTotalNs / uint64(current.NumGC)
	}

	// 计算 GC 频率 (基于历史数据)
	history := c.GetHistory(12) // 最近 1 分钟 (5秒间隔)
	if len(history) >= 2 {
		first := history[0]
		last := history[len(history)-1]
		duration := last.Timestamp.Sub(first.Timestamp).Seconds()
		if duration > 0 {
			gcCount := last.NumGC - first.NumGC
			stats.GCFrequency = float64(gcCount) / duration
		}
	}

	return stats
}

// MemoryStats 内存统计信息
type MemoryStats struct {
	HeapAllocMB  float64
	HeapInuseMB  float64
	HeapIdleMB   float64
	HeapObjects  uint64
	LiveObjects  uint64
	SysMB        float64
	StackInuseMB float64
	AllocRate    float64 // 分配速率 (MB/s)
	GrowthRate   float64 // 内存增长率 (最近 5 分钟)
}

// GetMemoryStats 获取内存统计
func (c *Collector) GetMemoryStats() *MemoryStats {
	current := c.GetCurrent()
	if current == nil {
		return &MemoryStats{}
	}

	stats := &MemoryStats{
		HeapAllocMB:  float64(current.HeapAlloc) / 1024 / 1024,
		HeapInuseMB:  float64(current.HeapInuse) / 1024 / 1024,
		HeapIdleMB:   float64(current.HeapIdle) / 1024 / 1024,
		HeapObjects:  current.HeapObjects,
		LiveObjects:  current.LiveObjects,
		SysMB:        float64(current.Sys) / 1024 / 1024,
		StackInuseMB: float64(current.StackInuse) / 1024 / 1024,
	}

	// 计算分配速率 (基于最近 1 分钟)
	history := c.GetHistory(12) // 1 分钟
	if len(history) >= 2 {
		first := history[0]
		last := history[len(history)-1]
		duration := last.Timestamp.Sub(first.Timestamp).Seconds()
		if duration > 0 {
			allocDiff := float64(last.Mallocs - first.Mallocs)
			stats.AllocRate = allocDiff / duration / 1024 / 1024
		}
	}

	// 计算内存增长率 (最近 5 分钟)
	history5m := c.GetHistory(60) // 5 分钟
	if len(history5m) >= 2 {
		first := history5m[0]
		last := history5m[len(history5m)-1]
		if first.HeapInuse > 0 {
			stats.GrowthRate = float64(last.HeapInuse-first.HeapInuse) / float64(first.HeapInuse)
		}
	}

	return stats
}

// GoroutineStats Goroutine 统计信息
type GoroutineStats struct {
	Current    int
	Baseline   int     // 基线值 (启动后稳定值)
	Peak       int     // 峰值
	GrowthRate float64 // 增长率 (最近 5 分钟)
	Trend      string  // 趋势: "stable", "increasing", "decreasing"
}

// GetGoroutineStats 获取 Goroutine 统计
func (c *Collector) GetGoroutineStats() *GoroutineStats {
	current := c.GetCurrent()
	if current == nil {
		return &GoroutineStats{}
	}

	stats := &GoroutineStats{
		Current: current.NumGoroutine,
	}

	history := c.GetHistory(c.historySize)
	if len(history) == 0 {
		return stats
	}

	// 计算基线值 (前 10% 的平均值)
	baselineCount := len(history) / 10
	if baselineCount < 1 {
		baselineCount = 1
	}
	var baselineSum int
	for i := 0; i < baselineCount && i < len(history); i++ {
		baselineSum += history[i].NumGoroutine
	}
	stats.Baseline = baselineSum / baselineCount

	// 计算峰值
	for _, m := range history {
		if m.NumGoroutine > stats.Peak {
			stats.Peak = m.NumGoroutine
		}
	}

	// 计算增长率 (最近 5 分钟)
	history5m := c.GetHistory(60)
	if len(history5m) >= 2 {
		first := history5m[0]
		last := history5m[len(history5m)-1]
		if first.NumGoroutine > 0 {
			stats.GrowthRate = float64(last.NumGoroutine-first.NumGoroutine) / float64(first.NumGoroutine)
		}
	}

	// 判断趋势 (基于最近 10 个数据点)
	recentCount := 10
	if len(history) < recentCount {
		recentCount = len(history)
	}
	if recentCount >= 2 {
		recent := history[len(history)-recentCount:]
		first := recent[0].NumGoroutine
		last := recent[len(recent)-1].NumGoroutine
		diff := float64(last-first) / float64(first)

		if diff > 0.1 {
			stats.Trend = "increasing"
		} else if diff < -0.1 {
			stats.Trend = "decreasing"
		} else {
			stats.Trend = "stable"
		}
	}

	return stats
}
