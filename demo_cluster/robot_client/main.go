package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
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

// ==================== 配置变量 ====================
var (
	maxRobotNum    = 8000                       // 最大机器人数
	batchSize      = 800                        // 每批启动数量
	batchInterval  = 1 * time.Millisecond       // 批次间隔
	errorThreshold = 0.1                        // 错误率阈值 (10%)
	printInterval  = 5 * time.Second            // 状态打印间隔
	holdDuration   = 60 * time.Second           // 保持连接时间
	spinInterval   = 500 * time.Millisecond     // Spin 请求间隔
	url            = "http://10.10.10.251:8081" // web node

	pid                   = "2126001" // sdk包id
	printLog              = false
	useServerList         = true  // 是否使用 serverList 接口获取地址
	defaultAreaId   int32 = 1     // 默认区ID
	defaultServerId int32 = 10001 // 默认服ID（0表示自动选择）
	useWebSocket          = true  // 使用 WebSocket 连接（serverList返回的是ws地址）
	// 备用配置（当 useServerList=false 时使用）
	fallbackAddr = "10.10.10.251:10010" // 备用网关地址（TCP）
)

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
	// 初始化日志：同时输出到控制台和文件
	initLogger()

	testStartTime = time.Now()
	initAPIMetrics() // 初始化 API 指标

	clog.Info("========== Load Test Starting ==========")
	clog.Infof("Config: maxRobots=%d, batchSize=%d, holdDuration=%v, spinInterval=%v",
		maxRobotNum, batchSize, holdDuration, spinInterval)
	clog.Infof("Connection: useServerList=%v, useWebSocket=%v", useServerList, useWebSocket)

	accounts := make(map[string]string)
	for i := 1; i <= maxRobotNum; i++ {
		accounts[fmt.Sprintf("loadtest%d", i)] = fmt.Sprintf("loadtest%d", i)
	}
	// RegisterDevAccount(url, accounts)
	// return
	stopPrinting := make(chan struct{})
	go PrintStatusLoop(stopPrinting)

	RunLoadTest(accounts)
	clog.Infof("Starting continuous Spin for %v...", holdDuration)
	// RunContinuousSpin()
	PrintSummary()
	close(stopPrinting)
	time.Sleep(100000 * time.Millisecond)
	DisconnectAllRobots()
}

// fetchServerList 获取区服列表
func fetchServerList(cli *robotclient.Robot) error {
	// cli := robotclient.New(pomeloClient.New())
	serverList, err := cli.GetServerList(url, pid)
	if err != nil {
		return err
	}

	// serverListMu.Lock()
	cachedServerList = serverList
	// serverListMu.Unlock()

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

// getGateAddrAndServerId 获取Gate地址和ServerId
func getGateAddrAndServerId() (gateAddr string, serverId int32) {
	serverListMu.RLock()
	defer serverListMu.RUnlock()
	return fallbackAddr, defaultServerId
	if cachedServerList == nil || len(cachedServerList.Areas) == 0 {
		// 使用备用配置
		return fallbackAddr, defaultServerId
	}

	// 查找指定区
	var targetArea *robotclient.AreaInfo
	for _, area := range cachedServerList.Areas {
		if area.AreaId == defaultAreaId {
			targetArea = area
			break
		}
	}
	if targetArea == nil {
		targetArea = cachedServerList.Areas[0]
	}

	// 查找指定服
	var targetServer *robotclient.ServerInfo
	for _, server := range cachedServerList.Servers {
		if server.AreaId == targetArea.AreaId {
			if defaultServerId == 0 || server.ServerId == defaultServerId {
				targetServer = server
				break
			}
		}
	}
	if targetServer == nil {
		// 找该区的第一个服
		for _, server := range cachedServerList.Servers {
			if server.AreaId == targetArea.AreaId {
				targetServer = server
				break
			}
		}
	}

	if targetServer == nil {
		return fallbackAddr, defaultServerId
	}

	return targetArea.Gate, targetServer.ServerId
}

func RunLoadTest(accounts map[string]string) {
	type acc struct{ user, pass string }
	list := make([]acc, 0, len(accounts))
	for u, p := range accounts {
		list = append(list, acc{u, p})
	}
	totalBatches := (len(list) + batchSize - 1) / batchSize
	clog.Infof("Starting: %d robots in %d batches", len(list), totalBatches)

	for batch := 0; batch < totalBatches; batch++ {
		if atomic.LoadInt32(&stopSpawning) == 1 {
			break
		}
		start, end := batch*batchSize, (batch+1)*batchSize
		if end > len(list) {
			end = len(list)
		}
		clog.Infof("Batch %d/%d (%d robots)", batch+1, totalBatches, end-start)

		var wg sync.WaitGroup
		for _, a := range list[start:end] {
			wg.Add(1)
			go func(u, p string) {
				defer wg.Done()
				if robot := RunRobotWithMetrics(url, pid, u, p, printLog); robot != nil {
					connectedRobotsMu.Lock()
					connectedRobots = append(connectedRobots, robot)
					connectedRobotsMu.Unlock()
				}
			}(a.user, a.pass)
		}
		wg.Wait()

		if t, e := atomic.LoadInt64(&totalRequests), atomic.LoadInt64(&errorCount); t > 0 && float64(e)/float64(t) > errorThreshold {
			clog.Warnf("Error rate exceeds threshold, stopping")
			atomic.StoreInt32(&stopSpawning, 1)
			break
		}
		if batch < totalBatches-1 {
			time.Sleep(batchInterval)
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
	clog.Infof("Spinning with %d robots for %v", len(robots), holdDuration)

	var wg sync.WaitGroup
	stopTime := time.Now().Add(holdDuration)
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
				time.Sleep(spinInterval)
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
	// 如果使用 serverList，先获取区服列表
	// if useServerList {
	// 	if err := fetchServerList(cli); err != nil {
	// 		clog.Errorf("Failed to fetch server list: %v", err)
	// 		clog.Info("Falling back to default address")
	// 	} else {
	// 		//  printServerList()
	// 	}
	// }
	// 获取Gate地址和ServerId
	gateAddr, serverId := getGateAddrAndServerId()

	// 构建步骤列表
	var steps []struct {
		name string
		fn   func() error
	}

	steps = append(steps, struct {
		name string
		fn   func() error
	}{"GetToken", func() error { return cli.GetToken(url, pid, userName, password) }})

	// 根据配置选择连接方式
	if useServerList && useWebSocket {
		steps = append(steps, struct {
			name string
			fn   func() error
		}{"ConnectWS", func() error { return cli.ConnectToWebSocket(gateAddr) }})
	} else {
		steps = append(steps, struct {
			name string
			fn   func() error
		}{"ConnectTCP", func() error { return cli.ConnectToTCP(gateAddr) }})
	}

	steps = append(steps,
		struct {
			name string
			fn   func() error
		}{"UserLogin", func() error { return cli.UserLogin(serverId) }},
		struct {
			name string
			fn   func() error
		}{"PlayerSelect", func() error { return cli.PlayerSelect() }},
		struct {
			name string
			fn   func() error
		}{"ActorCreate", func() error { return cli.ActorCreate() }},
		struct {
			name string
			fn   func() error
		}{"ActorEnter", func() error { return cli.ActorEnter() }},
		struct {
			name string
			fn   func() error
		}{"ActorEnterMachine", func() error { return cli.ActorEnterEnterMachine() }},
		struct {
			name string
			fn   func() error
		}{"ActorMachine", func() error { return cli.ActorMachine() }},
		struct {
			name string
			fn   func() error
		}{"ActorSpin", func() error { return cli.ActorSpin() }},
	)
	for i := 0; i < 100; i++ {
		steps = append(steps, struct {
			name string
			fn   func() error
		}{"ActorSpin", func() error { return cli.ActorSpin() }})
	}

	for _, step := range steps {
		apiStart := time.Now()
		if err := step.fn(); err != nil {
			recordAPIMetrics(step.name, apiStart, true)
			recordError(startTime)
			clog.Errorf("Failed to %s: %v", step.name, err)
			return nil
		}
		recordAPIMetrics(step.name, apiStart, false)
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
	ticker := time.NewTicker(printInterval)
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
	clog.Infof("Duration: %.1fs | Hold: %v", time.Since(testStartTime).Seconds(), holdDuration)
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
	accountChan := make(chan struct{}, batchSize)
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
