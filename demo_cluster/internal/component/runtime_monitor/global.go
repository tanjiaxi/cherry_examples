package runtime_monitor

import (
	"sync"
)

// 全局 runtime monitor 实例
var (
	globalMonitor *Component
	globalMu      sync.RWMutex
)

// SetGlobal 设置全局 runtime monitor 实例
func SetGlobal(c *Component) {
	globalMu.Lock()
	globalMonitor = c
	globalMu.Unlock()
}

// Global 获取全局 runtime monitor 实例
func Global() *Component {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalMonitor
}

// GetCollector 获取全局采集器
func GetCollector() *Collector {
	if c := Global(); c != nil {
		return c.GetCollector()
	}
	return nil
}

// GetGoroutineStats 获取 Goroutine 统计 (便捷方法)
func GetGoroutineStats() *GoroutineStats {
	if collector := GetCollector(); collector != nil {
		return collector.GetGoroutineStats()
	}
	return &GoroutineStats{}
}

// GetGCStats 获取 GC 统计 (便捷方法)
func GetGCStats() *GCStats {
	if collector := GetCollector(); collector != nil {
		return collector.GetGCStats()
	}
	return &GCStats{}
}

// GetMemoryStats 获取内存统计 (便捷方法)
func GetMemoryStats() *MemoryStats {
	if collector := GetCollector(); collector != nil {
		return collector.GetMemoryStats()
	}
	return &MemoryStats{}
}

// GetCurrent 获取当前指标快照 (便捷方法)
func GetCurrent() *RuntimeMetrics {
	if collector := GetCollector(); collector != nil {
		return collector.GetCurrent()
	}
	return nil
}
