package runtime_monitor

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	cherryFacade "github.com/cherry-game/cherry/facade"
	cherryLogger "github.com/cherry-game/cherry/logger"
	cprofile "github.com/cherry-game/cherry/profile"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Config 组件配置
type Config struct {
	Enabled         bool          `json:"enabled"`          // 是否启用
	CollectInterval time.Duration `json:"collect_interval"` // 采集间隔 (秒)
	PrintInterval   time.Duration `json:"print_interval"`   // 打印间隔 (秒)
	HistorySize     int           `json:"history_size"`     // 历史记录大小
	MetricsPath     string        `json:"metrics_path"`     // Prometheus metrics 路径
	MetricsPort     int           `json:"metrics_port"`     // Prometheus metrics 端口 (0=不启用)
	EnableAlert     bool          `json:"enable_alert"`     // 是否启用告警
}

// DefaultConfig 默认配置
func DefaultConfig() *Config {
	return &Config{
		Enabled:         true,
		CollectInterval: 5 * time.Second,
		PrintInterval:   60 * time.Second,
		HistorySize:     120, // 10 分钟历史 (5秒间隔)
		MetricsPath:     "/metrics",
		MetricsPort:     30010, // 默认不启用独立端口
		EnableAlert:     true,
	}
}
func GetConfig(nodeName string) *Config {
	config := DefaultConfig()
	runtimeMonitorConfig := cprofile.GetConfig("runtime_monitor")
	nodeRuntimeMonitor := runtimeMonitorConfig.GetConfig(nodeName)

	config.CollectInterval = time.Duration(nodeRuntimeMonitor.GetInt("collect_interval")) * time.Second
	config.PrintInterval = time.Duration(nodeRuntimeMonitor.GetInt("print_interval")) * time.Second
	config.HistorySize = nodeRuntimeMonitor.GetInt("history_size")
	config.MetricsPath = nodeRuntimeMonitor.GetString("metrics_path")
	config.MetricsPort = nodeRuntimeMonitor.GetInt("metrics_port")
	config.EnableAlert = nodeRuntimeMonitor.GetBool("enable_alert")
	config.Enabled = nodeRuntimeMonitor.GetBool("enabled")
	return config
}

// Component Runtime 监控组件
type Component struct {
	cherryFacade.Component

	config      *Config
	collector   *Collector
	promMetrics *PrometheusMetrics
	registry    *prometheus.Registry
	alertEngine *AlertEngine
	profileDump *ProfileDumper
	httpServer  *http.Server
	stopChan    chan struct{}
	nodeName    string
}

// New 创建 Runtime 监控组件
func New(nodeName string) *Component {
	return NewWithConfig(GetConfig(nodeName))
}

// NewWithConfig 使用自定义配置创建组件
func NewWithConfig(config *Config) *Component {
	return &Component{
		config:   config,
		stopChan: make(chan struct{}),
	}
}

func (c *Component) Name() string {
	return "runtime_monitor_component"
}

func (c *Component) OnAfterInit() {
	cherryLogger.Warnf("[RuntimeMonitor] OnAfterInit")

	c.nodeName = c.App().NodeID()

	if !c.config.Enabled {
		cherryLogger.Warnf("[RuntimeMonitor] Component disabled")
		return
	}

	// 初始化采集器
	c.collector = NewCollector(c.config.HistorySize)
	c.collector.Start(c.config.CollectInterval)

	// 初始化 Prometheus 指标
	c.registry = prometheus.NewRegistry()
	c.promMetrics = NewPrometheusMetrics("go", "runtime")
	if err := c.promMetrics.Register(c.registry); err != nil {
		cherryLogger.Errorf("[RuntimeMonitor] Failed to register prometheus metrics: %v", err)
	}

	// 初始化告警引擎
	if c.config.EnableAlert {
		c.alertEngine = NewAlertEngine(c.collector)
		// 生产可用的动态采样埋点：告警触发的那一刻自动抓取 heap/goroutine/cpu/trace，
		// 而不是等运维人工登录服务器时现场已经消失（尤其是CPU飙高/GC暂停这类瞬时问题）。
		c.profileDump = NewProfileDumper(fmt.Sprintf("logs/pprof/%s", c.nodeName), 5*time.Minute)
	}

	// 启动定时任务
	go c.updateLoop()
	go c.printLoop()
	if c.config.EnableAlert {
		go c.alertLoop()
	}

	// 启动 HTTP 服务 (如果配置了端口)
	if c.config.MetricsPort > 0 {
		go c.startHTTPServer()
	}

	cherryLogger.Warnf("[RuntimeMonitor] Component started for node: %s, collect_interval=%v, print_interval=%v",
		c.nodeName, c.config.CollectInterval, c.config.PrintInterval)
}

func (c *Component) OnStop() {
	if !c.config.Enabled {
		return
	}

	close(c.stopChan)

	if c.collector != nil {
		c.collector.Stop()
	}

	if c.httpServer != nil {
		c.httpServer.Close()
	}

	cherryLogger.Warnf("[RuntimeMonitor] Component stopped for node: %s", c.nodeName)
}

// updateLoop 定时更新 Prometheus 指标
func (c *Component) updateLoop() {
	ticker := time.NewTicker(c.config.CollectInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.promMetrics.Update(c.collector)
		}
	}
}

// printLoop 定时打印统计信息
func (c *Component) printLoop() {
	ticker := time.NewTicker(c.config.PrintInterval)
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			c.PrintStats()
		}
	}
}

// alertLoop 定时检查告警
func (c *Component) alertLoop() {
	ticker := time.NewTicker(30 * time.Second) // 每 30 秒检查一次
	defer ticker.Stop()

	for {
		select {
		case <-c.stopChan:
			return
		case <-ticker.C:
			alerts := c.alertEngine.Check()
			c.maybeDumpProfiles(alerts)
		}
	}
}

// maybeDumpProfiles 命中 Critical 级别告警时，自动落盘 pprof/trace 快照。
// 具体的抓取频率限流交给 ProfileDumper 的 cooldown 处理，这里只负责"发信号"。
func (c *Component) maybeDumpProfiles(alerts []Alert) {
	if c.profileDump == nil {
		return
	}
	for _, alert := range alerts {
		if alert.Level == AlertLevelCritical {
			c.profileDump.TriggerOnAlert(alert.Rule)
			return
		}
	}
}

// PrintStats 打印统计信息
func (c *Component) PrintStats() {
	current := c.collector.GetCurrent()
	if current == nil {
		return
	}

	cherryLogger.Warnf("[RuntimeMonitor] ========== %s Runtime Stats ==========", c.nodeName)

	// Goroutine 统计
	goroutineStats := c.collector.GetGoroutineStats()
	cherryLogger.Warnf("  [Goroutine] Current: %d | Baseline: %d | Peak: %d | Growth: %.1f%% | Trend: %s",
		goroutineStats.Current, goroutineStats.Baseline, goroutineStats.Peak,
		goroutineStats.GrowthRate*100, goroutineStats.Trend)

	// GC 统计
	gcStats := c.collector.GetGCStats()
	cherryLogger.Warnf("  [GC] Count: %d | Frequency: %.2f/s | P50: %.2fms | P90: %.2fms | P99: %.2fms | Max: %.2fms",
		gcStats.NumGC, gcStats.GCFrequency,
		float64(gcStats.PauseP50Ns)/1e6, float64(gcStats.PauseP90Ns)/1e6,
		float64(gcStats.PauseP99Ns)/1e6, float64(gcStats.PauseMaxNs)/1e6)

	// 内存统计
	memStats := c.collector.GetMemoryStats()
	cherryLogger.Warnf("  [Memory] HeapAlloc: %.2fMB | HeapInuse: %.2fMB | HeapIdle: %.2fMB | Sys: %.2fMB",
		memStats.HeapAllocMB, memStats.HeapInuseMB, memStats.HeapIdleMB, memStats.SysMB)
	cherryLogger.Warnf("  [Memory] Objects: %d | LiveObjects: %d | AllocRate: %.2fMB/s | Growth: %.1f%%",
		memStats.HeapObjects, memStats.LiveObjects, memStats.AllocRate, memStats.GrowthRate*100)

	// 线程统计
	cherryLogger.Warnf("  [Thread] NumCPU: %d | GOMAXPROCS: %d | CgoCalls: %d",
		current.NumCPU, current.GOMAXPROCS, current.NumCgoCall)

	cherryLogger.Warnf("[RuntimeMonitor] ==========================================")
}

// GetCollector 获取采集器 (用于外部访问)
func (c *Component) GetCollector() *Collector {
	return c.collector
}

// GetAlertEngine 获取告警引擎
func (c *Component) GetAlertEngine() *AlertEngine {
	return c.alertEngine
}

// GetRegistry 获取 Prometheus Registry
func (c *Component) GetRegistry() *prometheus.Registry {
	return c.registry
}

// startHTTPServer 启动 HTTP 服务
func (c *Component) startHTTPServer() {
	mux := http.NewServeMux()

	// Prometheus metrics 端点
	mux.Handle(c.config.MetricsPath, promhttp.HandlerFor(c.registry, promhttp.HandlerOpts{}))

	// JSON API 端点
	mux.HandleFunc("/api/runtime/stats", c.handleStats)
	mux.HandleFunc("/api/runtime/goroutine", c.handleGoroutine)
	mux.HandleFunc("/api/runtime/gc", c.handleGC)
	mux.HandleFunc("/api/runtime/memory", c.handleMemory)
	mux.HandleFunc("/api/runtime/alerts", c.handleAlerts)
	mux.HandleFunc("/api/runtime/dump", c.handleDump)

	addr := fmt.Sprintf(":%d", c.config.MetricsPort)
	c.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	cherryLogger.Infof("[RuntimeMonitor] HTTP server started on %s", addr)
	if err := c.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		cherryLogger.Errorf("[RuntimeMonitor] /: %v", err)
	}
}

// HTTP 处理器

func (c *Component) handleStats(w http.ResponseWriter, r *http.Request) {
	current := c.collector.GetCurrent()
	if current == nil {
		http.Error(w, "No data available", http.StatusServiceUnavailable)
		return
	}

	stats := map[string]interface{}{
		"timestamp":  current.Timestamp,
		"goroutine":  c.collector.GetGoroutineStats(),
		"gc":         c.collector.GetGCStats(),
		"memory":     c.collector.GetMemoryStats(),
		"num_cpu":    current.NumCPU,
		"gomaxprocs": current.GOMAXPROCS,
		"cgo_calls":  current.NumCgoCall,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (c *Component) handleGoroutine(w http.ResponseWriter, r *http.Request) {
	stats := c.collector.GetGoroutineStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (c *Component) handleGC(w http.ResponseWriter, r *http.Request) {
	stats := c.collector.GetGCStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (c *Component) handleMemory(w http.ResponseWriter, r *http.Request) {
	stats := c.collector.GetMemoryStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (c *Component) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if c.alertEngine == nil {
		http.Error(w, "Alert engine not enabled", http.StatusServiceUnavailable)
		return
	}

	alerts := c.alertEngine.Check()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(alerts)
}

// handleDump 手动触发一次 heap/goroutine/cpu/trace 抓取，用于线上人工排查
// （CPU profile 默认采样10秒，接口会立即返回，抓取在后台完成）。
// curl -X POST http://<node>:<metrics_port>/api/runtime/dump?reason=manual
func (c *Component) handleDump(w http.ResponseWriter, r *http.Request) {
	if c.profileDump == nil {
		http.Error(w, "profile dumper not enabled", http.StatusServiceUnavailable)
		return
	}
	reason := r.URL.Query().Get("reason")
	if reason == "" {
		reason = "manual"
	}
	c.profileDump.TriggerOnAlert(reason)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "accepted",
		"note":   "capture running in background (cpu profile ~10s, trace ~5s), check logs/pprof/<node>/ dir",
	})
}
