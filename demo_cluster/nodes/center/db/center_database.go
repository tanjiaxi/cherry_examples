package db

import (
	"sync"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cherryGORM "github.com/cherry-game/examples/demo_cluster/internal/component/pg_gorm"
	"gorm.io/gorm"
)

var (
	once     sync.Once
	centerDB *gorm.DB
)

// InitDatabase 初始化数据库连接
func InitDatabase(app cfacade.IApplication) {
	once.Do(func() {
		// 获取gorm组件
		gormComponent := app.Find(cherryGORM.Name).(*cherryGORM.Component)
		if gormComponent == nil {
			clog.Panicf("[component = %s] not found.", cherryGORM.Name)
		}

		// 获取数据库配置ID
		centerDbIDConfig := app.Settings().GetConfig("db_id_list")
		centerDbID := centerDbIDConfig.GetString("center_db_id")
		// 、.GetString("center_db_id")
		if centerDbID == "" {
			clog.Panic("center_db_id not configured")
		}

		// 获取数据库连接
		centerDB = gormComponent.GetDb(centerDbID)
		if centerDB == nil {
			clog.Panicf("database [%s] not found", centerDbID)
		}

		// 自动迁移表结构
		// if err := autoMigrate(); err != nil {
		// 	clog.Panicf("database migration failed: %v", err)
		// }

		clog.Info("Database initialized successfully")
	})
}

// GetDB 获取数据库连接
func GetDB() *gorm.DB {
	if centerDB == nil {
		clog.Panic("Database not initialized. Call InitDatabase first.")
	}
	return centerDB
}

// autoMigrate 自动迁移表结构
// func autoMigrate() error {
// 	db := GetDB()

// 	// 迁移用户相关表
// 	if err := db.AutoMigrate(&DevAccountTable{}); err != nil {
// 		return err
// 	}

// 	if err := db.AutoMigrate(&UserBindTable{}); err != nil {
// 		return err
// 	}

// 	clog.Info("Database tables migrated successfully")
// 	return nil
// }
