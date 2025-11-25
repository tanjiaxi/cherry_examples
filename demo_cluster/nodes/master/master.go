package master

import (
	"github.com/cherry-game/cherry"
	cdiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryETCD "github.com/cherry-game/components/etcd"
)

func Run(profileFilePath, nodeID string) {
	// 注册etcd组件（已修复protobuf版本冲突）
	cdiscovery.Register(cherryETCD.New())

	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)
	app.Startup()
}
