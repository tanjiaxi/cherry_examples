# write_Behind_queue Component

异步批量写入组件，支持分级持久化和批量写入优化。

## Features

- ✅ 分级持久化（按时间间隔批量写入）
- ✅ 多队列并发（按 PlayerID 分片）
- ✅ 去重合并（相同 key 只保留最新值）,只需更新一次
- ✅ 优雅关闭（保证数据不丢失）
- ✅ 可插拔后端（Redis/MongoDB/MySQL等自己实现不同数据库）

## Installation

```bash
go get github.com/cherry-game/components/write_behind_queue@latest

## 使用实例

	dbqueue "github.com/cherry-game/examples/demo_cluster/internal/component/write_behind_queue"
	// 2. 各个业务表的队列精细配置
	configs := map[string]dbqueue.TableConfig{
		"player_data": {
			QueueCount:    4,                // 该表开 4 个后台分流队列，时序按 PlayerID Hashing
			QueueSize:     2048,             // Channel 缓冲区
			BulkSize:      1200,             // 凑齐 100 条就批量保存
			FlushInterval: 5 * time.Second, // 或者没凑够，到了 5 秒也保存一次
		},
	}
	// 实例化数据库组件(redis),自己实现得到
	redisCompent := cherryRedis.NewRedisCompent()
	// redisCompent这里本来是个组件,但是Init()在Startup后面执行
	redisCompent.Init()
	saver := dbqueue.NewRedisBackend(redisCompent.GetDb())
    //注册write_behind_queue_
	dbQueueComponent := dbqueue.NewDBWriteQueueComponent(configs, saver)
	app.Register(dbQueueComponent)
