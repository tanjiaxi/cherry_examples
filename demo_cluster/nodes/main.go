package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	_ "net/http/pprof" // 1. 依然需要导入这个包，来自动注册 pprof 路由
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	cherryConst "github.com/cherry-game/cherry/const"
	"github.com/cherry-game/examples/demo_cluster/nodes/activity"
	"github.com/cherry-game/examples/demo_cluster/nodes/center"
	"github.com/cherry-game/examples/demo_cluster/nodes/game"
	"github.com/cherry-game/examples/demo_cluster/nodes/gate"
	"github.com/cherry-game/examples/demo_cluster/nodes/master"
	"github.com/cherry-game/examples/demo_cluster/nodes/web"
	"github.com/urfave/cli/v2"
)

// redirectStderr 使用系统调用重定向 stderr (fd 2) 到指定文件
// 这是唯一能真正重定向 GODEBUG=gctrace=1 输出的方法
func redirectStderr(f *os.File) error {
	return syscall.Dup2(int(f.Fd()), int(os.Stderr.Fd()))
}

// pprof 端口映射：每个节点类型使用不同端口
var pprofPorts = map[string]string{
	"game":     ":6060",
	"activity": ":6065",
	"gate":     ":6061",
	"web":      ":6062",
	"center":   ":6063",
	"master":   ":6064",
}

func main() {
	app := &cli.App{
		Name:        "game cluster node",
		Description: "game cluster node examples",
		Commands: []*cli.Command{
			versionCommand(),
			masterCommand(),
			centerCommand(),
			webCommand(),
			gateCommand(),
			activityCommand(),
			gameCommand(),
		},
	}
	_ = app.Run(os.Args)
}

func activityCommand() *cli.Command {
	return &cli.Command{
		Name:      "activity",
		Usage:     "run activity worker",
		UsageText: "node activity --path=../../config/demo-cluster.json --node=gc-activity-1",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			setupGCLog("activity", node)
			// 修复：pprofPorts 中已经预留了 activity:6065，但此前从未调用
			// startPprofServer("activity")，导致 activity 节点完全没有暴露
			// pprof/trace端点，线上一旦这个节点出问题(它承载了NATS
			// JetStream消费、活动结算等CPU/内存敏感逻辑)将完全无法用
			// go tool pprof 远程抓取诊断。
			startPprofServer("activity")
			activity.Run(path, node)
			return nil
		},
	}
}

// enableContentionProfiling 打开锁/阻塞采样。
// 默认 rate=0 时 pprof 的 block/mutex profile 永远是空的，线上一旦出现
// "锁粒度过大导致大量goroutine互相等待"的问题，没有这两个profile基本无法定位。
// SetMutexProfileFraction(N) 表示大约每 N 次锁竞争事件采样 1 次，
// 生产环境建议给一个较大的采样分母（如100）而不是1，避免额外开销。
func enableContentionProfiling() {
	runtime.SetBlockProfileRate(1000000) // 采样阈值：耗时超过1ms的阻塞事件才计入
	runtime.SetMutexProfileFraction(100) // 大约1/100的锁竞争事件被采样
}

// pprofListenAddr 决定 pprof HTTP 服务监听的地址。
// 生产环境默认只监听 127.0.0.1，避免 /debug/pprof/heap 这类可能包含
// 敏感数据、且能被用来做资源耗尽攻击(如长时间的 profile/trace 采集)的调试接口
// 直接暴露在公网/内网所有人可达的地址上；需要远程访问时通过 SSH 隧道
// (ssh -L 6060:127.0.0.1:6060 user@host) 转发，或显式设置环境变量放开。
func pprofListenAddr(port string) string {
	if host := os.Getenv("PPROF_LISTEN_HOST"); host != "" {
		return host + port
	}
	return "127.0.0.1" + port
}

// startPprofServer 为指定节点类型启动 pprof 服务器
func startPprofServer(nodeType string) {
	enableContentionProfiling()

	port, ok := pprofPorts[nodeType]
	if !ok {
		port = ":6060" // 默认端口
	}
	addr := pprofListenAddr(port)
	fmt.Printf("Starting pprof server for %s on %s (set PPROF_LISTEN_HOST=0.0.0.0 to expose externally)\n", nodeType, addr)
	go func() {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			log.Printf("pprof listen failed: %v", err)
			return
		}
		if err := http.Serve(listener, nil); err != nil {
			log.Printf("pprof server failed: %v", err)
		}
	}()
}

// setupGCLog 设置 GC 日志输出到文件
// 需要配合环境变量 GODEBUG=gctrace=1 使用
// 日志文件格式: logs/gc_{nodeType}_{nodeId}_{timestamp}.log
func setupGCLog(nodeType, nodeId string) {
	// 检查是否启用了 gctrace
	godebug := os.Getenv("GODEBUG")
	if godebug == "" {
		return // 没有设置 GODEBUG，不需要重定向
	}

	// 创建 logs 目录
	logsDir := "logs"
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		log.Printf("Failed to create logs directory: %v", err)
		return
	}

	// 生成日志文件名
	timestamp := time.Now().Format("20060102_150405")
	logFileName := fmt.Sprintf("gc_%s_%s_%s.log", nodeType, nodeId, timestamp)
	logFilePath := filepath.Join(logsDir, logFileName)

	// 打开日志文件
	gcLogFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		log.Printf("Failed to open GC log file: %v", err)
		return
	}

	// 使用 Dup2 重定向文件描述符 2 (stderr) 到日志文件
	// GODEBUG=gctrace=1 直接写入 fd 2，必须用系统调用重定向
	if err := redirectStderr(gcLogFile); err != nil {
		log.Printf("Failed to redirect stderr: %v", err)
		return
	}

	fmt.Printf("GC log enabled: %s (GODEBUG=%s)\n", logFilePath, godebug)
}

func versionCommand() *cli.Command {
	return &cli.Command{
		Name:      "version",
		Aliases:   []string{"ver", "v"},
		Usage:     "view version",
		UsageText: "game cluster node version",
		Action: func(c *cli.Context) error {
			fmt.Println(cherryConst.Version())
			return nil
		},
	}
}

func masterCommand() *cli.Command {
	return &cli.Command{
		Name:      "master",
		Usage:     "run master node",
		UsageText: "node master --path=../../config/demo-cluster.json --node=gc-master",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			setupGCLog("master", node)
			startPprofServer("master")
			master.Run(path, node)
			return nil
		},
	}
}

func centerCommand() *cli.Command {
	return &cli.Command{
		Name:      "center",
		Usage:     "run center node",
		UsageText: "node center --path=../../config/demo-cluster.json --node=gc-center",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			setupGCLog("center", node)
			startPprofServer("center")
			center.Run(path, node)
			return nil
		},
	}
}

func webCommand() *cli.Command {
	return &cli.Command{
		Name:      "web",
		Usage:     "run web node",
		UsageText: "node web --path=../../config/demo-cluster.json --node=gc-web-1",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			setupGCLog("web", node)
			startPprofServer("web")
			web.Run(path, node)
			return nil
		},
	}
}

func gateCommand() *cli.Command {
	return &cli.Command{
		Name:      "gate",
		Usage:     "run gate node",
		UsageText: "node gate --path=../../config/demo-cluster.json --node=gc-gate-1",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			setupGCLog("gate", node)
			startPprofServer("gate")
			gate.Run(path, node)
			return nil
		},
	}
}

func gameCommand() *cli.Command {
	return &cli.Command{
		Name:      "game",
		Usage:     "run game node",
		UsageText: "node game --path=../../config/demo-cluster.json --node=10001",
		Flags:     getFlag(),
		Action: func(c *cli.Context) error {
			path, node := getParameters(c)
			setupGCLog("game", node)
			startPprofServer("game")
			game.Run(path, node)
			return nil
		},
	}
}

func getParameters(c *cli.Context) (path, node string) {
	path = c.String("path")
	node = c.String("node")
	return path, node
}

func getFlag() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{
			Name:     "path",
			Usage:    "profile config path",
			Required: false,
			Value:    "../../config/demo-cluster.json",
		},
		&cli.StringFlag{
			Name:     "node",
			Usage:    "node id name",
			Required: true,
			Value:    "",
		},
	}
}
