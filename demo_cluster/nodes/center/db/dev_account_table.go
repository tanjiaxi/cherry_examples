/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-29 16:29:22
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-28 10:48:39
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

// CreateUserInfo 创建用户信息
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
	if err := GetDB().Where("device_name = ?", accountName).First(&account).Error; err != nil {
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
