package gate

import (
	cslice "github.com/cherry-game/cherry/extend/slice"
	cstring "github.com/cherry-game/cherry/extend/string"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	pmessage "github.com/cherry-game/cherry/net/parser/pomelo/message"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
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
// 1.(建立连接)客户端建立连接，服务端对应创建一个agent用于处理玩家消息,actorID == sid
// 2.(用户登录)客户端进行帐号登录验证，通过uid绑定当前sid
// 3.(角色登录)客户端通过'beforeLoginRoutes'中的协议完成角色登录
func onPomeloDataRoute(agent *pomelo.Agent, route *pmessage.Route, msg *pmessage.Message) {
	session := pomelo.BuildSession(agent, msg)

	// agent没有"用户登录",且请求不是第一条协议，则踢掉agent，断开连接
	if !session.IsBind() && msg.Route != firstRouteName {
		agent.Kick(notLoginRsp, true)
		return
	}
	//检测是不是相同的节点（相同的服务）
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

	// 1. 从session中获取玩家绑定的游戏服务器ID
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
	numberInfo, found := agent.Discovery().GetMember(nodeID)
	clog.Info("game node", numberInfo)
	return found
}

// 节点没有在线处理
func handleGameNodeOffline(agent *pomelo.Agent, session *cproto.Session) {
	// 1. 选择新的游戏节点
	newGameNode := selectGameNode(agent)
	if newGameNode == "" {
		// 没有可用的Game节点，踢掉玩家
		agent.Kick(&pb.Int32{Value: code.ServerMaintenance}, true)
		return
	}

	// 2. 更新session中的serverID
	session.Set(sessionKey.ServerID, newGameNode)
	clog.Infof("Player %d reassigned from offline server to: %s", session.Uid, newGameNode)

	// 3. 通知玩家服务器切换（可选）
	// agent.Response(session, &pb.ReconnectResponse{
	// 	NewServerID: newGameNode,
	// 	Reason:      "服务器维护，已自动切换",
	// })

	// 4. 可选：保存玩家状态到数据库
	// savePlayerStateToDatabase(session.Uid)
}

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
