/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-17 11:01:50
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-24 17:11:06
 * @FilePath: /examples/demo_cluster/nodes/game/db/center_database.go
 * @Description: 获取gorm 连接
 */
package db

import (
	"sync"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cherryGORM "github.com/cherry-game/examples/demo_cluster/internal/component/pg_gorm"
	"gorm.io/gorm"
)

var (
	once   sync.Once
	gameDB *gorm.DB
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
		gameDBIDConfig := app.Settings().GetConfig("db_id_list")
		gameDBID := gameDBIDConfig.GetString("game_db_id")
		// 、.GetString("game_db_id")
		if gameDBID == "" {
			clog.Panic("game_db_id not configured")
		}

		// 获取数据库连接
		gameDB = gormComponent.GetDb(gameDBID)
		if gameDB == nil {
			clog.Panicf("database [%s] not found", gameDBID)
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
	if gameDB == nil {
		clog.Panic("Database not initialized. Call InitDatabase first.")
	}
	return gameDB
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
