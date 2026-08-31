/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-29 17:09:34
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-06-30 09:58:24
 * @FilePath: /examples/demo_cluster/nodes/center/module/account/actor_account.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package spin

import (
	"strings"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/server"
)

type (
	SyncSpinData struct {
		cactor.Base
		count int
	}
)

func (p *SyncSpinData) AliasID() string {
	return "syncSpinData"
}

// OnInit center为后端节点，不直接与客户端通信，所以了一些remote函数，供RPC调用
func (p *SyncSpinData) OnInit() {
	p.Remote().Register("saveSpinData", p.saveSpinData)
}

// 保存spin的数据
func (p *SyncSpinData) saveSpinData() {
}

// registerDevAccount 注册开发者帐号
func (p *SyncSpinData) registerDevAccount(req *pb.DevRegister) int32 {
	done := metrics.TrackRequest("center.account.registerDevAccount")
	defer done(false)
	accountName := req.AccountName
	password := req.Password

	if strings.TrimSpace(accountName) == "" || strings.TrimSpace(password) == "" {
		return code.RegisterError
	}

	if len(accountName) < 3 || len(accountName) > 18 {
		return code.RegisterError
	}

	if len(password) < 3 || len(password) > 18 {
		return code.RegisterError
	}
	return server.DevAccountRegister(accountName, password, req.Ip)
}

// getDevAccount 根据帐号名获取开发者帐号表
func (p *SyncSpinData) getDevAccount(req *pb.DevRegister) (*pb.String, int32) {
	accountName := req.AccountName
	passWord := req.Password

	devAccount, _ := server.DevAccountWithName(accountName)
	if devAccount == nil || passWord != devAccount.Password {
		clog.Warnf("[getDevAccount] AccountAuthFail accountName = %s, passWord = %s, findAccountName = %s, findPassWord = %s", accountName, passWord, devAccount.DeviceName, devAccount.Password)
		return nil, code.AccountAuthFail
	}

	return &pb.String{Value: devAccount.DeviceName}, code.OK
}

// getUserID 获取uid
func (p *SyncSpinData) getUserID(req *pb.User) (*pb.Int64, int32) {
	//req.OpenId 其实就是deviceName
	// 3. 计算并打印执行时间
	p.count++
	startTime := time.Now()
	accout, error := server.DevAccountWithName(req.OpenId)
	if error != nil {
		return nil, code.AccountTokenValidateFail
	}
	userId, ok := server.BindUID(req.SdkId, req.Pid, req.OpenId, accout.UserID)
	if userId == 0 || !ok {
		return nil, code.AccountBindFail
	}

	elapsed := time.Since(startTime)
	clog.Debugf("getUserID代码执行耗时: %s ,id: %s ,count: %d ", elapsed, req.OpenId, p.count)
	return &pb.Int64{Value: int64(userId)}, code.OK
}
