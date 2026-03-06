package runtime_monitor

import (
	"fmt"
	"sync"
	"time"

	cherryLogger "github.com/cherry-game/cherry/logger"
)

// AlertLevel 告警级别
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "INFO"
	AlertLevelWarning  AlertLevel = "WARNING"
	AlertLevelCritical AlertLevel = "CRITICAL"
)

// AlertRule 告警规则
type AlertRule struct {
	Name        string
	Description string
	Level       AlertLevel
	CheckFunc   func(*Collector) (bool, string) // 返回 (是否触发, 详细信息)
	Cooldown    time.Duration                   // 冷却期
	lastAlert   time.Time
	mu          sync.Mutex
}

// ShouldAlert 检查是否应该告警 (考虑冷却期)
func (r *AlertRule) ShouldAlert() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	if now.Sub(r.lastAlert) < r.Cooldown {
		return false
	}

	r.lastAlert = now
	return true
}

// AlertEngine 告警引擎
type AlertEngine struct {
	rules     []*AlertRule
	collector *Collector
	enabled   bool
	mu        sync.RWMutex
}

// NewAlertEngine 创建告警引擎
func NewAlertEngine(collector *Collector) *AlertEngine {
	engine := &AlertEngine{
		collector: collector,
		enabled:   true,
		rules:     make([]*AlertRule, 0),
	}

	// 注册默认告警规则
	engine.registerDefaultRules()

	return engine
}

// registerDefaultRules 注册默认告警规则
func (e *AlertEngine) registerDefaultRules() {
	// 1. Goroutine 泄露告警
	e.AddRule(&AlertRule{
		Name:        "goroutine_leak",
		Description: "Goroutine 持续增长不回落",
		Level:       AlertLevelCritical,
		Cooldown:    5 * time.Minute,
		CheckFunc: func(c *Collector) (bool, string) {
			stats := c.GetGoroutineStats()
			// 增长率超过 20% 且趋势为持续增长
			if stats.GrowthRate > 0.2 && stats.Trend == "increasing" {
				return true, fmt.Sprintf("Goroutine 数量持续增长: 当前=%d, 基线=%d, 增长率=%.1f%%, 峰值=%d",
					stats.Current, stats.Baseline, stats.GrowthRate*100, stats.Peak)
			}
			return false, ""
		},
	})

	// 2. Goroutine 数量过高告警
	e.AddRule(&AlertRule{
		Name:        "goroutine_high",
		Description: "Goroutine 数量过高",
		Level:       AlertLevelWarning,
		Cooldown:    10 * time.Minute,
		CheckFunc: func(c *Collector) (bool, string) {
			stats := c.GetGoroutineStats()
			// 超过基线 2 倍
			if stats.Current > stats.Baseline*2 && stats.Baseline > 0 {
				return true, fmt.Sprintf("Goroutine 数量过高: 当前=%d, 基线=%d (%.1f倍)",
					stats.Current, stats.Baseline, float64(stats.Current)/float64(stats.Baseline))
			}
			return false, ""
		},
	})

	// 3. GC 停顿时间过长告警
	e.AddRule(&AlertRule{
		Name:        "gc_pause_high",
		Description: "GC 停顿时间过长",
		Level:       AlertLevelCritical,
		Cooldown:    5 * time.Minute,
		CheckFunc: func(c *Collector) (bool, string) {
			gcStats := c.GetGCStats()
			// P99 停顿超过 50ms
			p99Ms := float64(gcStats.PauseP99Ns) / 1e6
			if p99Ms > 50 {
				return true, fmt.Sprintf("GC P99 停顿时间过长: %.2fms (阈值: 50ms), P95=%.2fms, 平均=%.2fms",
					p99Ms, float64(gcStats.PauseP95Ns)/1e6, float64(gcStats.PauseAvgNs)/1e6)
			}
			return false, ""
		},
	})

	// 4. GC 停顿时间警告
	e.AddRule(&AlertRule{
		Name:        "gc_pause_warning",
		Description: "GC 停顿时间较长",
		Level:       AlertLevelWarning,
		Cooldown:    10 * time.Minute,
		CheckFunc: func(c *Collector) (bool, string) {
			gcStats := c.GetGCStats()
			// P99 停顿超过 10ms
			p99Ms := float64(gcStats.PauseP99Ns) / 1e6
			if p99Ms > 10 && p99Ms <= 50 {
				return true, fmt.Sprintf("GC P99 停顿时间较长: %.2fms (建议: <10ms), 频率=%.2f次/秒",
					p99Ms, gcStats.GCFrequency)
			}
			return false, ""
		},
	})

	// 5. 内存持续增长告警
	e.AddRule(&AlertRule{
		Name:        "memory_leak",
		Description: "内存持续增长",
		Level:       AlertLevelCritical,
		Cooldown:    5 * time.Minute,
		CheckFunc: func(c *Collector) (bool, string) {
			memStats := c.GetMemoryStats()
			// 5 分钟内增长超过 30%
			if memStats.GrowthRate > 0.3 {
				return true, fmt.Sprintf("内存持续增长: HeapInuse=%.2fMB, 增长率=%.1f%%, 分配速率=%.2fMB/s",
					memStats.HeapInuseMB, memStats.GrowthRate*100, memStats.AllocRate)
			}
			return false, ""
		},
	})

	// 6. 内存使用过高告警
	e.AddRule(&AlertRule{
		Name:        "memory_high",
		Description: "内存使用过高",
		Level:       AlertLevelWarning,
		Cooldown:    10 * time.Minute,
		CheckFunc: func(c *Collector) (bool, string) {
			memStats := c.GetMemoryStats()
			// HeapInuse 超过 1GB
			if memStats.HeapInuseMB > 1024 {
				return true, fmt.Sprintf("内存使用过高: HeapInuse=%.2fMB, HeapAlloc=%.2fMB, 对象数=%d",
					memStats.HeapInuseMB, memStats.HeapAllocMB, memStats.HeapObjects)
			}
			return false, ""
		},
	})

	// 7. 对象数量过多告警
	e.AddRule(&AlertRule{
		Name:        "objects_high",
		Description: "堆对象数量过多",
		Level:       AlertLevelWarning,
		Cooldown:    10 * time.Minute,
		CheckFunc: func(c *Collector) (bool, string) {
			memStats := c.GetMemoryStats()
			// 对象数超过 1000 万
			if memStats.HeapObjects > 10000000 {
				return true, fmt.Sprintf("堆对象数量过多: %d (建议使用 sync.Pool 优化)",
					memStats.HeapObjects)
			}
			return false, ""
		},
	})

	// 8. CGO 调用过多告警 (可能导致线程数飙高)
	e.AddRule(&AlertRule{
		Name:        "cgo_calls_high",
		Description: "CGO 调用过多",
		Level:       AlertLevelWarning,
		Cooldown:    10 * time.Minute,
		CheckFunc: func(c *Collector) (bool, string) {
			current := c.GetCurrent()
			if current == nil {
				return false, ""
			}

			// 检查 CGO 调用增长率
			history := c.GetHistory(12) // 1 分钟
			if len(history) >= 2 {
				first := history[0]
				last := history[len(history)-1]
				duration := last.Timestamp.Sub(first.Timestamp).Seconds()
				if duration > 0 {
					cgoDiff := last.NumCgoCall - first.NumCgoCall
					cgoRate := float64(cgoDiff) / duration
					// 每秒超过 1000 次 CGO 调用
					if cgoRate > 1000 {
						return true, fmt.Sprintf("CGO 调用频率过高: %.0f次/秒 (可能导致线程数增加)",
							cgoRate)
					}
				}
			}
			return false, ""
		},
	})
}

// AddRule 添加自定义告警规则
func (e *AlertEngine) AddRule(rule *AlertRule) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.rules = append(e.rules, rule)
}

// Enable 启用告警
func (e *AlertEngine) Enable() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = true
}

// Disable 禁用告警
func (e *AlertEngine) Disable() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.enabled = false
}

// Check 检查所有告警规则
func (e *AlertEngine) Check() []Alert {
	e.mu.RLock()
	if !e.enabled {
		e.mu.RUnlock()
		return nil
	}
	rules := e.rules
	e.mu.RUnlock()

	alerts := make([]Alert, 0)

	for _, rule := range rules {
		triggered, message := rule.CheckFunc(e.collector)
		if triggered && rule.ShouldAlert() {
			alert := Alert{
				Rule:      rule.Name,
				Level:     rule.Level,
				Message:   message,
				Timestamp: time.Now(),
			}
			alerts = append(alerts, alert)

			// 记录告警日志
			e.logAlert(alert)
		}
	}

	return alerts
}

// Alert 告警信息
type Alert struct {
	Rule      string
	Level     AlertLevel
	Message   string
	Timestamp time.Time
}

// logAlert 记录告警日志
func (e *AlertEngine) logAlert(alert Alert) {
	logMsg := fmt.Sprintf("[RuntimeMonitor Alert] [%s] %s: %s",
		alert.Level, alert.Rule, alert.Message)

	switch alert.Level {
	case AlertLevelCritical:
		cherryLogger.Errorf(logMsg)
	case AlertLevelWarning:
		cherryLogger.Warnf(logMsg)
	default:
		cherryLogger.Infof(logMsg)
	}
}

// GetRules 获取所有规则
func (e *AlertEngine) GetRules() []*AlertRule {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.rules
}
