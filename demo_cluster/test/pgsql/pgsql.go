/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-26 13:55:56
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-29 16:41:33
 * @FilePath: /examples/demo_cluster/test/pgsql/pgsql.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
// /*
//   - @Author: t 921865806@qq.com
//   - @Date: 2025-12-26 13:55:56
//   - @LastEditors: t 921865806@qq.com
//   - @LastEditTime: 2025-12-26 14:48:11
//   - @FilePath: /examples/demo_cluster/test/pgsql_test.go
//   - @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
//     */
package main

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	// "github.com/cherry-game/examples/demo_cluster/tools/data/dal/model"
	// _ "github.com/lib/pq" // 记得导入驱动

	// _ "gorm.io/driver/postgres"

	"github.com/jmoiron/sqlx"
	// _ "github.com/lib/pq"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	DSN         = "host=10.10.10.251 user=postgres password=postgres dbname=classic_slots port=5432 sslmode=disable"
	Concurrency = 20 // 模拟 50 个并发
	TotalReqs   = 10000
)

func main() {
	db, err := sqlx.Connect("pgx", DSN)
	if err != nil {
		log.Fatal(err)
	}
	// 即使这里设大了，如果 docker 慢，一样没用
	db.SetMaxOpenConns(Concurrency)
	db.SetMaxIdleConns(Concurrency)

	start := time.Now()
	var wg sync.WaitGroup
	var finished int64

	// 限制并发的通道
	sem := make(chan struct{}, Concurrency)
	for i := 0; i < TotalReqs; i++ {
		wg.Add(1)
		sem <- struct{}{} // 获取令牌

		// 【关键点1】把 i 作为参数传给匿名函数 (idx)
		// 如果不传，闭包里的 i 会在循环结束后变成最大值
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }() // 释放令牌

			// 【关键点2】构造名字
			// 假设你的数据库里有 loadtest1 到 loadtest1000
			// 使用取模 (%) 运算，保证请求永远落在存在的范围内，避免查不到
			// 假设你数据库只有 1000 个测试用户：
			currentID := (idx % 1000) + 1
			targetName := fmt.Sprintf("loadtest%d", currentID)

			var device SlotsDevice

			// 执行查询
			err := db.Get(&device, `SELECT * FROM "newsz_2024"."slots_device" WHERE device_name = $1 LIMIT 1`, targetName)

			if err != nil {
				// 打印一下到底查哪个名字失败了，方便调试
				log.Printf("Query error for %s: %v", targetName, err)
			} else {
				// log.Printf("Query result for %s: %v", targetName, device)
				atomic.AddInt64(&finished, 1)
			}
		}(i) // 【关键点1】这里传入 i
	}

	wg.Wait()
	duration := time.Since(start)

	fmt.Printf("完成请求: %d\n", finished)
	fmt.Printf("总耗时: %v\n", duration)
	fmt.Printf("QPS: %.2f\n", float64(finished)/duration.Seconds())
	fmt.Printf("平均每次请求耗时: %v\n", duration/time.Duration(finished)*time.Duration(Concurrency)) // 近似计算排队后的平均延迟
}

type SlotsDevice struct {
	DeviceID              int32     `gorm:"column:device_id;primaryKey;autoIncrement:true" db:"device_id"`
	DeviceName            string    `gorm:"column:device_name;not null;comment:用户唯一标示" db:"device_name"` // 用户唯一标示
	EmailID               int32     `gorm:"column:email_id;comment:绑定邮箱的id" db:"email_id"`               // 绑定邮箱的id
	UserID                int32     `gorm:"column:user_id" db:"user_id"`
	IsLogin               bool      `gorm:"column:is_login;comment:登录状态-在线推送（待定）" db:"is_login"`                           // 登录状态-在线推送（待定）
	PushToken             string    `gorm:"column:push_token;comment:推送id-客户推送（第三方SD-OneSingle,客户端传送过来）" db:"push_token"`  // 推送id-客户推送（第三方SD-OneSingle,客户端传送过来）
	Platform              int32     `gorm:"column:platform;not null;comment:平台-ios=1 android=2 web=4 aws=5" db:"platform"` // 平台-ios=1 android=2 web=4 aws=5
	LoginTime             time.Time `gorm:"column:login_time" db:"login_time"`
	CountryCode           string    `gorm:"column:country_code;comment:国家代码(ip去取，库guip)" db:"country_code"`     // 国家代码(ip去取，库guip)
	ClientTimezone        string    `gorm:"column:client_timezone;default:0;comment:用户时区" db:"client_timezone"` // 用户时区
	ClientIP              string    `gorm:"column:client_ip;comment:ip地址-不区分代理地址，如需要由客端传递" db:"client_ip"`      // ip地址-不区分代理地址，如需要由客端传递
	CurVer                string    `gorm:"column:cur_ver;comment:当前版本-最新版？？？" db:"cur_ver"`                    // 当前版本-最新版？？？
	LastVer               string    `gorm:"column:last_ver;comment:上个版本" db:"last_ver"`                         // 上个版本
	CollectNewGift        bool      `gorm:"column:collect_new_gift;comment:是否收集了新设备礼物" db:"collect_new_gift"`   // 是否收集了新设备礼物
	HotVersion            string    `gorm:"column:hot_version;comment:热更新版本" db:"hot_version"`                  // 热更新版本
	InstallChannel        string    `gorm:"column:install_channel;comment:安装渠道" db:"install_channel"`           // 安装渠道
	OsVersion             string    `gorm:"column:os_version;comment:机型" db:"os_version"`                       // 机型
	PhoneNumModel         string    `gorm:"column:phone_num_model" db:"phone_num_model"`
	UserAgreeServiceTerms bool      `gorm:"column:user_agree_service_terms;comment:是否同意了协议条款" db:"user_agree_service_terms"` // 是否同意了协议条款
	AdjustAdid            string    `gorm:"column:adjust_adid;comment:adjust相关统计id" db:"adjust_adid"`                        // adjust相关统计id
	AdjustIdfa            string    `gorm:"column:adjust_idfa;comment:adjust相关id" db:"adjust_idfa"`                          // adjust相关id
	AdjustGpsAdid         string    `gorm:"column:adjust_gps_adid;comment:adjust相关ID" db:"adjust_gps_adid"`                  // adjust相关ID
	FbSource              string    `gorm:"column:fb_source;comment:fb的来源" db:"fb_source"`                                   // fb的来源
	City                  string    `gorm:"column:city;comment:城市" db:"city"`                                                // 城市
	Timezone              string    `gorm:"column:timezone;comment:时区" db:"timezone"`                                        // 时区
	TimezoneOffset        float32   `gorm:"column:timezone_offset" db:"timezone_offset"`
	Latitude              float32   `gorm:"column:latitude;comment:纬度" db:"latitude"`                     // 纬度
	Longitude             float32   `gorm:"column:longitude;comment:经度" db:"longitude"`                   // 经度
	CreateTime            time.Time `gorm:"column:create_time;comment:创建时间" db:"create_time"`             // 创建时间
	PackageName           string    `gorm:"column:package_name;comment:包名" db:"package_name"`             // 包名
	DeviceInfo            string    `gorm:"column:device_info;comment:设备信息" db:"device_info"`             // 设备信息
	IsComment             bool      `gorm:"column:is_comment;comment:是否评论" db:"is_comment"`               // 是否评论
	CommentTimes          int32     `gorm:"column:comment_times;comment:弹评论次数" db:"comment_times"`        // 弹评论次数
	UpdatedTimezone       bool      `gorm:"column:updated_timezone;comment:是否更新时区" db:"updated_timezone"` // 是否更新时区
	URLParams             string    `gorm:"column:url_params" db:"url_params"`
	Referrer              string    `gorm:"column:referrer" db:"referrer"`
	FbQuery               string    `gorm:"column:fb_query;comment:fb参数" db:"fb_query"` // fb参数
	AdjustAdidMd5         string    `gorm:"column:adjust_adid_md5;default:-" db:"adjust_adid_md5"`
	AdjustIdfaMd5         string    `gorm:"column:adjust_idfa_md5;default:-" db:"adjust_idfa_md5"`
	AdjustGpsAdidMd5      string    `gorm:"column:adjust_gps_adid_md5;default:-" db:"adjust_gps_adid_md5"`
	AdjustIsRas           int32     `gorm:"column:adjust_is_ras" db:"adjust_is_ras"`
	AdjustInfo            *string   `gorm:"column:adjust_info" db:"adjust_info"`
	LastLoginTime         time.Time `gorm:"column:last_login_time;comment:上一次登录时间" db:"last_login_time"` // 上一次登录时间
	FbInstallReferrer     *string   `gorm:"column:fb_install_referrer" db:"fb_install_referrer"`
	ClientDeviceInfo      *string   `gorm:"column:client_device_info" db:"client_device_info"`
	IPInfo                *string   `gorm:"column:ip_info" db:"ip_info"`
	Password              string    `gorm:"column:password;comment:用户密码" db:"password"` // 用户密码
}
