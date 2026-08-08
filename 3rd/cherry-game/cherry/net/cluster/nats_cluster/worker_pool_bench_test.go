package cherryNatsCluster

// 本文件量化 handleConcurrent 里 worker池获取逻辑"修复前 vs 修复后"的
// 内存分配差异。
//
// 之所以不直接调用 handleConcurrent 本身来做benchmark：它依赖真实的
// NATS连接、Cluster.Init()、以及完整的Actor路由，为了一个纯粹是
// "select获取信号量令牌"的模式对比而拉起整套集群基础设施并不合算，
// 反而会让CPU/内存开销被NATS网络IO/序列化淹没，测不出我们真正关心的
// 差异点。这里直接复用包内真实的全局 workerPool（同一个 channel、
// 同一段临界区代码），只是跳过了 Cluster.Init() 里依赖真实NATS的那部分。
//
// 运行方式：
//   go test ./3rd/cherry-game/cherry/net/cluster/nats_cluster/... \
//     -bench WorkerAcquire -benchmem -run '^$'

import (
	"testing"
	"time"
)

func ensureWorkerPool(size int) {
	poolOnce.Do(func() {
		workerPool = make(chan struct{}, size)
	})
}

// acquireWorker_Before 是修复前的写法：不管worker池是否空闲，
// 每次调用都无条件 time.After(3*time.Second) 创建一个新计时器。
// 即使第一个case立刻命中（99%的正常请求都会立刻命中），
// 那个已经排队等待3秒后触发的runtimeTimer依然会被创建出来，
// 并一直挂在运行时的计时器堆里直到3秒后自然触发才能被彻底回收——
// 这段时间里它是不会被GC提前清理掉的"合法但无用"的对象。
func acquireWorker_Before() (release func(), ok bool) {
	select {
	case workerPool <- struct{}{}:
		return func() { <-workerPool }, true
	case <-time.After(3 * time.Second):
		return nil, false
	}
}

// acquireWorker_After 是修复后的写法：先做一次无阻塞、无分配的尝试，
// 只有在真正拿不到令牌（池已满）时才降级创建计时器等待。
func acquireWorker_After() (release func(), ok bool) {
	select {
	case workerPool <- struct{}{}:
		return func() { <-workerPool }, true
	default:
		timer := time.NewTimer(3 * time.Second)
		defer timer.Stop()
		select {
		case workerPool <- struct{}{}:
			return func() { <-workerPool }, true
		case <-timer.C:
			return nil, false
		}
	}
}

// BenchmarkWorkerAcquire_Before 对应修复前的实现。
// 预期现象：B/op、allocs/op 明显高于 After 版本，因为每次调用都会
// 产生一个 *time.Timer（time.After 内部就是 NewTimer(d).C）。
func BenchmarkWorkerAcquire_Before(b *testing.B) {
	ensureWorkerPool(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		release, ok := acquireWorker_Before()
		if !ok {
			b.Fatal("unexpected pool exhaustion in benchmark")
		}
		release()
	}
}

// BenchmarkWorkerAcquire_After 对应修复后的实现（快路径0分配）。
func BenchmarkWorkerAcquire_After(b *testing.B) {
	ensureWorkerPool(1000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		release, ok := acquireWorker_After()
		if !ok {
			b.Fatal("unexpected pool exhaustion in benchmark")
		}
		release()
	}
}

// BenchmarkWorkerAcquire_After_Parallel 高并发下验证快路径依然0分配、
// 且不会因为多个goroutine争抢同一个channel而退化。
func BenchmarkWorkerAcquire_After_Parallel(b *testing.B) {
	ensureWorkerPool(1000)
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			release, ok := acquireWorker_After()
			if !ok {
				b.Fatal("unexpected pool exhaustion in benchmark")
			}
			release()
		}
	})
}
