// account_handlers.go - 并发处理的账号服务 handlers
// 这些函数可以被 ConcurrentCluster 直接调用，绕过 Actor mailbox
package server

import (
	"strings"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

// RegisterDevAccountHandler 注册开发者帐号 - 并发版本
func RegisterDevAccountHandler(req *pb.DevRegister) int32 {
	accountName := req.AccountName
	password := req.Password

	if strings.TrimSpace(accountName) == "" || strings.TrimSpace(password) == "" {
		return code.LoginError
	}

	if len(accountName) < 3 || len(accountName) > 18 {
		return code.LoginError
	}

	if len(password) < 3 || len(password) > 18 {
		return code.LoginError
	}
	return DevAccountRegister(accountName, password, req.Ip)
}

// GetDevAccountHandler 根据帐号名获取开发者帐号表 - 并发版本
func GetDevAccountHandler(req *pb.DevRegister) (*pb.String, int32) {
	accountName := req.AccountName
	passWord := req.Password

	devAccount, _ := DevAccountWithName(accountName)
	if devAccount == nil || passWord != devAccount.Password {
		return nil, code.AccountAuthFail
	}

	return &pb.String{Value: devAccount.DeviceName}, code.OK
}

// GetUIDHandler 获取uid - 并发版本
func GetUIDHandler(req *pb.User) (*pb.Int64, int32) {
	startTime := time.Now()

	account, err := DevAccountWithName(req.OpenId)
	if err != nil {
		return nil, code.AccountTokenValidateFail
	}

	userId, ok := BindUID(req.SdkId, req.Pid, req.OpenId, account.UserID)
	if userId == 0 || !ok {
		return nil, code.AccountBindFail
	}

	elapsed := time.Since(startTime)
	clog.Debugf("[ConcurrentHandler] getUID执行耗时: %s, id: %s", elapsed, req.OpenId)

	return &pb.Int64{Value: int64(userId)}, code.OK
}
