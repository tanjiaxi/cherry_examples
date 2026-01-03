package location

import (
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	"github.com/cherry-game/examples/demo_cluster/nodes/center/server"
)

type (
	// ActorLocation 玩家位置管理Actor
	ActorLocation struct {
		cactor.Base
		locationMgr     *server.PlayerLocationManager
		nodeCounter     *server.NodeOnlineCounter
		healthChecker   *server.NodeHealthChecker
		stopHealthCheck func()
	}
)

func NewActorLocation() *ActorLocation {
	return &ActorLocation{}
}

func (p *ActorLocation) AliasID() string {
	return "location"
}

// OnInit 初始化
func (p *ActorLocation) OnInit() {
	// 初始化组件
	p.nodeCounter = server.NewNodeOnlineCounter()
	p.healthChecker = server.NewNodeHealthChecker(10) // 10秒超时
	p.locationMgr = server.NewPlayerLocationManager(p.nodeCounter, p.healthChecker)

	// 注册Remote方法
	p.Remote().Register("allocateNodes", p.allocateNodes)
	p.Remote().Register("getLocation", p.getLocation)
	p.Remote().Register("removeLocation", p.removeLocation)
	p.Remote().Register("heartbeat", p.heartbeat)
	p.Remote().Register("getBestGate", p.getBestGate)
	p.Remote().Register("getBestGateFromNodes", p.getBestGateFromNodes)
	p.Remote().Register("getBestGameFromNodes", p.getBestGameFromNodes)
	p.Remote().Register("getBestGame", p.getBestGame)
	p.Remote().Register("getNodeOnlineCount", p.getNodeOnlineCount)

	// 启动健康检查
	p.startHealthCheck()

	clog.Info("[ActorLocation] 初始化完成")
}

// OnStop 停止时清理
func (p *ActorLocation) OnStop() {
	if p.stopHealthCheck != nil {
		p.stopHealthCheck()
	}
}

// startHealthCheck 启动健康检查
func (p *ActorLocation) startHealthCheck() {
	p.stopHealthCheck = p.healthChecker.StartHealthCheck(
		5*time.Second, // 每5秒检查一次
		func(nodeId string) {
			// 发现不健康节点，触发迁移
			p.handleUnhealthyNode(nodeId)
		},
	)
}

// handleUnhealthyNode 处理不健康节点
func (p *ActorLocation) handleUnhealthyNode(nodeId string) {
	// 获取健康的Game节点
	healthyNodes := p.getHealthyGameNodes()
	if len(healthyNodes) == 0 {
		clog.Errorf("[ActorLocation] 没有可用的健康节点进行迁移")
		return
	}

	// 迁移玩家
	count, err := p.locationMgr.MigratePlayersFromNode(nodeId, healthyNodes)
	if err != nil {
		clog.Errorf("[ActorLocation] 迁移玩家失败: %v", err)
		return
	}

	clog.Infof("[ActorLocation] 从节点 %s 迁移了 %d 个玩家", nodeId, count)
}

// getHealthyGameNodes 获取健康的Game节点列表
func (p *ActorLocation) getHealthyGameNodes() []string {
	// 从Discovery获取所有Game节点
	gameNodes := p.App().Discovery().ListByType("game")
	var nodeIds []string
	for _, node := range gameNodes {
		nodeIds = append(nodeIds, node.GetNodeID())
	}

	// 过滤出健康的节点
	return p.healthChecker.FilterHealthyNodes(nodeIds)
}

// allocateNodes 为玩家分配节点
func (p *ActorLocation) allocateNodes(req *pb.AllocateNodesRequest) (*pb.AllocateNodesResponse, int32) {
	if req.UserId <= 0 || req.GateNodeId == "" {
		return nil, code.ParamError
	}

	// 获取可用的Game节点
	gameNodes := p.getHealthyGameNodes()
	if len(gameNodes) == 0 {
		// 如果没有健康节点，尝试使用所有Game节点
		allGameNodes := p.App().Discovery().ListByType("game")
		for _, node := range allGameNodes {
			gameNodes = append(gameNodes, node.GetNodeID())
		}
	}

	if len(gameNodes) == 0 {
		clog.Errorf("[ActorLocation] 没有可用的Game节点")
		return nil, code.NoAvailableGame
	}

	// 分配节点
	loc, err := p.locationMgr.AllocateNodes(req.UserId, req.GateNodeId, gameNodes)
	if err != nil {
		clog.Errorf("[ActorLocation] 分配节点失败: %v", err)
		return nil, code.AllocateNodeFail
	}

	return &pb.AllocateNodesResponse{
		UserId:     loc.UserId,
		GateNodeId: loc.GateNodeId,
		GameNodeId: loc.GameNodeId,
		LoginTime:  loc.LoginTime,
	}, code.OK
}

// getLocation 获取玩家位置
func (p *ActorLocation) getLocation(req *pb.Int64) (*pb.AllocateNodesResponse, int32) {
	loc, exists := p.locationMgr.GetLocation(req.Value)
	if !exists {
		return nil, code.PlayerLocationNotFound
	}

	return &pb.AllocateNodesResponse{
		UserId:     loc.UserId,
		GateNodeId: loc.GateNodeId,
		GameNodeId: loc.GameNodeId,
		LoginTime:  loc.LoginTime,
	}, code.OK
}

// removeLocation 移除玩家位置
func (p *ActorLocation) removeLocation(req *pb.Int64) int32 {
	clog.Infof("[removeLocation] 队列中消息: RemoteCount=%d, LocalCount=%d", int(p.Remote().Count()), int(p.Local().Count()))
	err := p.locationMgr.RemoveLocation(req.Value)
	if err != nil {
		return code.RemoveLocationFail
	}
	return code.OK
}

// heartbeat 节点心跳
func (p *ActorLocation) heartbeat(req *pb.HeartbeatRequest) int32 {
	if req.NodeId == "" {
		return code.ParamError
	}

	p.healthChecker.UpdateHeartbeat(req.NodeId)
	return code.OK
}

// getBestGate 获取最优Gate节点
func (p *ActorLocation) getBestGate(_ *pb.None) (*pb.String, int32) {
	// 获取所有Gate节点
	gateNodes := p.App().Discovery().ListByType("gate")
	if len(gateNodes) == 0 {
		return nil, code.NoAvailableGate
	}

	var nodeIds []string
	for _, node := range gateNodes {
		nodeIds = append(nodeIds, node.GetNodeID())
	}

	// 选择在线人数最少的
	bestNode := p.nodeCounter.GetLeastLoadedNode("gate", nodeIds)
	if bestNode == "" {
		// 如果没有统计数据，返回第一个
		bestNode = nodeIds[0]
	}

	// 获取节点地址
	for _, node := range gateNodes {
		if node.GetNodeID() == bestNode {
			// 返回节点的TCP地址
			settings := node.GetSettings()
			if tcpAddr, ok := settings["tcp_address"]; ok && tcpAddr != "" {
				return &pb.String{Value: tcpAddr}, code.OK
			}
			return &pb.String{Value: node.GetAddress()}, code.OK
		}
	}

	return nil, code.NoAvailableGate
}

// getBestGame 获取最优Game节点
func (p *ActorLocation) getBestGame(_ *pb.None) (*pb.String, int32) {
	gameNodes := p.getHealthyGameNodes()
	if len(gameNodes) == 0 {
		return nil, code.NoAvailableGame
	}

	bestNode := p.nodeCounter.GetLeastLoadedNode("game", gameNodes)
	if bestNode == "" {
		bestNode = gameNodes[0]
	}

	return &pb.String{Value: bestNode}, code.OK
}

// getBestGateFromNodes 从指定的Gate节点列表中获取最优节点
func (p *ActorLocation) getBestGateFromNodes(req *pb.StringList) (*pb.String, int32) {
	if req == nil || len(req.Values) == 0 {
		return nil, code.ParamError
	}

	// 获取所有Gate节点
	gateNodes := p.App().Discovery().ListByType("gate")
	if len(gateNodes) == 0 {
		return nil, code.NoAvailableGate
	}

	// 过滤出请求中指定的节点
	var validNodeIds []string
	nodeAddrMap := make(map[string]string) // nodeId -> address
	for _, node := range gateNodes {
		nodeId := node.GetNodeID()
		for _, reqNodeId := range req.Values {
			if nodeId == reqNodeId {
				validNodeIds = append(validNodeIds, nodeId)
				// 获取TCP地址
				settings := node.GetSettings()
				if tcpAddr, ok := settings["tcp_address"]; ok && tcpAddr != "" {
					nodeAddrMap[nodeId] = tcpAddr
				} else {
					nodeAddrMap[nodeId] = node.GetAddress()
				}
				break
			}
		}
	}

	if len(validNodeIds) == 0 {
		return nil, code.NoAvailableGate
	}

	// 选择在线人数最少的
	bestNode := p.nodeCounter.GetLeastLoadedNode("gate", validNodeIds)
	if bestNode == "" {
		bestNode = validNodeIds[0]
	}

	// 返回节点地址
	if addr, ok := nodeAddrMap[bestNode]; ok {
		return &pb.String{Value: addr}, code.OK
	}

	return nil, code.NoAvailableGate
}

// getBestGameFromNodes 从指定的Game节点列表中获取最优节点
func (p *ActorLocation) getBestGameFromNodes(req *pb.StringList) (*pb.String, int32) {
	if req == nil || len(req.Values) == 0 {
		return nil, code.ParamError
	}

	// 获取所有Game节点
	gameNodes := p.App().Discovery().ListByType("game")
	if len(gameNodes) == 0 {
		return nil, code.NoAvailableGame
	}

	// 过滤出请求中指定的节点，并检查健康状态
	var validNodeIds []string
	for _, node := range gameNodes {
		nodeId := node.GetNodeID()
		for _, reqNodeId := range req.Values {
			if nodeId == reqNodeId {
				// 检查节点是否健康
				if p.healthChecker.IsHealthy(nodeId) {
					validNodeIds = append(validNodeIds, nodeId)
				}
				break
			}
		}
	}

	if len(validNodeIds) == 0 {
		return nil, code.NoAvailableGame
	}

	// 选择在线人数最少的
	bestNode := p.nodeCounter.GetLeastLoadedNode("game", validNodeIds)
	if bestNode == "" {
		bestNode = validNodeIds[0]
	}

	return &pb.String{Value: bestNode}, code.OK
}

// getNodeOnlineCount 获取节点在线人数
func (p *ActorLocation) getNodeOnlineCount(req *pb.String) (*pb.Int32, int32) {
	count := p.nodeCounter.GetCount(req.Value)
	return &pb.Int32{Value: count}, code.OK
}

// GetLocationManager 获取位置管理器（供其他组件使用）
func (p *ActorLocation) GetLocationManager() *server.PlayerLocationManager {
	return p.locationMgr
}

// GetNodeCounter 获取节点计数器（供其他组件使用）
func (p *ActorLocation) GetNodeCounter() *server.NodeOnlineCounter {
	return p.nodeCounter
}

// GetHealthChecker 获取健康检查器（供其他组件使用）
func (p *ActorLocation) GetHealthChecker() *server.NodeHealthChecker {
	return p.healthChecker
}

// GetApp 获取应用实例
func (p *ActorLocation) GetApp() cfacade.IApplication {
	return p.App()
}
