package runtime_monitor

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"runtime/trace"
	"sync"
	"time"

	cherryLogger "github.com/cherry-game/cherry/logger"
)

// ProfileDumper 在线上出现"CPU飙高/内存暴涨/Goroutine泄露"的瞬间，
// 自动把 heap/goroutine/allocs/block/mutex 的 pprof 快照以及一段
// CPU profile + execution trace 落盘，供事后用 `go tool pprof` /
// `go tool trace` 离线分析。
//
// 设计要点：
//  1. 覆盖"人不在电脑前"的场景 —— 触发条件来自 AlertEngine 的告警规则
//     （goroutine_leak / memory_leak / gc_pause_high 等），而不是依赖运维
//     手动登录服务器执行 go tool pprof。
//  2. 抓取动作本身也有成本（CPU profile会有一定的采样开销、trace文件可能较大），
//     所以用 cooldown 限流，避免"雪崩式"抓取反而拖垃现场。
//  3. CPU profile / trace 的采集放在独立 goroutine 里做，且用 sync.Mutex
//     保证同一时间只有一个采集任务在跑，避免 pprof.StartCPUProfile 的
//     "cpu profiling already in use" 报错。
type ProfileDumper struct {
	dir      string
	cooldown time.Duration

	mu           sync.Mutex
	lastDumpAt   time.Time
	dumping      bool
	cpuProfileMs time.Duration
	traceMs      time.Duration
}

// NewProfileDumper dir: profile文件输出目录；cooldown: 两次自动抓取之间的最小间隔。
func NewProfileDumper(dir string, cooldown time.Duration) *ProfileDumper {
	if dir == "" {
		dir = "logs/pprof"
	}
	if cooldown <= 0 {
		cooldown = 5 * time.Minute
	}
	return &ProfileDumper{
		dir:          dir,
		cooldown:     cooldown,
		cpuProfileMs: 10 * time.Second,
		traceMs:      5 * time.Second,
	}
}

// TriggerOnAlert 供 AlertEngine 在触发 Critical 告警时调用。
// 非阻塞：内部会另起 goroutine 完成实际抓取，调用方（alertLoop）不会被 CPU
// profile 的采样时长（默认10s）卡住。
func (d *ProfileDumper) TriggerOnAlert(reason string) {
	d.mu.Lock()
	if d.dumping || time.Since(d.lastDumpAt) < d.cooldown {
		d.mu.Unlock()
		return
	}
	d.dumping = true
	d.lastDumpAt = time.Now()
	d.mu.Unlock()

	go func() {
		defer func() {
			d.mu.Lock()
			d.dumping = false
			d.mu.Unlock()
		}()
		d.dumpOnce(reason)
	}()
}

func (d *ProfileDumper) dumpOnce(reason string) {
	ts := time.Now().Format("20060102_150405")
	outDir := filepath.Join(d.dir, fmt.Sprintf("%s_%s", ts, sanitize(reason)))
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		cherryLogger.Errorf("[ProfileDumper] mkdir %s failed: %v", outDir, err)
		return
	}

	cherryLogger.Warnf("[ProfileDumper] auto capture triggered, reason=%s dir=%s", reason, outDir)

	// 1. 即时快照类：heap / goroutine / allocs / block / mutex
	//    这些都是"读当前状态"，几乎零额外开销，第一时间落盘。
	dumpLookup(outDir, "goroutine", 2)
	dumpLookup(outDir, "heap", 0)
	dumpLookup(outDir, "allocs", 0)
	dumpLookup(outDir, "block", 0) // 需要 runtime.SetBlockProfileRate > 0 才有数据
	dumpLookup(outDir, "mutex", 0) // 需要 runtime.SetMutexProfileFraction > 0 才有数据
	dumpLookup(outDir, "threadcreate", 0)

	// 2. CPU profile：有采样开销，异步采集一段时间窗口。
	cpuPath := filepath.Join(outDir, "cpu.pprof")
	if f, err := os.Create(cpuPath); err == nil {
		if err := pprof.StartCPUProfile(f); err == nil {
			time.Sleep(d.cpuProfileMs)
			pprof.StopCPUProfile()
		}
		_ = f.Close()
	} else {
		cherryLogger.Errorf("[ProfileDumper] create cpu.pprof failed: %v", err)
	}

	// 3. execution trace：用于分析调度延迟、GC STW、goroutine阻塞点的时间线，
	//    对定位"channel互相等待造成的死锁/雪崩"特别有效。
	tracePath := filepath.Join(outDir, "trace.out")
	if f, err := os.Create(tracePath); err == nil {
		if err := trace.Start(f); err == nil {
			time.Sleep(d.traceMs)
			trace.Stop()
		}
		_ = f.Close()
	} else {
		cherryLogger.Errorf("[ProfileDumper] create trace.out failed: %v", err)
	}

	writeMemStatsSummary(outDir)

	cherryLogger.Warnf("[ProfileDumper] auto capture finished, dir=%s. 使用 `go tool pprof %s/heap.pprof` 或 `go tool trace %s/trace.out` 离线分析",
		outDir, outDir, outDir)
}

func dumpLookup(dir, name string, debug int) {
	profile := pprof.Lookup(name)
	if profile == nil {
		return
	}
	path := filepath.Join(dir, name+".pprof")
	f, err := os.Create(path)
	if err != nil {
		cherryLogger.Errorf("[ProfileDumper] create %s failed: %v", path, err)
		return
	}
	defer f.Close()
	if err := profile.WriteTo(f, debug); err != nil {
		cherryLogger.Errorf("[ProfileDumper] write %s failed: %v", path, err)
	}
}

func writeMemStatsSummary(dir string) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	path := filepath.Join(dir, "memstats.txt")
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()

	fmt.Fprintf(f, "NumGoroutine=%d\n", runtime.NumGoroutine())
	fmt.Fprintf(f, "HeapAlloc=%dMB HeapInuse=%dMB HeapIdle=%dMB Sys=%dMB\n",
		m.HeapAlloc/1024/1024, m.HeapInuse/1024/1024, m.HeapIdle/1024/1024, m.Sys/1024/1024)
	fmt.Fprintf(f, "NumGC=%d PauseTotalNs=%d LastGC=%s\n",
		m.NumGC, m.PauseTotalNs, time.Unix(0, int64(m.LastGC)).Format(time.RFC3339))
	fmt.Fprintf(f, "GCCPUFraction=%.4f\n", m.GCCPUFraction)
}

func sanitize(s string) string {
	out := make([]rune, 0, len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			out = append(out, r)
		default:
			out = append(out, '_')
		}
	}
	if len(out) == 0 {
		return "alert"
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return string(out)
}
