package account

import (
	"strings"

	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/server"
)

type (
	ActorAccount struct {
		cactor.Base
	}
)

func (p *ActorAccount) AliasID() string {
	return "account"
}

// OnInit center为后端节点，不直接与客户端通信，所以了一些remote函数，供RPC调用
func (p *ActorAccount) OnInit() {
	p.Remote().Register("registerDevAccount", p.registerDevAccount)
	p.Remote().Register("getDevAccount", p.getDevAccount)
	p.Remote().Register("getUID", p.getUID)
}

// registerDevAccount 注册开发者帐号
func (p *ActorAccount) registerDevAccount(req *pb.DevRegister) int32 {
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
	return server.DevAccountRegister(accountName, password, req.Ip)
}

// getDevAccount 根据帐号名获取开发者帐号表
func (p *ActorAccount) getDevAccount(req *pb.DevRegister) (*pb.Int64, int32) {
	accountName := req.AccountName
	passWord := req.Password

	devAccount, _ := server.DevAccountWithName(accountName)
	if devAccount == nil || passWord != devAccount.Password {
		return nil, code.AccountAuthFail
	}

	return &pb.Int64{Value: int64(devAccount.UserID)}, code.OK
}

// getUID 获取uid
func (p *ActorAccount) getUID(req *pb.User) (*pb.Int64, int32) {
	accout, error := server.DevAccountWithName(req.DeviceName)
	if error != nil {
		return nil, code.AccountTokenValidateFail
	}
	userId, ok := server.BindUID(req.SdkId, req.Pid, req.OpenId, accout.UserID)
	if userId == 0 || !ok {
		return nil, code.AccountBindFail
	}

	return &pb.Int64{Value: int64(userId)}, code.OK
}
