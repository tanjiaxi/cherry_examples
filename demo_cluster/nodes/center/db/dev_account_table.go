/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-29 16:29:22
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-01-14 21:17:34
 * @FilePath: /examples/demo_cluster/nodes/center/db/dev_account_table.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package db

import (
	"errors"
	"fmt"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AccountRepository 账户数据访问层
type AccountRepository struct {
}

func NewAccountRepository() *AccountRepository {
	return &AccountRepository{}
}

// CreateAccount 创建账户 - 简单操作，直接调用
func CreateAccount(account *model.SlotsDevice) (*model.SlotsDevice, error) {
	if err := GetDB().Create(account).Error; err != nil {
		clog.Errorf("Failed to create account: %v", err)
		return nil, err
	}
	return account, nil
}

// SelectAndCreateAccount 创建用户信息 (如果用户已存在则不插入，一次请求完成)
func SelectAndCreateAccount(account *model.SlotsDevice) (*model.SlotsDevice, error) {
	if err := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "device_name"}}, // 必须在数据库有唯一索引
		DoNothing: true,
	}).Create(account).Error; err != nil {
		clog.Errorf("Failed to create account: %v", err)
		return nil, err
	}
	return account, nil
}

// SelectAndCreateUserInfo 创建用户信息 (如果用户已存在则不插入，一次请求完成)
func SelectAndCreateUserInfo(userInfo *model.SlotsUser) (*model.SlotsUser, error) {
	if err := GetDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}}, // 必须在数据库有唯一索引
		DoNothing: true,
	}).Create(userInfo).Error; err != nil {
		clog.Errorf("Failed to create user: %v", err)
		return nil, err
	}
	return userInfo, nil
}

// CreateUserInfo 创建用户信息 (如果用户已存在则不插入，一次请求完成)
func CreateUserInfo(userInfo *model.SlotsUser) (*model.SlotsUser, error) {
	if err := GetDB().Create(userInfo).Error; err != nil {
		clog.Errorf("Failed to create user info: %v", err)
		return nil, err
	}
	return userInfo, nil
}

// GetAccountByName 根据名称查询账户 - 简单操作，直接调用
func GetAccountByName(accountName string) (*model.SlotsDevice, error) {
	var account model.SlotsDevice
	if err := GetDB().Where("device_name = ?", accountName).Take(&account).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("account not found")
		}
		return nil, err
	}
	return &account, nil
}

// UpdateAccountLastLogin 更新最后登录时间 - 简单操作，直接调用
func UpdateAccountLastLogin(accountName int64, loginTime int64, loginIP string) error {
	return GetDB().Model(&model.SlotsDevice{}).
		Where("device_name = ?", accountName).
		Updates(map[string]interface{}{
			"last_login_time": loginTime,
			"ip_info":         loginIP,
		}).Error
}

// GetAccountStats 获取账户统计信息 - 复杂查询，可考虑使用DB Actor
func GetAccountStats() (*AccountStats, error) {
	var stats AccountStats

	// 总账户数
	if err := GetDB().Model(&model.SlotsDevice{}).Count(&stats.TotalAccounts).Error; err != nil {
		return nil, err
	}

	// 今日注册数
	if err := GetDB().Model(&model.SlotsDevice{}).
		Where("DATE(FROM_UNIXTIME(create_time)) = CURDATE()").
		Count(&stats.TodayRegistered).Error; err != nil {
		return nil, err
	}

	return &stats, nil
}

// BatchCreateAccounts 批量创建账户 - 复杂操作，建议使用DB Actor
func BatchCreateAccounts(accounts []*model.SlotsDevice) error {
	return GetDB().Transaction(func(tx *gorm.DB) error {
		for _, account := range accounts {
			if err := tx.Create(account).Error; err != nil {
				return fmt.Errorf("failed to create account %s: %v", account.DeviceName, err)
			}
		}
		return nil
	})
}

type AccountStats struct {
	TotalAccounts   int64 `json:"totalAccounts"`
	TodayRegistered int64 `json:"todayRegistered"`
}

// CreateUserAndDevice 在事务中创建 user 和 device，确保原子性
func CreateUserAndDevice(accountName, ip, password string) (*model.SlotsUser, *model.SlotsDevice, error) {
	var userInfo *model.SlotsUser
	var deviceInfo *model.SlotsDevice

	err := GetDB().Transaction(func(tx *gorm.DB) error {
		// 1. 先检查 device 是否存在（在事务内加锁）
		var existing model.SlotsDevice
		if err := tx.Where("device_name = ?", accountName).First(&existing).Error; err == nil {
			// 已存在，返回现有记录
			deviceInfo = &existing
			// 查询对应的 user
			var user model.SlotsUser
			if err := tx.Where("user_id = ?", existing.UserID).First(&user).Error; err == nil {
				userInfo = &user
			}
			return nil // 不是错误，只是已存在
		}

		// 2. 创建 user
		userInfo = &model.SlotsUser{
			UserLevel:  1,
			CurExp:     "0",
			ExpPercent: 0,
			Money:      10000,
			Diamond:    10000,
			Birthday:   "1970-01-01",
		}
		if err := tx.Create(userInfo).Error; err != nil {
			return fmt.Errorf("create user failed: %w", err)
		}

		// 3. 创建 device
		deviceInfo = &model.SlotsDevice{
			UserID:     userInfo.UserID,
			DeviceName: accountName,
			ClientIP:   ip,
			Password:   password,
		}
		if err := tx.Create(deviceInfo).Error; err != nil {
			return fmt.Errorf("create device failed: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, nil, err
	}
	return userInfo, deviceInfo, nil
}
