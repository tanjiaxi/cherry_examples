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

	// ------------------- 连接到数据库 -------------------
	// 替换为你自己的数据库连接字符串 (DSN)
	dsn := "host=localhost user=postgres password=123456 dbname=classic_slots port=5432 sslmode=disable TimeZone=Asia/Shanghai"
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
	g.GenerateModel("user_bind")

	// 或者，为数据库中的所有表生成模型
	// g.ApplyBasic(g.GenerateAllTable()...)

	// ------------------- 执行生成 -------------------
	g.Execute()
}
