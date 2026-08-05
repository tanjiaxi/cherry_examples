/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-07-16 11:50:16
 * @FilePath: /examples/demo_cluster/nodes/game/game.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package game

import (
	"time"

	"github.com/cherry-game/cherry"
	cherrySnowflake "github.com/cherry-game/cherry/extend/snowflake"
	cstring "github.com/cherry-game/cherry/extend/string"
	cherryUtils "github.com/cherry-game/cherry/extend/utils"
	cherryCron "github.com/cherry-game/components/cron"
	cherryGops "github.com/cherry-game/components/gops"
	dbqueue "github.com/cherry-game/customize/components/write_behind_queue"
	checkCenter "github.com/cherry-game/examples/demo_cluster/internal/component/check_center"
	checkConfigVersion "github.com/cherry-game/examples/demo_cluster/internal/component/check_config_version"
	commonDb "github.com/cherry-game/examples/demo_cluster/internal/component/db"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	"github.com/cherry-game/examples/demo_cluster/internal/component/outbox"
	"github.com/cherry-game/examples/demo_cluster/internal/component/runtime_monitor"
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/module/player"
	slotsRoom "github.com/cherry-game/examples/demo_cluster/nodes/game/module/slots/room"

	cdiscovery "github.com/cherry-game/cherry/net/discovery"
	cherryETCD "github.com/cherry-game/components/etcd"
	cherryGORM "github.com/cherry-game/examples/demo_cluster/internal/component/pg_gorm"
	cherryRedis "github.com/cherry-game/examples/demo_cluster/internal/component/redis"
	slotsLeveCore "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/core"
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
	// 注册检测中心节点组件，确认中心节点启动后，再启动当前节点
	app.Register(checkCenter.New())

	// 注册gorm组件，数据库具体配置请查看 config/demo-gorm.json文件
	app.Register(cherryGORM.NewComponent())
	// 注册数据配置组件
	app.Register(data.New())
	// 注册节点db组件
	app.Register(db.New())
	// 注册公共db组件
	app.Register(commonDb.New())
	// Must be registered after GORM/common DB.  The relay runs outside actors
	// and only dispatches already committed Outbox rows.
	app.Register(outbox.NewComponent())

	// 注册服务端 QPS 统计组件
	metricsComponent := metrics.New()
	app.Register(metricsComponent)
	metrics.SetGlobal(metricsComponent)

	// 注册配置etcd缓存组件
	app.Register(checkConfigVersion.New("/cherry/config/slots/levels/", configCacheSlots.GetInstance()))
	// 注册关卡相关逻辑
	app.Register(slotsLeveCore.New())
	// 注册心跳组件，定时向Center发送心跳
	// app.Register(heartbeat.New("game"))

	// 注册redis组件
	// app.Register(cherryRedis.NewRedisCompent())

	// 2. 各个业务表的队列精细配置
	configs := map[string]dbqueue.TableConfig{
		"classic_slots_user_room": {
			QueueCount:      4,                  // 该表开 4 个后台分流队列，时序按 PlayerID Hashing
			QueueSize:       2048,               // Channel 缓冲区
			BulkSize:        1200,               // 凑齐 100 条就批量保存
			FlushInterval:   2000 * time.Second, // 或者没凑够，到了 3 秒也保存一次
			StopBulkSize:    2000,
			WriteTimeout:    3 * time.Second,  // 写入数据库的超时时间,根据数据库的响应时间
			ShutdownTimeout: 15 * time.Second, // 关闭超时时间
		},
	}
	// 注册db写入队列组件
	redisCompent := cherryRedis.NewRedisCompent()
	// redisCompent这里本来是个组件,但是Init()在Startup后面执行
	redisCompent.Init()
	saver := dbqueue.NewRedisBackend(redisCompent.GetDb())
	dbQueueComponent := dbqueue.NewDBWriteQueueComponent(configs, saver)
	app.Register(dbQueueComponent)
	// 注册 runtime monitor 组件
	runtimeMonitor := runtime_monitor.New(nodeID)
	app.Register(runtimeMonitor)
	runtime_monitor.SetGlobal(runtimeMonitor) // 设置全局访问
	app.AddActors(
		&player.ActorPlayers{},
		&slotsRoom.ActorRooms{},
	)
	app.Startup()
}
