package db

import (
	"errors"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/model"
	"gorm.io/gorm"
)

// CreateAccount 创建账户 - 简单操作，直接调用
func CreateUserBind(userBind *model.UserBind) error {
	if err := GetDB().Create(userBind).Error; err != nil {
		clog.Errorf("Failed to create account: %v", err)
		return err
	}
	return nil
}

// GetUserBind 根据名称查询账户 - 简单操作，直接调用
func GetUserBind(pid int32, openId string) (*model.UserBind, error) {
	var userBind model.UserBind
	if err := GetDB().Where("pid = ?", pid).Where("open_id = ?", openId).First(&userBind).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("account not found")
		}
		return nil, err
	}
	return &userBind, nil
}
