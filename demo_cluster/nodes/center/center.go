/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-01-12 22:10:23
 * @FilePath: /examples/demo_cluster/nodes/center/center.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package center

import (
	"github.com/cherry-game/cherry"
	cdiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryCron "github.com/cherry-game/components/cron"
	cherryETCD "github.com/cherry-game/components/etcd"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	cherryGORM "github.com/cherry-game/examples/demo_cluster/internal/component/pg_gorm"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/module/account"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/module/location"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/module/ops"
)

func Run(profileFilePath, nodeID string) {

	app := cherry.Configure(
		profileFilePath,
		nodeID,
		false,
		cherry.Cluster,
	)
	// 注册etcd组件（已修复protobuf版本冲突）
	cdiscovery.Register(cherryETCD.New())

	// 注册gorm组件，数据库具体配置请查看 config/demo-gorm.json文件
	app.Register(cherryGORM.NewComponent())
	app.Register(cherryCron.New())
	app.Register(data.New())
	app.Register(db.New())
	// 注册服务端 QPS 统计组件
	metricsComponent := metrics.New()
	app.Register(metricsComponent)
	metrics.SetGlobal(metricsComponent)
	// 注册Actor - 这里才是真正的Actor模型
	app.AddActors(
		&account.ActorAccount{},
		&ops.ActorOps{},
		location.NewActorLocation(), // 玩家位置管理Actor
	)

	app.Startup()
}
