package runtime_monitor_test

import (
	"fmt"
	"time"

	"github.com/cherry-game/examples/demo_cluster/internal/component/runtime_monitor"
)

// Example_basic 基础使用示例
func Example_basic() {
	// 创建组件
	monitor := runtime_monitor.New()
	runtime_monitor.SetGlobal(monitor)

	// 模拟组件启动 (实际使用中由 Cherry 框架管理)
	monitor.OnAfterInit()

	// 等待采集数据
	time.Sleep(6 * time.Second)

	// 获取 Goroutine 统计
	goroutineStats := runtime_monitor.GetGoroutineStats()
	fmt.Printf("Goroutines: %d (Baseline: %d, Trend: %s)\n",
		goroutineStats.Current, goroutineStats.Baseline, goroutineStats.Trend)

	// 获取 GC 统计
	gcStats := runtime_monitor.GetGCStats()
	fmt.Printf("GC P99: %.2fms, Frequency: %.2f/s\n",
		float64(gcStats.PauseP99Ns)/1e6, gcStats.GCFrequency)

	// 获取内存统计
	memStats := runtime_monitor.GetMemoryStats()
	fmt.Printf("Memory: %.2fMB (Growth: %.1f%%)\n",
		memStats.HeapInuseMB, memStats.GrowthRate*100)
}

// Example_customConfig 自定义配置示例
func Example_customConfig() {
	config := &runtime_monitor.Config{
		Enabled:         true,
		CollectInterval: 5 * time.Second,
		PrintInterval:   30 * time.Second,
		HistorySize:     60,
		MetricsPath:     "/metrics",
		MetricsPort:     9090,
		EnableAlert:     true,
	}

	monitor := runtime_monitor.NewWithConfig(config)
	runtime_monitor.SetGlobal(monitor)

	// 启动组件
	// monitor.OnAfterInit()
	fmt.Println("Runtime monitor started with custom config")
}

// Example_customAlert 自定义告警规则示例
func Example_customAlert() {
	monitor := runtime_monitor.New()
	runtime_monitor.SetGlobal(monitor)

	// 获取告警引擎
	alertEngine := monitor.GetAlertEngine()

	// 添加自定义告警规则
	alertEngine.AddRule(&runtime_monitor.AlertRule{
		Name:        "high_goroutine_count",
		Description: "Goroutine 数量超过 10000",
		Level:       runtime_monitor.AlertLevelCritical,
		Cooldown:    5 * time.Minute,
		CheckFunc: func(c *runtime_monitor.Collector) (bool, string) {
			stats := c.GetGoroutineStats()
			if stats.Current > 10000 {
				return true, fmt.Sprintf("Goroutine 数量过高: %d", stats.Current)
			}
			return false, ""
		},
	})

	fmt.Println("Custom alert rule added")
}

// Example_integration Cherry 框架集成示例
func Example_integration() {
	// 在 Cherry 应用中集成
	/*
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
	*/

	fmt.Println("See code comments for integration example")
}
