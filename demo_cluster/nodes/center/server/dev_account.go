/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-29 16:42:25
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-01-16 18:55:32
 * @FilePath: /examples/demo_cluster/nodes/center/server/dev_account.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package server

import (
	cherryError "github.com/cherry-game/cherry/error"
	cherryTime "github.com/cherry-game/cherry/extend/time"
	cherryLogger "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/db"
)

// SlotsDevice 开发模式的帐号信息表

func DevAccountRegister(accountName, password, ip string) int32 {
	var userInfo *model.SlotsUser
	_, err := db.GetAccountByName(accountName)
	// 关键：必须检查错误
	if err == nil {
		return code.AccountNameIsExist
	}
	userInfo, err = createUser()
	if err != nil {
		cherryLogger.Warnf("create account failed, err: %v", err)
		return code.AccountRegisterError
	}
	_, err = createDevice(userInfo.UserID, accountName, ip, password)
	if err != nil {
		cherryLogger.Warnf("create account failed, err: %v", err)
		return code.AccountRegisterError
	}
	return code.OK
}
func DevAccountRegister1(accountName, password, ip string) int32 {
	// 先检查 device 是否已存在
	existingDevice, _ := db.GetAccountByName(accountName)
	if existingDevice != nil {
		// 账号已存在，直接返回成功（幂等）
		return code.OK
	}

	// 使用事务确保 user 和 device 一起创建
	userInfo, deviceInfo, err := db.CreateUserAndDevice(accountName, ip, password)
	if err != nil {
		cherryLogger.Warnf("create account failed, err: %v", err)
		return code.AccountRegisterError
	}
	_ = userInfo
	_ = deviceInfo
	return code.OK
}
func DevAccountWithName(accountName string) (*model.SlotsDevice, error) {
	val, err := db.GetAccountByName(accountName)
	if err != nil {
		return nil, cherryError.Error("account not found")
	}

	return val, nil
}

// loadDevAccount 节点启动时预加载帐号数据
func loadDevAccount() {
	// 演示用，直接手工构建几个帐号
	// for i := 1; i <= 10; i++ {
	// 	index := cherryString.ToString(i)

	// 	devAccount := &DevAccountTable{
	// 		AccountID:   guid.Next(),
	// 		AccountName: "test" + index,
	// 		Password:    "test" + index,
	// 		CreateIP:    "127.0.0.1",
	// 		CreateTime:  cherryTime.Now().ToMillisecond(),
	// 	}

	// 	devAccountCache.Put(devAccount.AccountName, devAccount)
	// }

	cherryLogger.Info("preload DevAccountTable")
}
func createUser() (*model.SlotsUser, error) {
	var userInfo = &model.SlotsUser{
		UserLevel:     1,
		CurExp:        "0",
		ExpPercent:    0,
		Money:         100000000,
		Diamond:       10000,
		LogoutTime:    cherryTime.Now().Time,
		CreateTime:    cherryTime.Now().Time,
		LoginTime:     cherryTime.Now().Time,
		LastLoginTime: cherryTime.Now().Time,
		LoginStamp:    cherryTime.Now().Time,
		Birthday:      "1970-01-01",
	}
	userInfo, err := db.SelectAndCreateUserInfo(userInfo)
	if err != nil {
		cherryLogger.Panic("create account failed, err: %v", err)
		return nil, err
	}
	return userInfo, nil
}
func createDevice(userId int32, accountName string, ip string, password string) (*model.SlotsDevice, error) {
	var account = &model.SlotsDevice{
		UserID:            userId,
		DeviceName:        accountName,
		ClientIP:          ip,
		CreateTime:        cherryTime.Now().Time,
		Password:          password,
		AdjustInfo:        nil,
		FbInstallReferrer: nil,
		ClientDeviceInfo:  nil,
		IPInfo:            nil,
	}
	account, err := db.SelectAndCreateAccount(account)
	if err != nil {
		cherryLogger.Panic("create account failed, err: %v", err)
		return nil, err
	}
	return account, nil
}
