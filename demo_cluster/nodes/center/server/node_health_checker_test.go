package server

import (
	"fmt"
	"sync"
	"testing"
)

// TestNodeHealthChecker_ConcurrentAccess 回归测试：验证 heartbeats 这个map在
// 高并发读写下不会触发 Go runtime 的 "concurrent map read and map write" fatal
// panic。这个panic是进程级别的、无法recover的崩溃，一旦在生产环境的心跳上报
// 高峰期触发，就是整个center节点直接掉线。
//
// 必须配合 -race 运行才有意义：即使不加锁，本测试在没有-race的情况下也可能
// "看起来"跑得通过（map的底层实现在小规模、低频次竞争下不一定每次都触发
// fatal），但 `go test -race` 能可靠地检测出未同步的内存访问。
//
//	go test ./demo_cluster/nodes/center/server/... -run NodeHealthChecker -race -v
func TestNodeHealthChecker_ConcurrentAccess(t *testing.T) {
	checker := NewNodeHealthChecker(10)

	const numWriters = 50
	const numReaders = 50
	const numNodes = 20
	const opsPerGoroutine = 200

	nodeIds := make([]string, numNodes)
	for i := 0; i < numNodes; i++ {
		nodeIds[i] = fmt.Sprintf("node-%d", i)
	}

	var wg sync.WaitGroup

	// 高频写入：模拟各连接/RPC goroutine 并发上报心跳
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				checker.UpdateHeartbeat(nodeIds[(idx+j)%numNodes])
			}
		}(i)
	}

	// 高频读取：模拟健康检查定时器 + 查询方并发读取
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine; j++ {
				_ = checker.IsHealthy(nodeIds[(idx+j)%numNodes])
				_ = checker.GetUnhealthyNodes()
				_ = checker.GetHealthyNodes()
				_ = checker.FilterHealthyNodes(nodeIds)
				_, _ = checker.GetLastHeartbeat(nodeIds[(idx+j)%numNodes])
				_ = checker.GetAllHeartbeats()
			}
		}(i)
	}

	// 并发删除/改配置：模拟节点下线、动态调整超时阈值
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < opsPerGoroutine/4; j++ {
				checker.SetTimeout(int64(10 + idx))
				checker.RemoveNode(nodeIds[(idx+j)%numNodes])
			}
		}(i)
	}

	wg.Wait()
}

// BenchmarkNodeHealthChecker_UpdateHeartbeat 量化加锁后的写路径开销。
// 关注 B/op、allocs/op：RWMutex.Lock/Unlock本身不分配堆内存，
// 理论上应该和裸map写入的分配量几乎一致（差异主要来自函数调用开销）。
func BenchmarkNodeHealthChecker_UpdateHeartbeat(b *testing.B) {
	checker := NewNodeHealthChecker(10)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.UpdateHeartbeat("bench-node")
	}
}

// BenchmarkNodeHealthChecker_MixedReadWrite 模拟真实场景：
// 大量goroutine高频上报心跳（写），同时健康检查定时器周期性地
// 全量扫描（读，代价与节点数成正比）。用于观察 RWMutex 在
// "写多、读的单次代价也不小"场景下的实际并发吞吐。
func BenchmarkNodeHealthChecker_MixedReadWrite(b *testing.B) {
	checker := NewNodeHealthChecker(10)
	for i := 0; i < 100; i++ {
		checker.UpdateHeartbeat(fmt.Sprintf("node-%d", i))
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%10 == 0 {
				_ = checker.GetUnhealthyNodes()
			} else {
				checker.UpdateHeartbeat(fmt.Sprintf("node-%d", i%100))
			}
			i++
		}
	})
}
