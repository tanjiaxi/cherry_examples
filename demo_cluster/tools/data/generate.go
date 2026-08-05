/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-29 15:53:44
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-03 18:06:11
 * @FilePath: /examples/demo_cluster/tools/data/generate.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package main

import (
	"gorm.io/driver/postgres"
	"gorm.io/gen"
	"gorm.io/gorm"
)

func main() {
	// 指定生成代码的输出目录
	const outPath = "./dal/model"

	// ------------------- 配置代码生成器 -------------------
	g := gen.NewGenerator(gen.Config{
		OutPath:      outPath,                                                            // 生成代码的输出目录
		ModelPkgPath: "model",                                                            // 生成 model 的包名
		Mode:         gen.WithoutContext | gen.WithDefaultQuery | gen.WithQueryInterface, // 生成模式
	})
	// 🔴 显式配置：让所有生成的表名都在数据库底层自动补齐 schema 前缀
	g.WithTableNameStrategy(func(tableName string) string {
		// 假设这些表在物理上必须加上 "game_records."
		return "newsz_2024." + tableName
	})
	// ------------------- 连接到数据库 -------------------
	// 替换为你自己的数据库连接字符串 (DSN)
	dsn := "host=10.10.10.251 user=postgres password=postgres dbname=classic_slots port=5432 sslmode=disable TimeZone=Asia/Shanghai search_path=newsz_2024"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}

	// 将数据库连接设置到生成器中
	g.UseDB(db)

	// ------------------- 定义生成规则 -------------------
	// g.ApplyBasic 方法会为所有表生成基础的 CRUD 代码
	// 我们在这里指定要为哪些表生成模型

	// 为单个表生成模型
	// g.GenerateModel("n2_cfg_card")
	// g.GenerateModel("n2_cfg_reel_room")
	// g.GenerateModel("n2_cfg_roomlist")
	// g.GenerateModel("slots_device", gen.FieldType("adjust_info", "*string"),
	// 	gen.FieldType("fb_install_referrer", "*string"),
	// 	gen.FieldType("client_device_info", "*string"),
	// 	gen.FieldType("ip_info", "*string"),
	// )
	// g.GenerateModel("slots_user")
	// g.GenerateModel("user_bind")
	// g.GenerateModel("n2_cfg_level")
	// g.GenerateModel("lines3x3")
	// g.GenerateModel("lines3x4")
	// g.GenerateModel("lines3x5")
	// g.GenerateModel("lines3x6")
	// g.GenerateModel("lines4x3")
	// g.GenerateModel("lines4x5")
	// g.GenerateModel("lines4x6")
	// g.GenerateModel("lines5x5")
	// g.GenerateModel("lines_ids")
	g.GenerateModel("asset_ledger")
	g.GenerateModel("asset_operation")
	g.GenerateModel("domain_outbox")

	// 或者，为数据库中的所有表生成模型
	// g.ApplyBasic(g.GenerateAllTable()...)
	// -------------------执行生成 -------------------
	g.Execute()
}
