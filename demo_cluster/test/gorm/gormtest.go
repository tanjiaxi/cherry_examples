/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-26 14:51:31
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-30 15:54:54
 * @FilePath: /examples/demo_cluster/test/gormtest.go
 * @Description: GORM 压测工具 - 带详细延迟统计
 */
package main

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	DSN         = "host=10.10.10.251 user=postgres password=postgres dbname=classic_slots port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	Concurrency = 80      // 并发数
	TotalReqs   = 1000000 // 总请求数
)

// 延迟统计
var (
	latencies   []int64           // 存储每次查询的延迟(纳秒)
	latenciesMu sync.Mutex        // 保护 latencies 切片
	totalLatNs  int64             // 总延迟(纳秒)
	minLatNs    int64      = 1e18 // 最小延迟
	maxLatNs    int64             // 最大延迟
)

const TableNameSlotsUser = "newsz_2024.slots_user"

// SlotsUser mapped from table <slots_user>
type SlotsUser struct {
	UserID            int32     `gorm:"column:user_id;primaryKey;autoIncrement:true" json:"user_id"`
	PlayerGroup       int32     `gorm:"column:player_group;comment:用户分组" json:"player_group"`                  // 用户分组
	UserType          int32     `gorm:"column:user_type;comment:用户分类" json:"user_type"`                        // 用户分类
	UserName          string    `gorm:"column:user_name;comment:名字" json:"user_name"`                          // 名字
	Money             float64   `gorm:"column:money;comment:金币" json:"money"`                                  // 金币
	Diamond           int64     `gorm:"column:diamond;comment:钻石" json:"diamond"`                              // 钻石
	ScratchNum        int32     `gorm:"column:scratch_num;comment:刮刮卡" json:"scratch_num"`                     // 刮刮卡
	UserLevel         int32     `gorm:"column:user_level;comment:等级" json:"user_level"`                        // 等级
	CurExp            string    `gorm:"column:cur_exp;default:0;comment:当前经验值" json:"cur_exp"`                 // 当前经验值
	HonorLevel        int32     `gorm:"column:honor_level;comment:vip" json:"honor_level"`                     // vip
	HonorExp          int64     `gorm:"column:honor_exp;comment:当前VIP等级经验值" json:"honor_exp"`                  // 当前VIP等级经验值
	Sex               int32     `gorm:"column:sex;comment:性别1男2女" json:"sex"`                                  // 性别1男2女
	Icon              string    `gorm:"column:icon;comment:头像地址" json:"icon"`                                  // 头像地址
	ThirdIcon         string    `gorm:"column:third_icon;comment:第三方头像地址" json:"third_icon"`                   // 第三方头像地址
	MobilePhone       string    `gorm:"column:mobile_phone;comment:手机" json:"mobile_phone"`                    // 手机
	Birthday          string    `gorm:"column:birthday;default:0;comment:生日" json:"birthday"`                  // 生日
	AdIsOpen          bool      `gorm:"column:ad_is_open;default:true;comment:广告是否开启" json:"ad_is_open"`       // 广告是否开启
	AdDisplayNum      int32     `gorm:"column:ad_display_num;comment:广告观看完成总次数" json:"ad_display_num"`         // 广告观看完成总次数
	AdGainMoney       int64     `gorm:"column:ad_gain_money;comment:广告获得金币" json:"ad_gain_money"`              // 广告获得金币
	PayAllMoney       float32   `gorm:"column:pay_all_money;comment:付费总额(分)" json:"pay_all_money"`             // 付费总额(分)
	PayNum            int32     `gorm:"column:pay_num;comment:付费次数" json:"pay_num"`                            // 付费次数
	IsReplenish       bool      `gorm:"column:is_replenish;comment:是否补全信息" json:"is_replenish"`                // 是否补全信息
	LastLoginType     int32     `gorm:"column:last_login_type;comment:最后登录方式" json:"last_login_type"`          // 最后登录方式
	LastLoginPlatform int32     `gorm:"column:last_login_platform;comment:最后登录的平台" json:"last_login_platform"` // 最后登录的平台
	LoginCount        int32     `gorm:"column:login_count;comment:登录次数" json:"login_count"`                    // 登录次数
	LogoutTime        time.Time `gorm:"column:logout_time;comment:退出游戏时间" json:"logout_time"`                  // 退出游戏时间
	CreateTime        time.Time `gorm:"column:create_time;comment:创建时间" json:"create_time"`                    // 创建时间
	LoginTime         time.Time `gorm:"column:login_time;comment:最后登录时间录时间" json:"login_time"`                 // 最后登录时间录时间
	LoginStamp        time.Time `gorm:"column:login_stamp" json:"login_stamp"`
	AdStartNum        int32     `gorm:"column:ad_start_num;comment:广告点击次数" json:"ad_start_num"` // 广告点击次数
	BindEmail         string    `gorm:"column:bind_email;comment:绑定邮箱" json:"bind_email"`       // 绑定邮箱
	AdAllMoney        float32   `gorm:"column:ad_all_money" json:"ad_all_money"`
	LastLoginTime     time.Time `gorm:"column:last_login_time;comment:上一次登录时间" json:"last_login_time"` // 上一次登录时间
	AdjustIsGroup     int32     `gorm:"column:adjust_is_group" json:"adjust_is_group"`
	BeforePlayerGroup int32     `gorm:"column:before_player_group" json:"before_player_group"`
	AdSafeMark        int32     `gorm:"column:ad_safe_mark;default:1" json:"ad_safe_mark"`
	ExpPercent        float64   `gorm:"column:exp_percent;default:-1;comment:经验比" json:"exp_percent"` // 经验比
}

// TableName SlotsUser's table name
func (*SlotsUser) TableName() string {
	return TableNameSlotsUser
}

func main() {
	// 1. 初始化 GORM
	db, err := gorm.Open(postgres.Open(DSN), &gorm.Config{
		Logger:      logger.Default.LogMode(logger.Silent),
		PrepareStmt: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	// 2. 获取底层 sql.DB 以设置连接池
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}

	sqlDB.SetMaxOpenConns(Concurrency)
	sqlDB.SetMaxIdleConns(Concurrency)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// 预分配延迟切片
	latencies = make([]int64, 0, TotalReqs)

	start := time.Now()
	var wg sync.WaitGroup
	var finished int64

	// 限制并发的通道 (信号量)
	sem := make(chan struct{}, Concurrency)

	fmt.Println("开始 GORM 压测...")
	fmt.Printf("配置: 并发=%d, 总请求=%d\n\n", Concurrency, TotalReqs)

	for i := 0; i < TotalReqs; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			var device SlotsUser
			targetUserID := 10029 + idx

			// ⏱️ 记录单次查询开始时间
			queryStart := time.Now()

			// 执行查询
			err := db.Where("user_id = ?", targetUserID).Find(&device).Error

			// ⏱️ 计算单次查询延迟
			queryLatency := time.Since(queryStart).Nanoseconds()

			// 更新延迟统计
			atomic.AddInt64(&totalLatNs, queryLatency)

			// 更新最小延迟 (CAS)
			for {
				cur := atomic.LoadInt64(&minLatNs)
				if queryLatency >= cur || atomic.CompareAndSwapInt64(&minLatNs, cur, queryLatency) {
					break
				}
			}
			// 更新最大延迟 (CAS)
			for {
				cur := atomic.LoadInt64(&maxLatNs)
				if queryLatency <= cur || atomic.CompareAndSwapInt64(&maxLatNs, cur, queryLatency) {
					break
				}
			}

			// 存储延迟用于计算百分位
			latenciesMu.Lock()
			latencies = append(latencies, queryLatency)
			latenciesMu.Unlock()

			if err != nil {
				log.Println("Query error:", err)
			} else {
				atomic.AddInt64(&finished, 1)
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)

	// 计算百分位延迟
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	p50 := latencies[len(latencies)*50/100]
	p90 := latencies[len(latencies)*90/100]
	p95 := latencies[len(latencies)*95/100]
	p99 := latencies[len(latencies)*99/100]

	avgLatNs := atomic.LoadInt64(&totalLatNs) / int64(len(latencies))

	fmt.Printf("\n====== 压测结果 (GORM) ======\n")
	fmt.Printf("并发数: %d\n", Concurrency)
	fmt.Printf("完成请求: %d\n", finished)
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("QPS: %.2f\n", float64(finished)/duration.Seconds())

	fmt.Printf("\n====== 单次查询延迟统计 ======\n")
	fmt.Printf("最小延迟: %.3f ms\n", float64(minLatNs)/1e6)
	fmt.Printf("最大延迟: %.3f ms\n", float64(maxLatNs)/1e6)
	fmt.Printf("平均延迟: %.3f ms\n", float64(avgLatNs)/1e6)
	fmt.Printf("P50 延迟: %.3f ms\n", float64(p50)/1e6)
	fmt.Printf("P90 延迟: %.3f ms\n", float64(p90)/1e6)
	fmt.Printf("P95 延迟: %.3f ms\n", float64(p95)/1e6)
	fmt.Printf("P99 延迟: %.3f ms\n", float64(p99)/1e6)

	fmt.Printf("\n====== 整体响应时间 (含排队) ======\n")
	fmt.Printf("平均响应时间: %v\n", duration/time.Duration(finished)*time.Duration(Concurrency))
}
