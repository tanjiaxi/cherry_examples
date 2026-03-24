package gate

import (
	"time"

	cslice "github.com/cherry-game/cherry/extend/slice"
	cstring "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	pmessage "github.com/cherry-game/cherry/net/parser/pomelo/message"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	rpcCenter "github.com/cherry-game/examples/demo_cluster/internal/rpc/center"
	sessionKey "github.com/cherry-game/examples/demo_cluster/internal/session_key"
)

var (
	// 客户端连接后，必需先执行第一条协议，进行token验证后，才能进行后续的逻辑
	firstRouteName = "gate.user.login"

	// 角色进入游戏时的前三个协议
	beforeLoginRoutes = []string{
		"game.player.select", //查询玩家角色
		"game.player.create", //玩家创建角色
		"game.player.enter",  //玩家角色进入游戏
	}

	notLoginRsp = &pb.Int32{
		Value: code.PlayerDenyLogin,
	}
)

// onDataRoute 数据路由规则
//
// 登录逻辑:
// 1.(建立连接)客户端建立连接，服务端对应创建一个agent用于处理玩家消息,actorID == sid == childId
// 2.(用户登录)客户端进行帐号登录验证，通过uid绑定当前sid
// 3.(角色登录)客户端通过'beforeLoginRoutes'中的协议完成角色登录
func onPomeloDataRoute(agent *pomelo.Agent, route *pmessage.Route, msg *pmessage.Message) {
	// 使用 msg.StartTime 统计从收到消息到处理完成的时间
	// 如果 StartTime 为零值，使用当前时间
	startTime := msg.StartTime
	if startTime.IsZero() {
		startTime = time.Now()
	}

	// 记录请求开始
	routeName := msg.Route
	metrics.RecordRequest(routeName)

	// 使用 defer 确保统计被记录
	hasError := false
	defer func() {
		metrics.RecordResponse(routeName, startTime, hasError)
	}()

	session := pomelo.BuildSession(agent, msg)
	clog.Infof("[GATE-IN] route=%s, uid=%d, sid=%s, mid=%d, size=%d bytes",
		msg.Route,
		session.Uid,
		session.Sid,
		msg.ID,
		len(msg.Data))
	// agent没有"用户登录",且请求不是第一条协议，则踢掉agent，断开连接
	if !session.IsBind() && msg.Route != firstRouteName {
		hasError = true
		agent.Kick(notLoginRsp, true)
		return
	}
	//检测是不是相同的节点（相同的服务）比如都是gate
	if agent.NodeType() == route.NodeType() {
		targetPath := cfacade.NewChildPath(agent.NodeID(), route.HandleName(), session.Sid)
		pomelo.LocalDataRoute(agent, session, route, msg, targetPath)
	} else {
		gameNodeRoute(agent, session, route, msg)
	}
}

// gameNodeRoute 实现agent路由消息到游戏节点
func gameNodeRoute(agent *pomelo.Agent, session *cproto.Session, route *pmessage.Route, msg *pmessage.Message) {
	if !session.IsBind() {
		return
	}

	// 1. 从session中获取玩家绑定的游戏服务器ID,这里的ServerID，是game节点的，nodeId
	serverId := session.GetString(sessionKey.ServerID)
	if serverId == "" {
		// 没有可用的游戏服务器，踢掉玩家
		agent.Kick(&pb.Int32{Value: code.NoAvailableGameServer}, true)
		clog.Info("player is not bind server")
		return
	}

	// 2. 检查目标Game节点是否在线
	if !isGameNodeOnline(agent, serverId) {
		clog.Warnf("Player %d's bound server %s is offline, reassigning", session.Uid, serverId)
		handleGameNodeOffline(agent, session)
		return
	}

	// 3. 如果agent没有完成"角色登录",则禁止转发到game节点
	if !session.Contains(sessionKey.PlayerID) {
		// 如果不是角色登录协议则踢掉agent
		if found := cslice.StringInSlice(msg.Route, beforeLoginRoutes); !found {
			agent.Kick(notLoginRsp, true)
			return
		}
	}

	// 4. 转发消息到目标游戏节点
	childId := cstring.ToString(session.Uid)
	targetPath := cfacade.NewChildPath(serverId, route.HandleName(), childId)
	pomelo.ClusterLocalDataRoute(agent, session, route, msg, serverId, targetPath)
}

// 检测游戏节点是否在线
func isGameNodeOnline(agent *pomelo.Agent, nodeID string) bool {
	_, found := agent.Discovery().GetMember(nodeID)
	// clog.Info("game node", numberInfo)
	return found
}

// handleGameNodeOffline 节点没有在线处理，调用Center重新分配节点
func handleGameNodeOffline(agent *pomelo.Agent, session *cproto.Session) {
	userId := session.Uid
	gateNodeId := agent.NodeID()

	// 1. 调用Center重新分配Game节点（负载均衡）
	// agent 嵌入了 IApplication，可以直接作为 app 使用
	allocResp, errCode := rpcCenter.AllocateNodes(agent, userId, gateNodeId)
	if code.IsFail(errCode) || allocResp == nil {
		clog.Warnf("[handleGameNodeOffline] 重新分配节点失败: userId=%d, errCode=%d", userId, errCode)
		// 尝试本地选择
		newGameNode := selectGameNode(agent)
		if newGameNode == "" {
			agent.Kick(&pb.Int32{Value: code.ServerMaintenance}, true)
			return
		}
		session.Set(sessionKey.ServerID, newGameNode)
		session.Set(sessionKey.GameNodeID, newGameNode)
		clog.Infof("[handleGameNodeOffline] 本地选择新节点: userId=%d, gameNode=%s", userId, newGameNode)
		return
	}

	// 2. 更新session中的serverID
	session.Set(sessionKey.ServerID, allocResp.GameNodeId)
	session.Set(sessionKey.GameNodeID, allocResp.GameNodeId)
	clog.Infof("[handleGameNodeOffline] 重新分配成功: userId=%d, gameNode=%s", userId, allocResp.GameNodeId)
}

// selectGameNode 本地选择Game节点（作为后备方案）
func selectGameNode(agent *pomelo.Agent) string {
	members := agent.Discovery().ListByType("game", "")
	if len(members) == 0 {
		return ""
	}

	// 选择一个在线的游戏节点
	for _, member := range members {
		if isGameNodeOnline(agent, member.GetNodeID()) {
			return member.GetNodeID()
		}
	}

	return ""
}
