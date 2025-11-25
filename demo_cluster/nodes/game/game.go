/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-24 17:48:36
 * @FilePath: /examples/demo_cluster/nodes/game/game.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package game

import (
	"github.com/cherry-game/cherry"
	cherrySnowflake "github.com/cherry-game/cherry/extend/snowflake"
	cstring "github.com/cherry-game/cherry/extend/string"
	cherryUtils "github.com/cherry-game/cherry/extend/utils"
	cherryCron "github.com/cherry-game/components/cron"
	cherryGops "github.com/cherry-game/components/gops"
	checkCenter "github.com/cherry-game/examples/demo_cluster/internal/component/check_center"
	checkConfigVersion "github.com/cherry-game/examples/demo_cluster/internal/component/check_config_version"
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/module/player"
	slots "github.com/cherry-game/examples/demo_cluster/nodes/game/module/slots/room"

	cdiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryETCD "github.com/cherry-game/components/etcd"
	cherryGORM "github.com/cherry-game/examples/demo_cluster/internal/component/pg_gorm"
)

func Run(profileFilePath, nodeID string) {
	if !cherryUtils.IsNumeric(nodeID) {
		panic("node parameter must is number.")
	}

	// snowflake global id
	serverId, _ := cstring.ToInt64(nodeID)
	cherrySnowflake.SetDefaultNode(serverId)

	// 配置cherry引擎
	app := cherry.Configure(profileFilePath, nodeID, false, cherry.Cluster)
	// 注册etcd组件（已修复protobuf版本冲突）
	cdiscovery.Register(cherryETCD.New())
	// diagnose
	app.Register(cherryGops.New())
	// 注册调度组件
	app.Register(cherryCron.New())
	// 注册数据配置组件
	app.Register(data.New())
	// 注册检测中心节点组件，确认中心节点启动后，再启动当前节点
	app.Register(checkCenter.New())

	// 注册gorm组件，数据库具体配置请查看 config/demo-gorm.json文件
	app.Register(cherryGORM.NewComponent())
	// 注册db组件
	app.Register(db.New())
	//注册配置etcd缓存组件
	app.Register(checkConfigVersion.New("/cherry/config/slots/levels/", configCacheSlots.GetInstance()))

	app.AddActors(
		&player.ActorPlayers{},
		&slots.ActorRooms{},
	)

	app.Startup()
}
