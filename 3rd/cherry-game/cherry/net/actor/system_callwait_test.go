package cherryActor

// 本文件同时承担两个目的：
//  1. TestSystem_CallWait_TimeoutDoesNotDeadlockTargetActor —— 回归测试，
//     直接证明修复前的 bug（CallWait 超时后目标actor永久死锁）已被修复。
//  2. BenchmarkSystem_CallWait / BenchmarkSystem_CallWait_Parallel —— 用
//     go test -bench + -benchmem 量化修复后的分配情况(B/op、allocs/op)，
//     作为后续任何人再次改动这段代码时的性能基线(baseline)。
//
// 运行方式：
//   go test ./3rd/cherry-game/cherry/net/actor/... -run CallWait -v -race
//   go test ./3rd/cherry-game/cherry/net/actor/... -bench CallWait -benchmem -run '^$'
//   go test ./3rd/cherry-game/cherry/net/actor/... -bench CallWait -benchmem -run '^$' -cpuprofile cpu.out
//   go tool pprof cpu.out

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	ccode "github.com/cherry-game/cherry/code"
	cfacade "github.com/cherry-game/cherry/facade"
)

// ---- 最小化的 IApplication 桩实现 --------------------------------------
// CallWait 的同节点(非集群)路径不会真正调用 app.Serializer()/app.Cluster()，
// 只要求 app != nil（用于 InvokeRemoteFunc 里的判空），因此这里给一个
// 满足接口即可、方法体基本为空的桩对象，避免为了一个单测拉起整套
// etcd/nats/gorm依赖。
type benchApp struct{}

func (benchApp) NodeID() string                            { return "bench-node" }
func (benchApp) NodeType() string                          { return "bench" }
func (benchApp) Address() string                           { return "" }
func (benchApp) RpcAddress() string                        { return "" }
func (benchApp) Settings() cfacade.ProfileJSON             { return nil }
func (benchApp) Enabled() bool                             { return true }
func (benchApp) Running() bool                             { return true }
func (benchApp) DieChan() chan bool                        { return nil }
func (benchApp) IsFrontend() bool                          { return false }
func (benchApp) Register(components ...cfacade.IComponent) {}
func (benchApp) Find(name string) cfacade.IComponent       { return nil }
func (benchApp) Remove(name string) cfacade.IComponent     { return nil }
func (benchApp) All() []cfacade.IComponent                 { return nil }
func (benchApp) OnShutdown(fn ...func())                   {}
func (benchApp) Startup()                                  {}
func (benchApp) Shutdown()                                 {}
func (benchApp) Serializer() cfacade.ISerializer           { return nil }
func (benchApp) Discovery() cfacade.IDiscovery             { return nil }
func (benchApp) Cluster() cfacade.ICluster                 { return nil }
func (benchApp) ActorSystem() cfacade.IActorSystem         { return nil }

// ---- 最小化的 Actor Handler：注册一个 Echo 远程函数 ---------------------
// echoHandler 直接实现包内私有接口 IActorLoader(load(actor *Actor))，
// 从而拿到 *Actor 引用去注册 Remote 函数，等价于业务代码里
// actor.Remote().Register(...) 的用法。
type echoHandler struct {
	aliasID   string
	sleep     time.Duration // 模拟业务函数执行耗时，用于制造"超过callTimeout"的场景
	sleepOnce bool          // 为true时只有第1次调用会sleep，模拟"一次性抖动/慢查询，随后恢复正常"
	callCount int32
}

func (h *echoHandler) AliasID() string { return h.aliasID }
func (h *echoHandler) OnInit()         {}
func (h *echoHandler) OnStop()         {}
func (h *echoHandler) OnLocalReceived(m *cfacade.Message) (bool, bool) {
	return true, false
}

// OnRemoteReceived 契约参考 actor_base.go 里 Base 的默认实现：
// next=true 表示"继续走通用invoke流程"，invoke=false 表示"不要在这里提前调用一次"。
// 若误写成 (true, true)，processRemote 会对同一条消息调用两次 invokeFunc，
// 从而触发两次 ChanResult 写入——这正是本文件用于验证
// sendChanResult 非阻塞发送防御是否生效的场景，见下面的
// TestSystem_CallWait_DuplicateInvokeDoesNotDeadlock。
func (h *echoHandler) OnRemoteReceived(m *cfacade.Message) (bool, bool) {
	return true, false
}
func (h *echoHandler) OnFindChild(m *cfacade.Message) (cfacade.IActor, bool) {
	return nil, false
}

// load 在actor创建时被 newActor 自动调用（见 actor.go 里的 IActorLoader 判断）。
//
// 注意：Echo 的返回值签名是"单个int32"，对应 invoke.go/retValue 里
// retsLen==1 的分支——框架会把这个返回值直接当成响应码(rsp.Code)，
// 而不是"业务数据"。所以这里必须固定返回 ccode.OK，不能像naive的echo
// 语义那样把请求参数req原样返回，否则调用方拿到的"code"其实是
// req的值，会被CallWait错误地判定为失败（这也是本文件调试过程中
// 真实踩到的一个坑，留作注释提醒后来者）。
func (h *echoHandler) load(actor *Actor) {
	actor.Remote().Register("Echo", func(ctx context.Context, req int32) int32 {
		n := atomic.AddInt32(&h.callCount, 1)
		if h.sleep > 0 && (!h.sleepOnce || n == 1) {
			time.Sleep(h.sleep)
		}
		return ccode.OK
	})
}

// duplicateInvokeHandler 故意模拟一次"OnRemoteReceived误用"的场景：
// 对同一条消息触发两次 invokeFunc（真实的bug只需要业务代码把
// (next, invoke) 误写成 (true, true) 就会发生，见 actor.go processRemote），
// 用来验证 invoke.go 的 sendChanResult 非阻塞发送能否兜住"重复发送"。
type duplicateInvokeHandler struct {
	echoHandler
}

func (h *duplicateInvokeHandler) OnRemoteReceived(m *cfacade.Message) (bool, bool) {
	return true, true // 错误用法：next=true 又 invoke=true，会导致 invokeFunc 被调用2次
}

func newBenchSystem(callTimeout time.Duration) *System {
	sys := NewSystem()
	sys.SetApp(benchApp{})
	sys.SetCallTimeout(callTimeout)
	return sys
}

func actorPath(sys *System, aliasID string) string {
	return cfacade.NewPath(sys.NodeID(), aliasID)
}

// waitActorReady CreateActor 内部是 `go thisActor.run()` 异步启动的，
// 框架没有提供"等待actor真正进入WorkerState"的同步信号。生产环境里这个
// 窗口几乎不会被踩到（所有actor都在节点启动阶段统一创建，之后才开始
// 对外服务），但在单测/benchmark里创建完actor立刻发起调用，存在极小概率
// 的竞态：actor的goroutine还没来得及跑到 onInit() 把状态置为
// WorkerState。这里用短暂重试代替真正的"就绪通知机制"（这也是本次
// 排查顺带发现、值得在后续版本给 CreateActor 补一个ready channel的点）。
func waitActorReady(t testing.TB, sys *System, sourcePath, targetPath string) {
	deadline := time.Now().Add(2 * time.Second)
	for {
		code := sys.CallWait(sourcePath, targetPath, "Echo", "", int32(0), nil)
		if code == ccode.OK {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("actor %s never became ready, last code=%d", targetPath, code)
		}
		time.Sleep(time.Millisecond)
	}
}

// TestSystem_CallWait_TimeoutDoesNotDeadlockTargetActor 复现并验证修复：
// 目标actor的业务函数执行时间(200ms) > callTimeout(50ms)时，
//  1. CallWait 必须能正常返回 ActorCallTimeout（而不是一直卡住测试进程）。
//  2. 目标actor事后写回结果时(m.ChanResult <- rsp)不能死锁在自己的
//     处理goroutine里 —— 用"超时之后再发一次正常调用，必须成功"来断言，
//     因为 Actor.loop() 是单goroutine串行处理消息，一旦被卡死，
//     后续所有消息（包含其他玩家/其他逻辑）都会被无限期阻塞，
//     这正是该bug在生产环境里表现为"部分玩家彻底卡死不响应"的根因。
func TestSystem_CallWait_TimeoutDoesNotDeadlockTargetActor(t *testing.T) {
	const callTimeout = 50 * time.Millisecond
	const handlerSleep = 200 * time.Millisecond

	sys := newBenchSystem(callTimeout)
	defer sys.Stop()

	caller := &echoHandler{aliasID: "caller"}
	// sleepOnce: 只有第一次调用会人为卡住200ms(> 50ms的callTimeout)，
	// 模拟一次GC停顿/慢查询等瞬时抖动；第二次调用应该立刻正常返回。
	// 如果按第一版写法让每次调用都sleep 200ms，第二次调用在同样50ms的
	// callTimeout下"本来就应该"超时，会把"目标actor被写死"和
	// "本次调用本身超时"这两件不同的事情混在一起，误导排查方向。
	slowTarget := &echoHandler{aliasID: "slow-target", sleep: handlerSleep, sleepOnce: true}

	if _, err := sys.CreateActor(caller.aliasID, caller); err != nil {
		t.Fatalf("create caller actor: %v", err)
	}
	if _, err := sys.CreateActor(slowTarget.aliasID, slowTarget); err != nil {
		t.Fatalf("create target actor: %v", err)
	}

	sourcePath := actorPath(sys, caller.aliasID)
	targetPath := actorPath(sys, slowTarget.aliasID)

	start := time.Now()
	code := sys.CallWait(sourcePath, targetPath, "Echo", "trace-1", int32(1), nil)
	elapsed := time.Since(start)

	if code != ccode.ActorCallTimeout {
		t.Fatalf("expected ActorCallTimeout(%d), got %d (elapsed=%v)", ccode.ActorCallTimeout, code, elapsed)
	}
	if elapsed > callTimeout*3 {
		t.Fatalf("CallWait should return promptly after callTimeout, elapsed=%v", elapsed)
	}

	// 等待迟到的业务函数真正执行完（模拟"响应结果姗姗来迟"到达的那一刻），
	// 这一步正是修复前会在 m.ChanResult <- rsp 上永久阻塞的地方。
	time.Sleep(handlerSleep + 100*time.Millisecond)

	// 关键断言：目标actor必须仍然存活、可以继续处理新消息。
	// 若旧的unbuffered channel bug仍然存在，target actor的处理goroutine
	// 会永远卡在上一条消息的发送上，这次调用会再次超时甚至永远拿不到结果。
	done := make(chan int32, 1)
	go func() {
		done <- sys.CallWait(sourcePath, targetPath, "Echo", "trace-2", int32(2), nil)
	}()

	select {
	case code2 := <-done:
		if code2 != ccode.OK {
			t.Fatalf("target actor did not recover, second CallWait code=%d", code2)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("target actor appears permanently deadlocked after a timed-out CallWait (goroutine leak reproduced)")
	}
}

// TestSystem_CallWait_DuplicateInvokeDoesNotDeadlock 验证第二层防御：
// 即便业务handler把 OnRemoteReceived 的返回值误写成 (next=true, invoke=true)
// 导致同一条消息被 invokeFunc 处理两次（因而产生两次 ChanResult 写入），
// 目标actor也绝不能被第二次写入卡死——sendChanResult 的 select+default
// 保证多余的写入会被静默丢弃而不是阻塞。
// 这个测试如果把 invoke.go 里的 sendChanResult 还原成裸的 `ch <- v`，
// 会立刻在第二次写入处永久阻塞，可用它验证"防御是否真的生效"。
func TestSystem_CallWait_DuplicateInvokeDoesNotDeadlock(t *testing.T) {
	sys := newBenchSystem(2 * time.Second)
	defer sys.Stop()

	caller := &echoHandler{aliasID: "dup-caller"}
	buggyTarget := &duplicateInvokeHandler{echoHandler: echoHandler{aliasID: "dup-target"}}

	if _, err := sys.CreateActor(caller.aliasID, caller); err != nil {
		t.Fatalf("create caller actor: %v", err)
	}
	if _, err := sys.CreateActor(buggyTarget.aliasID, buggyTarget); err != nil {
		t.Fatalf("create target actor: %v", err)
	}

	sourcePath := actorPath(sys, caller.aliasID)
	targetPath := actorPath(sys, buggyTarget.aliasID)

	done := make(chan int32, 1)
	go func() {
		done <- sys.CallWait(sourcePath, targetPath, "Echo", "", int32(7), nil)
	}()

	select {
	case code := <-done:
		if code != ccode.OK {
			t.Fatalf("expected OK, got code=%d", code)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("duplicate invoke deadlocked the target actor (sendChanResult defense failed)")
	}

	// 目标actor在"多余的一次发送"之后必须继续存活、可正常处理后续消息。
	code2 := sys.CallWait(sourcePath, targetPath, "Echo", "", int32(8), nil)
	if code2 != ccode.OK {
		t.Fatalf("target actor did not survive duplicate invoke, code=%d", code2)
	}
}

// BenchmarkSystem_CallWait 测量单条同节点 CallWait 往返的耗时与内存分配。
// 关注指标：ns/op（时延）、B/op 与 allocs/op（每次调用产生的堆分配量，
// 直接反映 GC 标记阶段需要扫描的对象数量）。
func BenchmarkSystem_CallWait(b *testing.B) {
	sys := newBenchSystem(2 * time.Second)
	defer sys.Stop()

	caller := &echoHandler{aliasID: "caller"}
	target := &echoHandler{aliasID: "target"}
	if _, err := sys.CreateActor(caller.aliasID, caller); err != nil {
		b.Fatalf("create caller actor: %v", err)
	}
	if _, err := sys.CreateActor(target.aliasID, target); err != nil {
		b.Fatalf("create target actor: %v", err)
	}

	sourcePath := actorPath(sys, caller.aliasID)
	targetPath := actorPath(sys, target.aliasID)

	// 预热：等待目标actor的goroutine真正跑起来，避免把"actor未就绪"的
	// 等待时间计入benchmark结果。
	waitActorReady(b, sys, sourcePath, targetPath)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if code := sys.CallWait(sourcePath, targetPath, "Echo", "", int32(i), nil); code != ccode.OK {
			b.Fatalf("CallWait failed at i=%d, code=%d", i, code)
		}
	}
}

// BenchmarkSystem_CallWait_Parallel 模拟多个goroutine并发向同一个目标actor
// 发起 CallWait（例如多个房间/连接同时调用某个公共服务型actor），
// 用于观察高并发下 P99 时延与分配量是否随并发度显著恶化。
func BenchmarkSystem_CallWait_Parallel(b *testing.B) {
	sys := newBenchSystem(2 * time.Second)
	defer sys.Stop()

	target := &echoHandler{aliasID: "target-parallel"}
	if _, err := sys.CreateActor(target.aliasID, target); err != nil {
		b.Fatalf("create target actor: %v", err)
	}
	targetPath := actorPath(sys, target.aliasID)
	// source 不需要真实存在的actor：CallWait只在 sourcePath.ActorID==targetPath.ActorID 时
	// 才会去查找同名actor，这里用一个固定的、不存在的source path即可。
	sourcePath := actorPath(sys, "caller-parallel")

	waitActorReady(b, sys, sourcePath, targetPath)

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var i int32
		for pb.Next() {
			if code := sys.CallWait(sourcePath, targetPath, "Echo", "", i, nil); code != ccode.OK {
				b.Fatalf("CallWait failed, code=%d", code)
			}
			i++
		}
	})
}
