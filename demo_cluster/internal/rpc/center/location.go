package rpcCenter

import (
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
)

const (
	locationActor = ".location"
)

const (
	allocateNodes        = "allocateNodes"
	getLocation          = "getLocation"
	removeLocation       = "removeLocation"
	heartbeat            = "heartbeat"
	getBestGate          = "getBestGate"
	getBestGateFromNodes = "getBestGateFromNodes"
	getBestGameFromNodes = "getBestGameFromNodes"
	getBestGame          = "getBestGame"
	getNodeOnlineCount   = "getNodeOnlineCount"
)

// AllocateNodes 为玩家分配Gate和Game节点
func AllocateNodes(app cfacade.IApplication, playerId int64, gateNodeId, traceId string) (*pb.AllocateNodesResponse, int32) {
	req := &pb.AllocateNodesRequest{
		UserId:     playerId,
		GateNodeId: gateNodeId,
	}

	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.AllocateNodesResponse{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, allocateNodes, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[AllocateNodes] playerId = %d, errCode = %v", playerId, errCode)
		return nil, errCode
	}

	return rsp, code.OK
}

// GetLocation 获取玩家位置
func GetLocation(app cfacade.IApplication, playerId int64, traceId string) (*pb.AllocateNodesResponse, int32) {
	req := &pb.Int64{Value: playerId}

	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.AllocateNodesResponse{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getLocation, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Debugf("[GetLocation] playerId = %d, errCode = %v", playerId, errCode)
		return nil, errCode
	}

	return rsp, code.OK
}

// RemoveLocation 移除玩家位置
func RemoveLocation(app cfacade.IApplication, playerId int64, traceId string) int32 {
	req := &pb.Int64{Value: playerId}

	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.Int32{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, removeLocation, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[RemoveLocation] playerId = %d, errCode = %v", playerId, errCode)
		return errCode
	}

	return rsp.Value
}

// Heartbeat 节点心跳
func Heartbeat(app cfacade.IApplication, nodeId, nodeType, traceId string) int32 {
	req := &pb.HeartbeatRequest{
		NodeId:   nodeId,
		NodeType: nodeType,
	}

	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.Int32{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, heartbeat, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[Heartbeat] nodeId = %s, errCode = %v", nodeId, errCode)
		return errCode
	}

	return rsp.Value
}

// GetBestGate 获取最优Gate节点
func GetBestGate(app cfacade.IApplication, traceId string) (string, int32) {
	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.String{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getBestGate, traceId, &pb.None{}, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetBestGate] errCode = %v", errCode)
		return "", errCode
	}

	return rsp.Value, code.OK
}

// GetBestGateFromNodes 从指定的Gate节点列表中获取最优节点
func GetBestGateFromNodes(app cfacade.IApplication, nodeIds []string, traceId string) (string, int32) {
	req := &pb.StringList{Values: nodeIds}

	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.String{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getBestGateFromNodes, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetBestGateFromNodes] errCode = %v", errCode)
		return "", errCode
	}

	return rsp.Value, code.OK
}

// GetBestGameFromNodes 从指定的Game节点列表中获取最优节点
func GetBestGameFromNodes(app cfacade.IApplication, nodeIds []string, traceId string) (string, int32) {
	req := &pb.StringList{Values: nodeIds}

	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.String{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getBestGameFromNodes, traceId, req, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetBestGameFromNodes] errCode = %v", errCode)
		return "", errCode
	}

	return rsp.Value, code.OK
}

// GetBestGame 获取最优Game节点
func GetBestGame(app cfacade.IApplication, traceId string) (string, int32) {
	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.String{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getBestGame, traceId, &pb.None{}, rsp)
	if code.IsFail(errCode) {
		clog.Warnf("[GetBestGame] errCode = %v", errCode)
		return "", errCode
	}

	return rsp.Value, code.OK
}

// GetNodeOnlineCount 获取节点在线人数
func GetNodeOnlineCount(app cfacade.IApplication, nodeId, traceId string) (int32, int32) {
	req := &pb.String{Value: nodeId}

	targetPath := GetTargetPath(app, locationActor)
	rsp := &pb.Int32{}
	errCode := app.ActorSystem().CallWait(sourcePath, targetPath, getNodeOnlineCount, traceId, req, rsp)
	if code.IsFail(errCode) {
		return 0, errCode
	}

	return rsp.Value, code.OK
}
