package robotclient

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	cherryError "github.com/cherry-game/cherry/error"
	cherryHttp "github.com/cherry-game/cherry/extend/http"
	cherryTime "github.com/cherry-game/cherry/extend/time"
	cherryLogger "github.com/cherry-game/cherry/logger"
	cherryClient "github.com/cherry-game/cherry/net/parser/pomelo/client"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	jsoniter "github.com/json-iterator/go"
)

// ServerListResponse 区服列表响应
type ServerListResponse struct {
	Areas          []*AreaInfo   `json:"areas"`
	Servers        []*ServerInfo `json:"servers"`
	UseLoadBalance bool          `json:"useLoadBalance"`
}

// AreaInfo 区信息
type AreaInfo struct {
	AreaId   int32  `json:"areaId"`
	AreaName string `json:"areaName"`
	Gate     string `json:"gate"` // WebSocket 地址
}

// ServerInfo 服信息
type ServerInfo struct {
	ServerId   int32  `json:"serverId"`
	ServerName string `json:"serverName"`
	AreaId     int32  `json:"areaId"`
	Status     int32  `json:"status"`
}

type (
	// Robot client robot
	Robot struct {
		*cherryClient.Client
		PrintLog   bool
		Token      string
		ServerId   int32
		PID        int32
		UID        int64
		OpenId     string
		UserId     int64
		PlayerName string
		StartTime  cherryTime.CherryTime
		// 新增：区服信息
		AreaId   int32
		GateAddr string // 当前连接的Gate地址
	}
)

func New(client *cherryClient.Client) *Robot {
	return &Robot{
		Client:   client,
		PrintLog: true,
	}
}

// GetServerList 获取区服列表
// GET /server/list/:pid
func (p *Robot) GetServerList(url string, pid string) (*ServerListResponse, error) {
	requestURL := fmt.Sprintf("%s/server/list/%s", url, pid)
	jsonBytes, _, err := cherryHttp.GlobalClientGet(requestURL, nil)
	if err != nil {
		return nil, cherryError.Errorf("get server list fail: %v", err)
	}

	// 解析响应
	rsp := &code.Result{}
	if err = jsoniter.Unmarshal(jsonBytes, rsp); err != nil {
		return nil, cherryError.Errorf("unmarshal server list fail: %v", err)
	}

	if code.IsFail(rsp.Code) {
		return nil, cherryError.Errorf("get server list fail: %s", rsp.Message)
	}

	// 解析 data 字段
	dataBytes, err := jsoniter.Marshal(rsp.Data)
	if err != nil {
		return nil, cherryError.Errorf("marshal data fail: %v", err)
	}

	serverList := &ServerListResponse{}
	if err = jsoniter.Unmarshal(dataBytes, serverList); err != nil {
		return nil, cherryError.Errorf("unmarshal server list data fail: %v", err)
	}

	p.Debugf("[%s] [GetServerList] areas=%d, servers=%d", p.TagName, len(serverList.Areas), len(serverList.Servers))
	return serverList, nil
}

// SelectAreaAndServer 选择区和服，返回Gate地址和ServerId
func (p *Robot) SelectAreaAndServer(serverList *ServerListResponse, areaId int32, serverId int32) (gateAddr string, selectedServerId int32, err error) {
	if serverList == nil || len(serverList.Areas) == 0 {
		return "", 0, cherryError.Error("server list is empty")
	}

	// 查找指定区
	var targetArea *AreaInfo
	for _, area := range serverList.Areas {
		if area.AreaId == areaId {
			targetArea = area
			break
		}
	}
	if targetArea == nil {
		// 默认选第一个区
		targetArea = serverList.Areas[0]
	}

	// 查找指定服
	var targetServer *ServerInfo
	for _, server := range serverList.Servers {
		if server.AreaId == targetArea.AreaId {
			if serverId == 0 || server.ServerId == serverId {
				targetServer = server
				break
			}
		}
	}
	if targetServer == nil {
		return "", 0, cherryError.Errorf("no server found for area %d", targetArea.AreaId)
	}

	p.AreaId = targetArea.AreaId
	p.GateAddr = targetArea.Gate
	p.ServerId = targetServer.ServerId

	p.Debugf("[%s] [SelectAreaAndServer] area=%d, server=%d, gate=%s",
		p.TagName, targetArea.AreaId, targetServer.ServerId, targetArea.Gate)

	return targetArea.Gate, targetServer.ServerId, nil
}

// ConnectToWebSocket 连接到WebSocket地址
// addr 格式: "127.0.0.1:20010" 或 "ws://127.0.0.1:20010"
func (p *Robot) ConnectToWebSocket(addr string) error {
	// 解析地址，提取 host 和 path
	host := addr
	path := "/"

	// 移除 ws:// 或 wss:// 前缀
	if strings.HasPrefix(addr, "ws://") {
		host = strings.TrimPrefix(addr, "ws://")
	} else if strings.HasPrefix(addr, "wss://") {
		host = strings.TrimPrefix(addr, "wss://")
	}

	// 检查是否包含路径
	if idx := strings.Index(host, "/"); idx > 0 {
		path = host[idx:]
		host = host[:idx]
	}

	p.Debugf("[%s] [ConnectToWebSocket] connecting to ws://%s%s", p.TagName, host, path)

	// start := time.Now()
	err := p.Client.ConnectToWS(host, path)
	// elapsed := time.Since(start)
	// if elapsed > 50*time.Millisecond {
	// 	cherryLogger.Warnf("[%s] ConnectToWS slow: %v (host=%s)", p.TagName, elapsed, host)
	// }
	if err != nil {
		return cherryError.Errorf("connect to websocket fail: %v", err)
	}

	p.GateAddr = addr
	p.Debugf("[%s] [ConnectToWebSocket] connected to %s", p.TagName, addr)
	return nil
}

// GetToken  http登录获取token对象
func (p *Robot) GetToken(url string, pid, userName, password string) error {
	requestURL := fmt.Sprintf("%s/login", url)
	traceId := fmt.Sprintf("%s%s", userName, password)
	jsonBytes, _, err, traceInfo := cherryHttp.GlobalClientGetWithTrace(requestURL, map[string]string{
		// jsonBytes, _, err := cherryHttp.GET(requestURL, map[string]string{
		"pid":      pid,
		"account":  userName,
		"password": password,
	})
	if err != nil {
		// 只在错误时打印详细trace
		if traceInfo != nil {
			cherryLogger.Errorf("[%s] GetToken FAILED - 详细追踪:", p.TagName)
			traceInfo.Print(traceId)
		}
		return err
	}

	rsp := code.Result{}
	if err = jsoniter.Unmarshal(jsonBytes, &rsp); err != nil {
		return err
	}

	if code.IsFail(rsp.Code) {
		return cherryError.Errorf("get Token fail. [message = %s]", rsp.Message)
	}

	p.Token = rsp.Data.(string)
	p.TagName = fmt.Sprintf("%s_%s", pid, userName)
	p.StartTime = cherryTime.Now()
	// 成功的情况下，可选：只在超过阈值时打印
	if traceInfo != nil && !traceInfo.GetConnStart.IsZero() && !traceInfo.FirstByte.IsZero() {
		totalTime := traceInfo.FirstByte.Sub(traceInfo.GetConnStart)
		if totalTime > 1000*time.Millisecond { // 超过1秒才打印
			cherryLogger.Warnf("[%s] GetToken SLOW (%v) - 详细追踪:", p.TagName, totalTime)
			traceInfo.Print(traceId)
		}
	}

	return nil
}

// UserLogin 用户登录对某游戏服
func (p *Robot) UserLogin(serverId int32) error {
	route := "gate.user.login"

	p.Debugf("[%s] [UserLogin] request ServerID = %d", p.TagName, serverId)

	msg, err := p.Request(route, &pb.LoginRequest{
		ServerId: serverId,
		Token:    p.Token,
		Params:   nil,
	})
	if err != nil {
		return err
	}

	p.ServerId = serverId

	rsp := &pb.LoginResponse{}
	err = p.Serializer().Unmarshal(msg.Data, rsp)
	if err != nil {
		return err
	}

	p.UID = rsp.UserId
	p.PID = rsp.Pid
	p.OpenId = rsp.OpenId

	p.Debugf("[%s] [UserLogin] response = %+v", p.TagName, rsp)
	return nil
}

// PlayerSelect 查看玩家列表
func (p *Robot) PlayerSelect() error {
	route := "game.player.select"

	msg, err := p.Request(route, &pb.None{})
	if err != nil {
		return err
	}

	rsp := &pb.PlayerSelectResponse{}
	err = p.Serializer().Unmarshal(msg.Data, rsp)
	if err != nil {
		return err
	}

	if len(rsp.List) < 1 {
		p.Debugf("[%s] not found player list.", p.TagName)
		return nil
	}

	p.UserId = rsp.List[0].UserId
	p.PlayerName = rsp.List[0].PlayerName

	p.Debugf("[%s] [PlayerSelect] response PlayerID = %d,PlayerName = %s", p.TagName, p.UserId, p.PlayerName)

	return nil
}

// ActorCreate 创建角色
func (p *Robot) ActorCreate() error {
	if p.UserId > 0 {
		p.Debugf("[%s] deny create actor", p.TagName)
		return nil
	}

	route := "game.player.create"
	gender := rand.Int31n(2)

	req := &pb.PlayerCreateRequest{
		PlayerName: "p" + p.OpenId,
		Gender:     gender,
	}

	msg, err := p.Request(route, req)
	if err != nil {
		return err
	}

	rsp := &pb.PlayerCreateResponse{}
	err = p.Serializer().Unmarshal(msg.Data, rsp)
	if err != nil {
		return err
	}

	p.UserId = rsp.Player.UserId
	p.PlayerName = rsp.Player.PlayerName

	p.Debugf("[%s] [ActorCreate] PlayerID = %d,ActorName = %s", p.TagName, p.UserId, p.PlayerName)

	return nil
}

// ActorEnter 角色进入游戏
func (p *Robot) ActorEnter() error {
	route := "game.player.enter"
	req := &pb.Int64{
		Value: p.UserId,
	}

	msg, err := p.Request(route, req)
	if err != nil {
		return err
	}

	rsp := &pb.PlayerEnterResponse{}
	err = p.Serializer().Unmarshal(msg.Data, rsp)
	if err != nil {
		return err
	}

	p.Debugf("[%s] [ActorEnter] response PlayerID = %d,ActorName = %s", p.TagName, p.UserId, p.PlayerName)
	return nil
}

// ActorEnterEnterMachine 角色进入机台
func (p *Robot) ActorEnterEnterMachine() error {
	route := "game.slots.entermachine"
	req := &pb.EnterMachine{
		Id:        86001,
		SelectBet: 100000,
	}

	msg, err := p.Request(route, req)
	if err != nil {
		return err
	}

	rsp := &pb.EnterMachineResponse{}
	err = p.Serializer().Unmarshal(msg.Data, rsp)
	if err != nil {
		return err
	}

	p.Debugf("[%s] [ActorEnterEnterMachine] response PlayerID = %d,ActorName = %s", p.TagName, p.UserId, p.PlayerName)
	return nil
}

// ActorMachine 角色机台信息
func (p *Robot) ActorMachine() error {
	route := "game.slots.machineinfo"
	req := &pb.MachineInfo{
		Id: 86001,
	}

	msg, err := p.Request(route, req)
	if err != nil {
		return err
	}

	rsp := &pb.MachineInfoResponse{}
	err = p.Serializer().Unmarshal(msg.Data, rsp)
	if err != nil {
		return err
	}

	p.Debugf("[%s] [ActorMachine] response PlayerID = %d,ActorName = %s", p.TagName, p.UserId, p.PlayerName)
	return nil
}

// ActorSpin 角色进入机台spin
func (p *Robot) ActorSpin() error {
	route := "game.slots.spin"
	req := &pb.Spin{
		Id:      86001,
		CurBet:  100000,
		CurCost: 10000000,
	}

	msg, err := p.Request(route, req)
	if err != nil {
		return err
	}

	rsp := &pb.SpinResponse{}
	err = p.Serializer().Unmarshal(msg.Data, rsp)
	if err != nil {
		return err
	}

	p.Debugf("[%s] [ActorSpin] response PlayerID = %d,ActorName = %s", p.TagName, p.UserId, p.PlayerName)
	return nil
}

func (p *Robot) RandSleep() {
	time.Sleep(time.Duration(rand.Int31n(10)) * time.Millisecond)
}

func (p *Robot) Debug(args ...any) {
	if p.PrintLog {
		cherryLogger.Debug(args...)
	}
}

func (p *Robot) Debugf(template string, args ...any) {
	if p.PrintLog {
		cherryLogger.Debugf(template, args...)
	}
}
