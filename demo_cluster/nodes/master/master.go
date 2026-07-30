package master

import (
	"github.com/cherry-game/cherry"
	cdiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryETCD "github.com/cherry-game/components/etcd"
	"github.com/cherry-game/examples/demo_cluster/internal/component/runtime_monitor"
)

func Run(profileFilePath, nodeID string) {

	// 注册etcd组件（已修复protobuf版本冲突）
	cdiscovery.Register(cherryETCD.New())

	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)
	// 注册 runtime monitor 组件
	runtimeMonitor := runtime_monitor.New(nodeID)
	app.Register(runtimeMonitor)
	runtime_monitor.SetGlobal(runtimeMonitor) // 设置全局访问
	app.Startup()
}
