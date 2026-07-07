package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof" // 1. 依然需要导入这个包，来自动注册 pprof 路由
	"os"
	"path/filepath"
	"syscall"
	"time"

	cherryConst "github.com/cherry-game/cherry/const"
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
	"game":   ":6060",
	"gate":   ":6061",
	"web":    ":6062",
	"center": "0.0.0.0:6063",
	"master": ":6064",
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
			gameCommand(),
		},
	}
	_ = app.Run(os.Args)
}

// startPprofServer 为指定节点类型启动 pprof 服务器
func startPprofServer(nodeType string) {
	port, ok := pprofPorts[nodeType]
	if !ok {
		port = ":6060" // 默认端口
	}
	fmt.Printf("Starting pprof server for %s on %s\n", nodeType, port)
	go func() {
		if err := http.ListenAndServe(port, nil); err != nil {
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
