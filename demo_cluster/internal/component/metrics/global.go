/*
 * @Author: t 921865806@qq.com
 * @Date: 2026-01-11 22:43:02
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-01-12 16:32:00
 * @FilePath: /examples/demo_cluster/internal/component/metrics/global.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package metrics

import (
	"sync"
	"time"
)

// 全局 metrics 实例 (用于在 Actor 中方便访问)
var (
	globalMetrics *Component
	globalMu      sync.RWMutex
)

// SetGlobal 设置全局 metrics 实例
func SetGlobal(c *Component) {
	globalMu.Lock()
	globalMetrics = c
	globalMu.Unlock()
}

// Global 获取全局 metrics 实例
func Global() *Component {
	globalMu.RLock()
	defer globalMu.RUnlock()
	return globalMetrics
}

// RecordRequest 全局记录请求 (便捷方法)
func RecordRequest(route string) {
	if c := Global(); c != nil {
		c.RecordRequest(route)
	}
}

// RecordResponse 全局记录响应 (便捷方法)
func RecordResponse(route string, startTime time.Time, isError bool) {
	if c := Global(); c != nil {
		c.RecordResponse(route, startTime, isError)
	}
}

// TrackRequest 返回一个用于追踪请求的 helper
// 用法:
//
//	done := metrics.TrackRequest("game.player.spin")
//	defer done(err != nil)
func TrackRequest(route string) func(isError bool) {
	startTime := time.Now()
	RecordRequest(route)
	return func(isError bool) {
		RecordResponse(route, startTime, isError)
	}
}
