/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-25 15:57:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-07-07 10:14:29
 * @FilePath: /examples/demo_cluster/nodes/web/web.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package web

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cherry-game/cherry"
	cherryFile "github.com/cherry-game/cherry/extend/file"
	cherryCron "github.com/cherry-game/components/cron"
	cherryGin "github.com/cherry-game/components/gin"
	checkCenter "github.com/cherry-game/examples/demo_cluster/internal/component/check_center"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	"github.com/cherry-game/examples/demo_cluster/internal/component/runtime_monitor"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/nodes/web/controller"
	"github.com/cherry-game/examples/demo_cluster/nodes/web/sdk"
	"github.com/gin-gonic/gin"

	cdiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryETCD "github.com/cherry-game/components/etcd"
)

func Run(profileFilePath, nodeID string) {
	// 配置cherry引擎,加载profile配置文件
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)
	// 注册etcd组件（已修复protobuf版本冲突）
	cdiscovery.Register(cherryETCD.New())
	// 注册调度组件
	app.Register(cherryCron.New())

	// 注册检查中心服是否启动组件
	app.Register(checkCenter.New())

	// 注册数据配表组件
	app.Register(data.New())

	// 加载http server组件
	app.Register(httpServerComponent(app.Address()))

	// 加载sdk逻辑
	sdk.Init(app)
	// 注册服务端 QPS 统计组件
	metricsComponent := metrics.New()
	app.Register(metricsComponent)
	metrics.SetGlobal(metricsComponent)

	// 注册 runtime monitor 组件
	runtimeMonitor := runtime_monitor.New(nodeID)
	app.Register(runtimeMonitor)
	runtime_monitor.SetGlobal(runtimeMonitor) // 设置全局访问
	// 启动cherry引擎
	app.Startup()
}

func httpServerComponent(addr string) *cherryGin.Component {
	gin.SetMode(gin.DebugMode)

	// new http server
	httpServer := cherryGin.NewHttp("http_server", addr)
	httpServer.Use(cherryGin.Cors())

	// http server使用gin组件搭建，这里增加一个RecoveryWithZap中间件
	httpServer.Use(cherryGin.RecoveryWithZap(true))

	// ✅ 获取可执行文件所在目录
	execPath, err := os.Executable()
	if err != nil {
		panic(fmt.Sprintf("failed to get executable path: %v", err))
	}
	execDir := filepath.Dir(execPath)

	// ✅ 构建绝对路径
	staticDir := filepath.Join(execDir, "web/static")
	viewDir := filepath.Join(execDir, "web/view")
	// 映射h5客户端静态文件到static目录，例如：http://127.0.0.1/static/protocol.js
	httpServer.Static("/static", staticDir)

	// 加载./view目录的html模板文件(详情查看gin文档)
	viewFiles := cherryFile.WalkFiles(viewDir, ".html")
	if len(viewFiles) < 1 {
		panic("view files not found.")
	}
	httpServer.LoadHTMLFiles(viewFiles...)

	// 注册 controller
	httpServer.Register(new(controller.Controller))

	return httpServer
}
