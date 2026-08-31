package gate

import (
	"context"
	"time"

	ccontext "github.com/cherry-game/cherry/extend/context"
	cstring "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	rpcCenter "github.com/cherry-game/examples/demo_cluster/internal/rpc/center"
	sessionKey "github.com/cherry-game/examples/demo_cluster/internal/session_key"
	"github.com/cherry-game/examples/demo_cluster/internal/token"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/server"
)

var (
	duplicateLoginCode []byte
)

type (
	// ActorAgent 每个网络连接对应一个ActorAgent
	ActorAgent struct {
		cactor.Base
	}
)

func (p *ActorAgent) OnInit() {
	duplicateLoginCode, _ = p.App().Serializer().Marshal(&cproto.I32{
		Value: code.PlayerDuplicateLogin,
	})
	// Local：处理相同节点的actor
	p.Local().Register("login", p.login)
	// Remote：处理其他Actor的RPC调用
	p.Remote().Register("setSession", p.setSession)
}

func (p *ActorAgent) setSession(ctx context.Context, req *pb.StringKeyValue) {
	if req.Key == "" {
		return
	}
	if agent, ok := pomelo.GetAgent(p.ActorID(), 0); ok {
		agent.Session().Set(req.Key, req.Value)
	}
}

// GetUID 获取帐号UID
func GetUID(app cfacade.IApplication, sdkId, pid int32, openId string) (cfacade.UID, int32) {
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
	clog.Debugf("getUID代码执行耗时: %s ,id: %s ,count: %d ", elapsed, openId)
	return int64(userId), code.OK
}

// login 用户登录，验证帐号 (*pb.LoginResponse, int32)
func (p *ActorAgent) login(ctx context.Context, session *cproto.Session, req *pb.LoginRequest) {
	done := metrics.TrackRequest("gate.actor.login")
	defer done(false)
	agent, found := pomelo.GetAgent(p.ActorID(), session.Uid)
	if !found {
		return
	}

	// 验证token
	userToken, errCode := p.validateToken(req.Token)
	if code.IsFail(errCode) {
		agent.Response(session, errCode)
		return
	}

	// 验证pid是否配置
	sdkRow := data.SdkConfig.Get(userToken.PID)
	if sdkRow == nil {
		agent.ResponseCode(session, code.PIDError, true)
		return
	}

	// 根据token带来的sdk参数，从中心节点获取userId
	// 3. 计算并打印执行时间
	// startTime := time.Now()
	traceId := ccontext.GetTraceId(ctx)
	userId, errCode := rpcCenter.GetUID(p.App(), sdkRow.SdkId, userToken.PID, userToken.OpenID, traceId)
	// elapsed := time.Since(startTime)
	// clog.Debugf("ReadySPin代码执行耗时: %s %s", elapsed, userToken.OpenID)
	if userId == 0 || code.IsFail(errCode) {
		agent.ResponseCode(session, code.AccountBindFail, true)
		return
	}
	// 先跨 Gate 挤号，再本机 Bind，减少旧连接 onClose 误删新 Location 的窗口
	p.checkGateSession(userId)

	oldAgent, err := pomelo.Bind(session.Sid, userId)
	if err != nil {
		agent.ResponseCode(session, code.AccountBindFail, true)
		clog.Warn(err)
		return
	}

	// 挤掉之前的agent
	if oldAgent != nil {
		// close=true，关掉旧连接
		oldAgent.Kick(duplicateLoginCode, true)
	}

	// 调用Center分配Game节点（负载均衡）
	gateNodeId := p.App().NodeID()
	allocResp, errCode := rpcCenter.AllocateNodes(p.App(), userId, gateNodeId, req.ServerId, traceId)
	if code.IsFail(errCode) || allocResp == nil || allocResp.GameNodeId == "" {
		clog.Warnf("[login] 分配节点失败: userId=%d, serverId=%d, errCode=%d",
			userId, req.ServerId, errCode)
		agent.ResponseCode(session, code.AllocateNodeFail, true)
		return
	}

	agent.Session().Set(sessionKey.AreaServerID, cstring.ToString(req.ServerId))
	agent.Session().Set(sessionKey.GameNodeID, allocResp.GameNodeId)
	agent.Session().Set(sessionKey.PID, cstring.ToString(userToken.PID))
	agent.Session().Set(sessionKey.OpenID, userToken.OpenID)

	clog.Infof("[login] 节点分配成功: userId=%d, serverId=%d, gameNode=%s",
		userId, req.ServerId, allocResp.GameNodeId)

	response := &pb.LoginResponse{
		UserId: userId,
		Pid:    userToken.PID,
		OpenId: userToken.OpenID,
	}

	// 直接使用 agent.Response，底层会自动打印日志
	agent.Response(session, response)
}

func (p *ActorAgent) validateToken(base64Token string) (*token.Token, int32) {
	userToken, ok := token.DecodeToken(base64Token)
	if !ok {
		return nil, code.AccountTokenValidateFail
	}

	platformRow := data.SdkConfig.Get(userToken.PID)
	if platformRow == nil {
		return nil, code.PIDError
	}

	statusCode, ok := token.Validate(userToken, platformRow.Salt)
	if !ok {
		return nil, statusCode
	}
	// 一次性票：Center 消费 jti（多 Gate 共享）
	if errCode := rpcCenter.ConsumeTokenJTI(p.App(), userToken.JTI, ""); code.IsFail(errCode) {
		return nil, errCode
	}
	return userToken, code.OK
}

func (p *ActorAgent) checkGateSession(uid cfacade.UID) {
	rsp := &cproto.PomeloKick{
		Uid:    uid,
		Reason: duplicateLoginCode,
		Close:  true,
	}

	// 遍历其他网关节点，挤掉旧的agent
	members := p.App().Discovery().ListByType(p.App().NodeType(), p.App().NodeID())
	for _, member := range members {
		// user是gate.go里自定义的agentActorID
		actorPath := cfacade.NewPath(member.GetNodeID(), "user")
		p.Call(actorPath, pomelo.KickFuncName, "", rsp)
	}
}

// onSessionClose  当agent断开时，关闭对应的ActorAgent
func (p *ActorAgent) onSessionClose(agent *pomelo.Agent) {
	session := agent.Session()
	userId := session.Uid
	sid := agent.SID()
	// 被挤下线：uid 已绑到新连接 → 不要 RemoveLocation（否则删掉新登录的位置）
	shouldRemoveLoc := userId > 0
	if shouldRemoveLoc {
		if cur, ok := pomelo.GetAgentWithUID(userId); ok && cur.SID() != sid {
			shouldRemoveLoc = false
			clog.Infof("[onSessionClose] 旧连接被挤下线，跳过 RemoveLocation uid=%d sid=%s", userId, sid)
		}
	}
	if shouldRemoveLoc {
		// 仅当 Location 仍指向本 Gate 时删除（多 Gate 挤号）
		errCode := rpcCenter.RemoveLocationIfGate(p.App(), userId, p.App().NodeID(), "")
		if code.IsFail(errCode) {
			clog.Infof("[onSessionClose] RemoveLocationIfGate fail uid=%d err=%d", userId, errCode)
		}
	}

	gameNodeId := session.GetString(sessionKey.GameNodeID)
	if gameNodeId != "" {
		childId := cstring.ToString(userId)
		p.Call(cfacade.NewChildPath(gameNodeId, "player", childId), "sessionClose", "", nil)
		p.Call(cfacade.NewChildPath(gameNodeId, "slots", childId), "sessionClose", "", nil)
	}
	p.Exit()
	clog.Infof("sessionClose path = %s", p.Path())
}
