package runtime_monitor

import (
	"github.com/prometheus/client_golang/prometheus"
)

// PrometheusMetrics Prometheus 指标集合
type PrometheusMetrics struct {
	// Goroutine 指标
	goroutines prometheus.Gauge

	// GC 指标
	gcCount     prometheus.Counter
	gcPauseP50  prometheus.Gauge
	gcPauseP90  prometheus.Gauge
	gcPauseP95  prometheus.Gauge
	gcPauseP99  prometheus.Gauge
	gcPauseMax  prometheus.Gauge
	gcPauseAvg  prometheus.Gauge
	gcFrequency prometheus.Gauge

	// 内存指标
	heapAlloc   prometheus.Gauge
	heapInuse   prometheus.Gauge
	heapIdle    prometheus.Gauge
	heapObjects prometheus.Gauge
	heapSys     prometheus.Gauge
	stackInuse  prometheus.Gauge
	stackSys    prometheus.Gauge
	sys         prometheus.Gauge
	mallocs     prometheus.Counter
	frees       prometheus.Counter
	liveObjects prometheus.Gauge

	// 线程指标
	numCPU     prometheus.Gauge
	gomaxprocs prometheus.Gauge
	numCgoCall prometheus.Counter

	// 自定义统计指标
	goroutineGrowthRate prometheus.Gauge
	memoryGrowthRate    prometheus.Gauge
	allocRate           prometheus.Gauge
}

// NewPrometheusMetrics 创建 Prometheus 指标
func NewPrometheusMetrics(namespace, subsystem string) *PrometheusMetrics {
	if namespace == "" {
		namespace = "go"
	}
	if subsystem == "" {
		subsystem = "runtime"
	}

	m := &PrometheusMetrics{
		// Goroutine 指标
		goroutines: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "goroutines",
			Help:      "Number of goroutines that currently exist",
		}),

		// GC 指标
		gcCount: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_count_total",
			Help:      "Total number of GC cycles",
		}),
		gcPauseP50: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_pause_p50_seconds",
			Help:      "GC pause P50 in seconds",
		}),
		gcPauseP90: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_pause_p90_seconds",
			Help:      "GC pause P90 in seconds",
		}),
		gcPauseP95: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_pause_p95_seconds",
			Help:      "GC pause P95 in seconds",
		}),
		gcPauseP99: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_pause_p99_seconds",
			Help:      "GC pause P99 in seconds",
		}),
		gcPauseMax: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_pause_max_seconds",
			Help:      "GC pause max in seconds",
		}),
		gcPauseAvg: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_pause_avg_seconds",
			Help:      "GC pause average in seconds",
		}),
		gcFrequency: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gc_frequency_per_second",
			Help:      "GC frequency (times per second)",
		}),

		// 内存指标
		heapAlloc: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "heap_alloc_bytes",
			Help:      "Bytes of allocated heap objects",
		}),
		heapInuse: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "heap_inuse_bytes",
			Help:      "Bytes in in-use spans",
		}),
		heapIdle: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "heap_idle_bytes",
			Help:      "Bytes in idle spans",
		}),
		heapObjects: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "heap_objects",
			Help:      "Number of allocated heap objects",
		}),
		heapSys: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "heap_sys_bytes",
			Help:      "Bytes of heap memory obtained from the OS",
		}),
		stackInuse: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "stack_inuse_bytes",
			Help:      "Bytes in stack spans",
		}),
		stackSys: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "stack_sys_bytes",
			Help:      "Bytes of stack memory obtained from the OS",
		}),
		sys: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "sys_bytes",
			Help:      "Total bytes of memory obtained from the OS",
		}),
		mallocs: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "mallocs_total",
			Help:      "Total number of mallocs",
		}),
		frees: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "frees_total",
			Help:      "Total number of frees",
		}),
		liveObjects: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "live_objects",
			Help:      "Number of live objects (mallocs - frees)",
		}),

		// 线程指标
		numCPU: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "num_cpu",
			Help:      "Number of logical CPUs",
		}),
		gomaxprocs: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "gomaxprocs",
			Help:      "GOMAXPROCS setting",
		}),
		numCgoCall: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "cgo_calls_total",
			Help:      "Total number of cgo calls",
		}),

		// 自定义统计指标
		goroutineGrowthRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "goroutine_growth_rate",
			Help:      "Goroutine growth rate (5 minutes)",
		}),
		memoryGrowthRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "memory_growth_rate",
			Help:      "Memory growth rate (5 minutes)",
		}),
		allocRate: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: namespace,
			Subsystem: subsystem,
			Name:      "alloc_rate_mb_per_second",
			Help:      "Memory allocation rate (MB/s)",
		}),
	}

	return m
}

// Register 注册所有指标到 Prometheus
func (m *PrometheusMetrics) Register(registry *prometheus.Registry) error {
	collectors := []prometheus.Collector{
		m.goroutines,
		m.gcCount,
		m.gcPauseP50,
		m.gcPauseP90,
		m.gcPauseP95,
		m.gcPauseP99,
		m.gcPauseMax,
		m.gcPauseAvg,
		m.gcFrequency,
		m.heapAlloc,
		m.heapInuse,
		m.heapIdle,
		m.heapObjects,
		m.heapSys,
		m.stackInuse,
		m.stackSys,
		m.sys,
		m.mallocs,
		m.frees,
		m.liveObjects,
		m.numCPU,
		m.gomaxprocs,
		m.numCgoCall,
		m.goroutineGrowthRate,
		m.memoryGrowthRate,
		m.allocRate,
	}

	for _, collector := range collectors {
		if err := registry.Register(collector); err != nil {
			return err
		}
	}

	return nil
}

// Update 更新所有指标
func (m *PrometheusMetrics) Update(collector *Collector) {
	current := collector.GetCurrent()
	if current == nil {
		return
	}

	// 更新 Goroutine 指标
	m.goroutines.Set(float64(current.NumGoroutine))

	// 更新 GC 指标
	gcStats := collector.GetGCStats()
	m.gcCount.Add(float64(gcStats.NumGC))
	m.gcPauseP50.Set(float64(gcStats.PauseP50Ns) / 1e9)
	m.gcPauseP90.Set(float64(gcStats.PauseP90Ns) / 1e9)
	m.gcPauseP95.Set(float64(gcStats.PauseP95Ns) / 1e9)
	m.gcPauseP99.Set(float64(gcStats.PauseP99Ns) / 1e9)
	m.gcPauseMax.Set(float64(gcStats.PauseMaxNs) / 1e9)
	m.gcPauseAvg.Set(float64(gcStats.PauseAvgNs) / 1e9)
	m.gcFrequency.Set(gcStats.GCFrequency)

	// 更新内存指标
	m.heapAlloc.Set(float64(current.HeapAlloc))
	m.heapInuse.Set(float64(current.HeapInuse))
	m.heapIdle.Set(float64(current.HeapIdle))
	m.heapObjects.Set(float64(current.HeapObjects))
	m.heapSys.Set(float64(current.HeapSys))
	m.stackInuse.Set(float64(current.StackInuse))
	m.stackSys.Set(float64(current.StackSys))
	m.sys.Set(float64(current.Sys))
	m.mallocs.Add(float64(current.Mallocs))
	m.frees.Add(float64(current.Frees))
	m.liveObjects.Set(float64(current.LiveObjects))

	// 更新线程指标
	m.numCPU.Set(float64(current.NumCPU))
	m.gomaxprocs.Set(float64(current.GOMAXPROCS))
	m.numCgoCall.Add(float64(current.NumCgoCall))

	// 更新自定义统计指标
	goroutineStats := collector.GetGoroutineStats()
	m.goroutineGrowthRate.Set(goroutineStats.GrowthRate)

	memoryStats := collector.GetMemoryStats()
	m.memoryGrowthRate.Set(memoryStats.GrowthRate)
	m.allocRate.Set(memoryStats.AllocRate)
}
