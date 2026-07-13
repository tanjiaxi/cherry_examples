package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	_ "net/http/pprof" // 1. 依然需要导入这个包，来自动注册 pprof 路由

	cherryLogger "github.com/cherry-game/cherry/logger"
	"github.com/redis/go-redis/v9"
)

func main() {
	StartPressureTest()
}

// 模拟 10,000 个玩家 Actor 同时触发定时器
func StartPressureTest() {
	startTime := time.Now()
	var wg sync.WaitGroup
	// 初始化一个极小的 Redis 连接池
	rdb := redis.NewClient(&redis.Options{
		Addr:     "10.10.10.251:6379",
		PoolSize: 500, // 核心：只有 5 个连接！
	})
	go func() {
		if err := http.ListenAndServe(":6060", nil); err != nil {
			log.Printf("pprof server failed: %v", err)
		}
	}()
	// 瞬间启动 10,000 个 Goroutine 模拟玩家同步
	for i := 0; i < 100000; i++ {
		wg.Add(1)
		playerID := i
		go func(uid int) {
			// 每个 Goroutine 携带自己的数据快照
			data := map[string]interface{}{"gold": 100, "lv": 1, "gold3": 100, "lv3": 1, "gold2": 100, "lv2": 1, "gold1": 100, "lv1": 1}

			// 设置 1 秒超时
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			jsons, errs := json.Marshal(data) // 转换成JSON返回的是byte[]
			if errs != nil {
				fmt.Println(errs.Error())
			}
			// fmt.Println(string(jsons)) // byte[]转换成string 输出
			// 写入 Redis（并发冲入连接池）
			time.Sleep(10 * time.Second)
			err := rdb.HSet(ctx, "startPressureTest", fmt.Sprintf("player:%d", uid), string(jsons)).Err()
			if err != nil {
				// 打印错误日志，观察雪崩现象
				cherryLogger.Infof("[UID: %d] 同步失败: %v", uid, err)
			}
			wg.Done()
		}(playerID)
	}
	wg.Wait()
	elapsed := time.Since(startTime).Milliseconds()
	cherryLogger.Info(fmt.Sprintf("Total elapsed time: %d ms", elapsed))
	time.Sleep(10 * time.Minute)
}
