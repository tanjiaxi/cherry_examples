/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-26 14:51:31
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-26 16:22:26
 * @FilePath: /examples/demo_cluster/test/gormtest.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cherry-game/examples/demo_cluster/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	// 注意：如果你之前提到有网络延迟，想减少交互次数，可以在 DSN 后面尝试加上 prefer_simple_protocol=true
	DSN         = "host=10.10.10.251 user=postgres password=postgres dbname=classic_slots port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	Concurrency = 1000  // 并发数
	TotalReqs   = 10000 // 总请求数
)

func main() {
	// 1. 初始化 GORM
	// 关键：Logger 设置为 Silent，否则打印日志会严重拖慢压测 QPS
	db, err := gorm.Open(postgres.Open(DSN), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
		// PrepareStmt: true, // 进阶优化：如果开启预编译缓存，在网络延迟高时可能会显著提升性能
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. 获取底层 sql.DB 以设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}

	// 设置连接池参数
	sqlDB.SetMaxOpenConns(Concurrency)
	sqlDB.SetMaxIdleConns(Concurrency)
	sqlDB.SetConnMaxLifetime(time.Hour)

	start := time.Now()
	var wg sync.WaitGroup
	var finished int64

	// 限制并发的通道 (信号量)
	sem := make(chan struct{}, Concurrency)

	fmt.Println("开始 GORM 压测...")

	for i := 0; i < TotalReqs; i++ {
		wg.Add(1)
		sem <- struct{}{} // 获取令牌
		go func(currentID int) {
			defer wg.Done()
			defer func() { <-sem }() // 释放令牌

			var device model.SlotsDevice
			targetName := fmt.Sprintf("loadtest%d", currentID)
			// 3. 执行查询
			// 使用 db.Raw 原生 SQL 模式，性能最接近 database/sql
			// 如果用 db.Table("...").Where("...").First(&id) 会多一些反射开销
			db.Table("newsz_2024.slots_device").Where("device_name = ?", targetName).First(&device)
			// err := db.Raw(`SELECT device_id FROM "newsz_2024"."slots_device" WHERE device_name = 'loadtest677' LIMIT 1`).Scan(&id).Error

			if err != nil {
				// 只有错误时才打印，避免刷屏
				log.Println("Query error:", err)
			} else {
				log.Printf("Query result for %s: %v", targetName, device)
				atomic.AddInt64(&finished, 1)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	fmt.Printf("====== 压测结果 (GORM) ======\n")
	fmt.Printf("并发数: %d\n", Concurrency)
	fmt.Printf("完成请求: %d\n", finished)
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("QPS: %.2f\n", float64(finished)/duration.Seconds())
	// 这个平均耗时包含了排队时间
	fmt.Printf("平均响应时间 (含排队): %v\n", duration/time.Duration(finished)*time.Duration(Concurrency))
}
