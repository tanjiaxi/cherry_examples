package rpcCenter

import (
	"fmt"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/server"
)

// route = 节点类型.节点handler.remote函数

const (
	centerType = "center"
)

const (
	opsActor     = ".ops"
	accountActor = ".account"
)

const (
	ping               = "ping"
	registerDevAccount = "registerDevAccount"
	getDevAccount      = "getDevAccount"
	getUID = "getUID"
)

const (
	sourcePath = ".system"
)

// Ping 访问center节点，确认center已启动
func Ping(app cfacade.IApplication) bool {
	nodeID := GetCenterNodeID(app)
	if nodeID == "" {
		return false
	}

	rsp := &pb.Bool{}
	targetPath := nodeID + opsActor
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, ping, "", nil, rsp)
	if code.IsFail(errCode) {
		return false
	}

	return rsp.Value
}

// RegisterDevAccount 注册帐号
func RegisterDevAccount(app cfacade.IApplication, accountName, password, ip string) int32 {
	req := &pb.DevRegister{
		AccountName: accountName,
		Password:    password,
		Ip:          ip,
	}
	// 构建Actor路径：center节点的account Actor
	targetPath := GetTargetPath(app, accountActor)
	rsp := &pb.Int32{}
	// 通过Actor系统发送消息到Center节点的Actor
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, registerDevAccount, "", req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[RegisterDevAccount] accountName = %s, errCode = %v", accountName, errCode)
		return errCode
	}

	return rsp.Value
}

// GetDevAccount 获取帐号信息
func GetDevAccount(app cfacade.IApplication, accountName, password string) string {
	req := &pb.DevRegister{
		AccountName: accountName,
		Password:    password,
	}
	startTime := time.Now()
	targetPath := GetTargetPath(app, accountActor)
	rsp := &pb.String{}
	traceId := fmt.Sprint(accountName, password)
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getDevAccount, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetDevAccount] accountName = %s, errCode = %v", accountName, errCode)
		return ""
	}
	clog.Infof("getDevAccount代码执行耗时: %s, traceId: %s, result: %s", time.Since(startTime), traceId, rsp.Value)

	return rsp.Value
}

// GetUID 获取帐号UID
func GetUID1(app cfacade.IApplication, sdkId, pid int32, openId string) (cfacade.UID, int32) {
	startTime := time.Now()
	accout, err := server.DevAccountWithName(openId)
	if err != nil {
		return 0, code.AccountTokenValidateFail
	}
	userId, ok := server.BindUID(sdkId, pid, openId, accout.UserID)
	if userId == 0 || !ok {
		return 0, code.AccountBindFail
	}

	elapsed := time.Since(startTime)
	clog.Debugf("getUserID代码执行耗时: %s ,id: %s ,count: %d ", elapsed, openId)
	return int64(userId), code.OK
}

// GetUID 获取帐号UID
func GetUID(app cfacade.IApplication, sdkId, pid int32, openId, traceId string) (cfacade.UID, int32) {
	req := &pb.User{
		SdkId:  sdkId,
		Pid:    pid,
		OpenId: openId,
	}

	targetPath := GetTargetPath(app, accountActor)
	rsp := &pb.Int64{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getUID, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetUID] errCode = %v", errCode)
		return 0, errCode
	}

	return rsp.Value, code.OK
}

func GetCenterNodeID(app cfacade.IApplication) string {
	list := app.Discovery().ListByType(centerType)
	if len(list) > 0 {
		return list[0].GetNodeID()
	}
	return ""
}

func GetTargetPath(app cfacade.IApplication, actorID string) string {
	nodeID := GetCenterNodeID(app)
	return nodeID + actorID
}

const consumeTokenJti = "consumeTokenJti"

// ConsumeTokenJTI 消费一次性登录票（多 Gate 共享）
func ConsumeTokenJTI(app cfacade.IApplication, jti, traceId string) int32 {
	if jti == "" {
		return code.AccountTokenValidateFail
	}
	req := &pb.String{Value: jti}
	rsp := &pb.Int32{}
	targetPath := GetTargetPath(app, accountActor)
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, consumeTokenJti, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[ConsumeTokenJTI] jti=%s errCode=%v", jti, errCode)
		return errCode
	}
	if rsp.Value != 0 {
		return rsp.Value
	}
	return code.OK
}
