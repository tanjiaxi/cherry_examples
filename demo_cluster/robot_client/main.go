package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	chttp "github.com/cherry-game/cherry/extend/http"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/logger/rotatelogs"
	pomeloClient "github.com/cherry-game/cherry/net/parser/pomelo/client"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/robot_client/pkg/robotclient"
	jsoniter "github.com/json-iterator/go"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoadTestConfig 压测运行时配置，全部通过命令行 flag 注入，避免再改代码重编。
type LoadTestConfig struct {
	URL            string
	PID            string
	Robots         int
	BatchSize      int
	BatchInterval  time.Duration
	HoldDuration   time.Duration
	SpinInterval   time.Duration
	PrintInterval  time.Duration
	ErrorThreshold float64
	UseWebSocket   bool
	UseServerList  bool
	FallbackAddr   string
	AreaId         int
	ServerId       int
	WarmupSpins    int
	RunSpin        bool
	PrintLog       bool
	RegisterFirst  bool
}

var loadCfg LoadTestConfig

func init() {
	flag.StringVar(&loadCfg.URL, "url", "http://10.10.10.251:8081", "web node URL")
	flag.StringVar(&loadCfg.PID, "pid", "2126001", "SDK PID")
	flag.IntVar(&loadCfg.Robots, "robots", 100, "number of robots")
	flag.IntVar(&loadCfg.BatchSize, "batch-size", 20, "robots started per batch")
	flag.DurationVar(&loadCfg.BatchInterval, "batch-interval", time.Second, "interval between batches")
	flag.DurationVar(&loadCfg.HoldDuration, "duration", 30*time.Minute, "steady-state spin duration")
	flag.DurationVar(&loadCfg.SpinInterval, "spin-interval", 500*time.Millisecond, "interval between spins per robot")
	flag.DurationVar(&loadCfg.PrintInterval, "print-interval", 5*time.Second, "status print interval")
	flag.Float64Var(&loadCfg.ErrorThreshold, "error-threshold", 0.01, "stop spawning when error rate exceeds this value")
	flag.BoolVar(&loadCfg.UseWebSocket, "websocket", true, "use websocket (true) or TCP (false)")
	flag.BoolVar(&loadCfg.UseServerList, "server-list", true, "fetch gate/server from /serverList API")
	flag.StringVar(&loadCfg.FallbackAddr, "gate", "10.10.10.251:10010", "fallback gate address when server-list is disabled or fails")
	flag.IntVar(&loadCfg.AreaId, "area", 1, "target area id (0 = first available)")
	flag.IntVar(&loadCfg.ServerId, "server", 10001, "target server id (0 = first available in area)")
	flag.IntVar(&loadCfg.WarmupSpins, "warmup-spins", 0, "extra ActorSpin calls during login steps")
	flag.BoolVar(&loadCfg.RunSpin, "spin", true, "run continuous spin after all robots connect")
	flag.BoolVar(&loadCfg.PrintLog, "verbose", false, "print per-robot debug logs")
	flag.BoolVar(&loadCfg.RegisterFirst, "register", false, "pre-register accounts via /register before load test")
}

// 服务器节点 pprof 地址
var serverPprofAddrs = map[string]string{
	"game":   "http://10.10.10.251:6060",
	"gate":   "http://10.10.10.251:6061",
	"web":    "http://10.10.10.251:6062",
	"center": "http://10.10.10.251:6063",
}

// ==================== 指标计数器 ====================
var (
	onlineCount, totalRequests, successCount, errorCount   int64
	totalLatencyMs, maxLatencyMs, spinRequests, spinErrors int64
	testStartTime                                          time.Time
	stopSpawning, stopSpinning                             int32
)

var (
	firstRequestTime int64 // 用Unix时间戳存储，替代time.Time结构体
	lastRequestTime  int64
)

// 保存所有成功连接的 robot
var (
	connectedRobots   []*robotclient.Robot
	connectedRobotsMu sync.Mutex
)

// 缓存的区服列表
var (
	cachedServerList *robotclient.ServerListResponse
	serverListMu     sync.RWMutex
)

// ==================== 每个接口独立的 QPS 统计器 ====================
type APIMetrics struct {
	// 累计统计
	TotalLatencyMs int64 // 总延迟(毫秒)
	Count          int64 // 总调用次数
	MaxLatencyMs   int64 // 最大延迟
	ErrorCount     int64 // 错误次数

	// 滑动窗口 QPS 统计 (固定大小环形缓冲区)
	windowMu     sync.Mutex
	windowCounts [60]int64 // 固定 60 秒的环形缓冲区
	windowSize   int       // 实际使用的窗口大小
	startSecond  int64     // 起始秒 (Unix 秒)
}

// NewAPIMetrics 创建新的 API 指标
func NewAPIMetrics(windowSize int) *APIMetrics {
	if windowSize > 60 {
		windowSize = 60
	}
	return &APIMetrics{
		windowSize:  windowSize,
		startSecond: time.Now().Unix(),
	}
}

// Record 记录一次 API 调用
func (m *APIMetrics) Record(latencyMs int64, isError bool) {
	// 累计统计
	atomic.AddInt64(&m.TotalLatencyMs, latencyMs)
	atomic.AddInt64(&m.Count, 1)
	if isError {
		atomic.AddInt64(&m.ErrorCount, 1)
	}
	// 更新最大延迟
	for {
		cur := atomic.LoadInt64(&m.MaxLatencyMs)
		if latencyMs <= cur || atomic.CompareAndSwapInt64(&m.MaxLatencyMs, cur, latencyMs) {
			break
		}
	}

	// 滑动窗口统计 - 使用环形缓冲区
	m.windowMu.Lock()
	defer m.windowMu.Unlock()

	nowSec := time.Now().Unix()
	idx := int(nowSec % 60)

	// FIX 1: 更安全的清理逻辑
	if nowSec > m.startSecond {
		// 计算需要清理的秒数
		skipSeconds := nowSec - m.startSecond

		// 如果跨度超过60秒，清空所有
		if skipSeconds >= 60 {
			for i := range m.windowCounts {
				m.windowCounts[i] = 0
			}
		} else {
			// 只清理过期的槽位（不包括当前秒）
			for i := int64(1); i <= skipSeconds; i++ {
				clearSec := m.startSecond + i
				clearIdx := int(clearSec % 60)
				// 不要清理当前秒的槽位
				if clearIdx != idx {
					m.windowCounts[clearIdx] = 0
				}
			}
		}
		m.startSecond = nowSec
	}
	m.windowCounts[idx]++
}

// GetRealtimeQPS 获取实时 QPS (最近 N 秒的平均，不包含当前秒)
func (m *APIMetrics) GetRealtimeQPS() float64 {
	m.windowMu.Lock()
	defer m.windowMu.Unlock()

	nowSec := time.Now().Unix()
	var total int64
	validSeconds := 0

	// 统计最近 windowSize 秒的数据 (不包含当前秒，因为当前秒不完整)
	for i := 1; i <= m.windowSize; i++ {
		sec := nowSec - int64(i)
		idx := int(sec % 60)

		// FIX: 正确检查数据是否有效
		// 只统计在 startSecond 之后的数据
		if sec > m.startSecond-60 && sec >= m.startSecond-int64(m.windowSize) {
			total += m.windowCounts[idx]
			validSeconds++
		}
	}

	if validSeconds > 0 {
		return float64(total) / float64(validSeconds)
	}
	return 0
}

// GetLastSecondQPS 获取上一秒的 QPS
func (m *APIMetrics) GetLastSecondQPS() float64 {
	m.windowMu.Lock()
	defer m.windowMu.Unlock()

	lastSec := time.Now().Unix() - 1
	idx := int(lastSec % 60)
	return float64(m.windowCounts[idx])
}

// GetCurrentSecondQPS 获取当前秒的 QPS (不完整)
func (m *APIMetrics) GetCurrentSecondQPS() float64 {
	m.windowMu.Lock()
	defer m.windowMu.Unlock()

	nowSec := time.Now().Unix()
	idx := int(nowSec % 60)
	return float64(m.windowCounts[idx])
}

// GetStats 获取统计数据
func (m *APIMetrics) GetStats() (count, totalLatency, maxLatency, errors int64) {
	return atomic.LoadInt64(&m.Count),
		atomic.LoadInt64(&m.TotalLatencyMs),
		atomic.LoadInt64(&m.MaxLatencyMs),
		atomic.LoadInt64(&m.ErrorCount)
}

var (
	apiMetrics   map[string]*APIMetrics
	apiMetricsMu sync.RWMutex
	apiOrder     = []string{
		"GetToken", "ConnectTCP", "ConnectWS", "UserLogin", "PlayerSelect",
		"ActorCreate", "ActorEnter", "ActorEnterMachine", "ActorMachine", "ActorSpin",
	}
)

func initAPIMetrics() {
	apiMetrics = make(map[string]*APIMetrics)
	for _, name := range apiOrder {
		apiMetrics[name] = NewAPIMetrics(10) // 10秒滑动窗口
	}
}

type SystemMetrics struct {
	CPUPercent            float64
	MemUsedMB, MemTotalMB uint64
	MemPercent            float64
	GoRoutines            int
	HeapAllocMB           uint64
}

type ServerNodeMetrics struct {
	Name       string
	Online     bool
	Goroutines int64
}

func getSystemMetrics() SystemMetrics {
	var sm SystemMetrics
	if cpuPct, err := cpu.Percent(0, false); err == nil && len(cpuPct) > 0 {
		sm.CPUPercent = cpuPct[0]
	}
	if memInfo, err := mem.VirtualMemory(); err == nil {
		sm.MemUsedMB = memInfo.Used / 1024 / 1024
		sm.MemTotalMB = memInfo.Total / 1024 / 1024
		sm.MemPercent = memInfo.UsedPercent
	}
	sm.GoRoutines = runtime.NumGoroutine()
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	sm.HeapAllocMB = memStats.HeapAlloc / 1024 / 1024
	return sm
}

func getServerPprofMetrics(name, addr string) ServerNodeMetrics {
	snm := ServerNodeMetrics{Name: name}
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(fmt.Sprintf("%s/debug/pprof/goroutine?debug=1", addr))
	if err != nil {
		return snm
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK {
		snm.Online = true
		body, _ := io.ReadAll(resp.Body)
		fmt.Sscanf(string(body), "goroutine profile: total %d", &snm.Goroutines)
	}
	return snm
}

func getAllServerMetrics() []ServerNodeMetrics {
	var metrics []ServerNodeMetrics
	var wg sync.WaitGroup
	var mu sync.Mutex
	for name, addr := range serverPprofAddrs {
		wg.Add(1)
		go func(n, a string) {
			defer wg.Done()
			m := getServerPprofMetrics(n, a)
			mu.Lock()
			metrics = append(metrics, m)
			mu.Unlock()
		}(name, addr)
	}
	wg.Wait()
	return metrics
}

func recordAPIMetrics(apiName string, startTime time.Time, isError bool) {
	// 记录时间时的逻辑
	now := time.Now().UnixNano()
	// 原子比较并交换，只保留最早的时间
	atomic.CompareAndSwapInt64(&firstRequestTime, 0, now)
	// 原子更新，只保留最晚的时间
	atomic.StoreInt64(&lastRequestTime, now)

	latencyMs := time.Since(startTime).Milliseconds()
	// if latencyMs > 100 {
	// 	clog.Warnf("[recordAPIMetrics] %s : timeout: %d ms", apiName, latencyMs)
	// }
	apiMetricsMu.RLock()
	m := apiMetrics[apiName]
	apiMetricsMu.RUnlock()
	// clog.Infof("[recordAPIMetrics] %s : time: %d ms", apiName, latencyMs)
	if m != nil {
		m.Record(latencyMs, isError)
	}
}

// initLogger 初始化日志，同时输出到控制台和文件
func initLogger() {
	// 创建 logs 目录
	if err := os.MkdirAll("logs", 0o755); err != nil {
		fmt.Printf("Failed to create logs directory: %v\n", err)
		return
	}

	// 配置日志轮转
	logFile := "logs/robot_client.log"
	logFileFormat := "logs/robot_client_%Y%m%d%H%M.log"

	hook, err := rotatelogs.New(
		logFileFormat,
		rotatelogs.WithLinkName(logFile),
		rotatelogs.WithMaxAge(time.Hour*24*7),     // 保留7天
		rotatelogs.WithRotationTime(time.Hour*24), // 每天轮转
	)
	if err != nil {
		fmt.Printf("Failed to create rotatelogs: %v\n", err)
		return
	}

	// 配置 encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "ts",
		LevelKey:       "level",
		CallerKey:      "caller",
		MessageKey:     "msg",
		StacktraceKey:  "stack",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.TimeEncoderOfLayout("15:04:05.000"),
		EncodeDuration: zapcore.StringDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// 同时输出到控制台和文件
	writers := []zapcore.WriteSyncer{
		zapcore.AddSync(os.Stderr), // 控制台
		zapcore.AddSync(hook),      // 文件
	}

	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderConfig),
		zapcore.NewMultiWriteSyncer(writers...),
		zap.NewAtomicLevelAt(zapcore.DebugLevel),
	)

	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))
	clog.DefaultLogger = &clog.CherryLogger{SugaredLogger: logger.Sugar()}

	clog.Info("Logger initialized: console + file (logs/robot_client.log)")
}

func main() {
	flag.Parse()
	initLogger()
	testStartTime = time.Now()
	initAPIMetrics()

	clog.Infow("load test started",
		"url", loadCfg.URL,
		"pid", loadCfg.PID,
		"robots", loadCfg.Robots,
		"batch_size", loadCfg.BatchSize,
		"batch_interval", loadCfg.BatchInterval,
		"duration", loadCfg.HoldDuration,
		"spin_interval", loadCfg.SpinInterval,
		"print_interval", loadCfg.PrintInterval,
		"run_spin", loadCfg.RunSpin,
		"server_list", loadCfg.UseServerList,
		"websocket", loadCfg.UseWebSocket,
		"area", loadCfg.AreaId,
		"server", loadCfg.ServerId,
		"gate", loadCfg.FallbackAddr,
	)

	// Ctrl+C / SIGTERM：停止拉人与持续 Spin，走统一的断开与汇总路径
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigCh
		clog.Warnf("received signal %v, stopping load test...", sig)
		atomic.StoreInt32(&stopSpawning, 1)
		atomic.StoreInt32(&stopSpinning, 1)
	}()

	accounts := buildAccounts(loadCfg.Robots)
	if loadCfg.RegisterFirst {
		clog.Infof("pre-registering %d accounts...", len(accounts))
		RegisterDevAccount(loadCfg.URL, accounts)
	}

	if err := prepareServerList(); err != nil {
		clog.Warnf("prepare server list failed: %v, will use fallback gate=%s server=%d",
			err, loadCfg.FallbackAddr, loadCfg.ServerId)
	} else if loadCfg.UseServerList {
		printServerList()
	}

	stopPrinting := make(chan struct{})
	go PrintStatusLoop(stopPrinting)

	RunLoadTest(accounts)

	if loadCfg.RunSpin && atomic.LoadInt32(&stopSpinning) == 0 {
		RunContinuousSpin()
	}

	DisconnectAllRobots()
	PrintSummary()
	close(stopPrinting)
}

func buildAccounts(robotCount int) map[string]string {
	if robotCount <= 0 {
		return map[string]string{}
	}

	accounts := make(map[string]string, robotCount)
	for i := 1; i <= robotCount; i++ {
		account := fmt.Sprintf("loadtest%06d", i)
		accounts[account] = account
	}
	return accounts
}

// prepareServerList 在压测开始前拉取一次区服列表并缓存。
// 每个 robot 共用同一份列表，避免 N 个机器人各自打一遍 /server/list。
func prepareServerList() error {
	if !loadCfg.UseServerList {
		return nil
	}

	cli := robotclient.New(pomeloClient.New(
		pomeloClient.WithRequestTimeout(10*time.Second),
		pomeloClient.WithErrorBreak(true),
	))
	cli.PrintLog = loadCfg.PrintLog
	return fetchServerList(cli)
}

// fetchServerList 获取区服列表
func fetchServerList(cli *robotclient.Robot) error {
	serverList, err := cli.GetServerList(loadCfg.URL, loadCfg.PID)
	if err != nil {
		return err
	}

	serverListMu.Lock()
	cachedServerList = serverList
	serverListMu.Unlock()
	return nil
}

// printServerList 打印区服列表
func printServerList() {
	serverListMu.RLock()
	defer serverListMu.RUnlock()

	if cachedServerList == nil {
		return
	}

	clog.Info("========== Server List ==========")
	for _, area := range cachedServerList.Areas {
		clog.Infof("  Area[%d]: %s, Gate: %s", area.AreaId, area.AreaName, area.Gate)
	}
	for _, server := range cachedServerList.Servers {
		clog.Infof("  Server[%d]: %s, AreaId: %d, Status: %d",
			server.ServerId, server.ServerName, server.AreaId, server.Status)
	}
	clog.Info("=================================")
}

// getGateAddrAndServerId 获取 Gate 地址和 ServerId。
// 优先按 -area / -server 从缓存的区服列表选取；列表不可用时回退到 -gate / -server。
func getGateAddrAndServerId() (gateAddr string, serverId int32) {
	fallbackServerId := int32(loadCfg.ServerId)
	if fallbackServerId == 0 {
		fallbackServerId = 10001
	}

	serverListMu.RLock()
	defer serverListMu.RUnlock()

	if !loadCfg.UseServerList || cachedServerList == nil || len(cachedServerList.Areas) == 0 {
		return loadCfg.FallbackAddr, fallbackServerId
	}

	areaId := int32(loadCfg.AreaId)
	wantServerId := int32(loadCfg.ServerId)

	var targetArea *robotclient.AreaInfo
	for _, area := range cachedServerList.Areas {
		if areaId == 0 || area.AreaId == areaId {
			targetArea = area
			break
		}
	}
	if targetArea == nil {
		targetArea = cachedServerList.Areas[0]
	}

	var targetServer *robotclient.ServerInfo
	for _, server := range cachedServerList.Servers {
		if server.AreaId != targetArea.AreaId {
			continue
		}
		if wantServerId == 0 || server.ServerId == wantServerId {
			targetServer = server
			break
		}
	}
	if targetServer == nil {
		for _, server := range cachedServerList.Servers {
			if server.AreaId == targetArea.AreaId {
				targetServer = server
				break
			}
		}
	}
	if targetServer == nil {
		clog.Warnf("no server found in area=%d, fallback to gate=%s server=%d",
			targetArea.AreaId, loadCfg.FallbackAddr, fallbackServerId)
		return loadCfg.FallbackAddr, fallbackServerId
	}

	return targetArea.Gate, targetServer.ServerId
}

func RunLoadTest(accounts map[string]string) {
	type acc struct{ user, pass string }
	list := make([]acc, 0, len(accounts))
	for u, p := range accounts {
		list = append(list, acc{u, p})
	}
	totalBatches := (len(list) + loadCfg.BatchSize - 1) / loadCfg.BatchSize
	clog.Infof("Starting: %d robots in %d batches", len(list), totalBatches)

	for batch := 0; batch < totalBatches; batch++ {
		if atomic.LoadInt32(&stopSpawning) == 1 {
			break
		}
		start, end := batch*loadCfg.BatchSize, (batch+1)*loadCfg.BatchSize
		if end > len(list) {
			end = len(list)
		}
		clog.Infof("Batch %d/%d (%d robots)", batch+1, totalBatches, end-start)

		var wg sync.WaitGroup
		for _, a := range list[start:end] {
			wg.Add(1)
			go func(u, p string) {
				defer wg.Done()
				if robot := RunRobotWithMetrics(loadCfg.URL, loadCfg.PID, u, p, loadCfg.PrintLog); robot != nil {
					connectedRobotsMu.Lock()
					connectedRobots = append(connectedRobots, robot)
					connectedRobotsMu.Unlock()
				}
			}(a.user, a.pass)
		}
		wg.Wait()

		if t, e := atomic.LoadInt64(&totalRequests), atomic.LoadInt64(&errorCount); t > 0 && float64(e)/float64(t) > loadCfg.ErrorThreshold {
			clog.Warnf("Error rate exceeds threshold, stopping")
			atomic.StoreInt32(&stopSpawning, 1)
			break
		}
		if batch < totalBatches-1 {
			time.Sleep(loadCfg.BatchInterval)
		}
	}
	connectedRobotsMu.Lock()
	clog.Infof("Spawning done. %d robots connected.", len(connectedRobots))
	connectedRobotsMu.Unlock()
}

func RunContinuousSpin() {
	connectedRobotsMu.Lock()
	robots := make([]*robotclient.Robot, len(connectedRobots))
	copy(robots, connectedRobots)
	connectedRobotsMu.Unlock()

	if len(robots) == 0 {
		clog.Warn("No robots for Spin")
		return
	}
	clog.Infof("Spinning with %d robots for %v", len(robots), loadCfg.HoldDuration)

	var wg sync.WaitGroup
	stopTime := time.Now().Add(loadCfg.HoldDuration)
	for _, r := range robots {
		wg.Add(1)
		go func(robot *robotclient.Robot) {
			defer wg.Done()
			for time.Now().Before(stopTime) && atomic.LoadInt32(&stopSpinning) == 0 {
				start := time.Now()
				err := robot.ActorSpin()
				atomic.AddInt64(&spinRequests, 1)
				if err != nil {
					atomic.AddInt64(&spinErrors, 1)
					recordAPIMetrics("ActorSpin", start, true)
				} else {
					recordAPIMetrics("ActorSpin", start, false)
				}
				time.Sleep(loadCfg.SpinInterval)
			}
		}(r)
	}
	wg.Wait()
	clog.Info("Spin completed")
}

func DisconnectAllRobots() {
	connectedRobotsMu.Lock()
	robots := connectedRobots
	connectedRobots = nil
	connectedRobotsMu.Unlock()
	clog.Infof("Disconnecting %d robots...", len(robots))
	var wg sync.WaitGroup
	for _, r := range robots {
		wg.Add(1)
		go func(robot *robotclient.Robot) {
			defer wg.Done()
			robot.Disconnect()
			atomic.AddInt64(&onlineCount, -1)
		}(r)
	}
	wg.Wait()
	clog.Info("All disconnected")
}

func RunRobotWithMetrics(url, pid, userName, password string, printLog bool) *robotclient.Robot {
	startTime := time.Now()
	atomic.AddInt64(&totalRequests, 1)

	cli := robotclient.New(pomeloClient.New(
		pomeloClient.WithRequestTimeout(10*time.Second),
		pomeloClient.WithErrorBreak(true),
	))
	cli.PrintLog = printLog
	cli.TagName = userName

	gateAddr, serverId := getGateAddrAndServerId()

	type step struct {
		name string
		fn   func() error
	}
	steps := []step{
		{"GetToken", func() error { return cli.GetToken(url, pid, userName, password) }},
	}

	if loadCfg.UseWebSocket {
		steps = append(steps, step{"ConnectWS", func() error { return cli.ConnectToWebSocket(gateAddr) }})
	} else {
		steps = append(steps, step{"ConnectTCP", func() error { return cli.ConnectToTCP(gateAddr) }})
	}

	steps = append(steps,
		step{"UserLogin", func() error { return cli.UserLogin(serverId) }},
		step{"PlayerSelect", func() error { return cli.PlayerSelect() }},
		step{"ActorCreate", func() error { return cli.ActorCreate() }},
		step{"ActorEnter", func() error { return cli.ActorEnter() }},
		step{"ActorEnterMachine", func() error { return cli.ActorEnterEnterMachine() }},
		step{"ActorMachine", func() error { return cli.ActorMachine() }},
		step{"ActorSpin", func() error { return cli.ActorSpin() }},
	)
	for i := 0; i < loadCfg.WarmupSpins; i++ {
		steps = append(steps, step{"ActorSpin", func() error { return cli.ActorSpin() }})
	}

	for _, s := range steps {
		apiStart := time.Now()
		if err := s.fn(); err != nil {
			recordAPIMetrics(s.name, apiStart, true)
			recordError(startTime)
			clog.Errorf("[%s] Failed to %s: %v", userName, s.name, err)
			cli.Disconnect()
			return nil
		}
		recordAPIMetrics(s.name, apiStart, false)
	}

	recordSuccess(startTime)
	atomic.AddInt64(&onlineCount, 1)
	return cli
}

func recordSuccess(startTime time.Time) {
	latencyMs := time.Since(startTime).Milliseconds()
	atomic.AddInt64(&successCount, 1)
	atomic.AddInt64(&totalLatencyMs, latencyMs)
	for {
		cur := atomic.LoadInt64(&maxLatencyMs)
		if latencyMs <= cur || atomic.CompareAndSwapInt64(&maxLatencyMs, cur, latencyMs) {
			break
		}
	}
}

func recordError(startTime time.Time) {
	latencyMs := time.Since(startTime).Milliseconds()
	atomic.AddInt64(&errorCount, 1)
	atomic.AddInt64(&totalLatencyMs, latencyMs)
	for {
		cur := atomic.LoadInt64(&maxLatencyMs)
		if latencyMs <= cur || atomic.CompareAndSwapInt64(&maxLatencyMs, cur, latencyMs) {
			break
		}
	}
}

func PrintStatusLoop(stop chan struct{}) {
	ticker := time.NewTicker(loadCfg.PrintInterval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			PrintStatus()
		}
	}
}

func PrintStatus() {
	online := atomic.LoadInt64(&onlineCount)
	total := atomic.LoadInt64(&totalRequests)
	errors := atomic.LoadInt64(&errorCount)
	spins := atomic.LoadInt64(&spinRequests)
	spinErrs := atomic.LoadInt64(&spinErrors)
	sm := getSystemMetrics()
	elapsed := time.Since(testStartTime).Seconds()

	var errRate float64
	if total > 0 {
		errRate = float64(errors) / float64(total) * 100
	}

	clog.Infof("[%.0fs] Online:%d | Total:%d | Errors:%d(%.1f%%) | Spins:%d(Err:%d) | CPU:%.1f%% | Mem:%dMB | GR:%d",
		elapsed, online, total, errors, errRate, spins, spinErrs, sm.CPUPercent, sm.MemUsedMB, sm.GoRoutines)

	// Server pprof
	clog.Info("  Server Nodes:")
	for _, m := range getAllServerMetrics() {
		if m.Online {
			clog.Infof("    %s: Goroutines=%d", m.Name, m.Goroutines)
		}
	}

	// 打印各接口实时 QPS
	PrintAPIMetricsRealtime()
}

func PrintSummary() {
	online := atomic.LoadInt64(&onlineCount)
	total := atomic.LoadInt64(&totalRequests)
	success := atomic.LoadInt64(&successCount)
	errors := atomic.LoadInt64(&errorCount)
	maxLat := atomic.LoadInt64(&maxLatencyMs)
	spins := atomic.LoadInt64(&spinRequests)
	spinErrs := atomic.LoadInt64(&spinErrors)

	var avgLat int64
	if success+errors > 0 {
		avgLat = atomic.LoadInt64(&totalLatencyMs) / (success + errors)
	}
	var successRate, spinSuccessRate float64
	if total > 0 {
		successRate = float64(success) / float64(total) * 100
	}
	if spins > 0 {
		spinSuccessRate = float64(spins-spinErrs) / float64(spins) * 100
	}

	sm := getSystemMetrics()
	fmt.Println()
	clog.Info("========== Load Test Summary ==========")
	clog.Infof("Max Online: %d | Total Logins: %d | Success: %.1f%%", online, total, successRate)
	clog.Infof("Avg Latency: %dms | Max Latency: %dms | Errors: %d", avgLat, maxLat, errors)
	clog.Infof("Spins: %d | Spin Success: %.1f%% | Spin Errors: %d", spins, spinSuccessRate, spinErrs)
	clog.Infof("Duration: %.1fs | Hold: %v", time.Since(testStartTime).Seconds(), loadCfg.HoldDuration)
	clog.Info("----------------------------------------")
	clog.Infof("Robot: CPU=%.1f%% | Mem=%dMB/%dMB | GR=%d | Heap=%dMB",
		sm.CPUPercent, sm.MemUsedMB, sm.MemTotalMB, sm.GoRoutines, sm.HeapAllocMB)
	clog.Info("----------------------------------------")
	clog.Info("Server Nodes (pprof):")
	for _, m := range getAllServerMetrics() {
		if m.Online {
			clog.Infof("  %s: Goroutines=%d", m.Name, m.Goroutines)
		} else {
			clog.Infof("  %s: OFFLINE", m.Name)
		}
	}
	clog.Info("----------------------------------------")
	clog.Info("Per-API Stats:")
	PrintAPIMetrics()
	clog.Info("========================================")
}

func PrintAPIMetrics() {
	apiMetricsMu.RLock()
	defer apiMetricsMu.RUnlock()

	for _, name := range apiOrder {
		m := apiMetrics[name]
		if m == nil {
			continue
		}
		cnt, tot, max, errs := m.GetStats()
		if cnt == 0 {
			continue
		}

		var avg int64
		if cnt > 0 {
			avg = tot / cnt
		}

		var errRate float64
		if cnt > 0 {
			errRate = float64(errs) / float64(cnt) * 100
		}

		// 计算总体平均 QPS
		// 计算活跃时长时的逻辑
		firstTime := atomic.LoadInt64(&firstRequestTime)
		lastTime := atomic.LoadInt64(&lastRequestTime)
		activeDuration := time.Duration(lastTime - firstTime).Seconds()
		elapsed := time.Since(testStartTime).Seconds()
		var avgQPS float64
		if elapsed > 0 {
			avgQPS = float64(cnt) / activeDuration
		}

		// 获取实时 QPS (滑动窗口)
		realtimeQPS := m.GetRealtimeQPS()

		clog.Infof("  %-18s: Avg=%4dms Max=%4dms Cnt=%6d Err=%4d(%.1f%%) AvgQPS=%.1f RealtimeQPS=%.1f",
			name, avg, max, cnt, errs, errRate, avgQPS, realtimeQPS)
	}
}

// PrintAPIMetricsRealtime 打印实时 QPS 统计 (用于定时状态输出)
func PrintAPIMetricsRealtime() {
	apiMetricsMu.RLock()
	defer apiMetricsMu.RUnlock()

	clog.Info("  API Realtime QPS (10s window):")
	for _, name := range apiOrder {
		m := apiMetrics[name]
		if m == nil {
			continue
		}
		cnt, _, _, errs := m.GetStats()
		if cnt == 0 {
			continue
		}

		realtimeQPS := m.GetRealtimeQPS()
		lastSecQPS := m.GetLastSecondQPS()

		clog.Infof("    %-18s: LastSec=%6.0f/s Avg10s=%6.1f/s Total=%6d Err=%4d",
			name, lastSecQPS, realtimeQPS, cnt, errs)
	}
}

func RegisterDevAccount(url string, accounts map[string]string) {
	reqURL := fmt.Sprintf("%s/register", url)
	accountChan := make(chan struct{}, loadCfg.BatchSize)
	var registWait sync.WaitGroup
	for k, v := range accounts {
		registWait.Add(1)
		accountChan <- struct{}{}
		go func(account, password string) {
			defer registWait.Done()
			defer func() { <-accountChan }()
			jsonBytes, _, err := chttp.GlobalClientGet(reqURL, map[string]string{"account": account, "password": password})
			if err != nil {
				return
			}
			rsp := &code.Result{}
			_ = jsoniter.Unmarshal(jsonBytes, rsp)
		}(k, v) // 同时传递 k 和 v
	}
	registWait.Wait()
	close(accountChan)
}
