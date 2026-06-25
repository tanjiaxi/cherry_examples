package player

import (
	"context"
	"fmt"
	"strconv"
	"time"

	cstring "github.com/cherry-game/cherry/extend/string"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	"github.com/cherry-game/examples/demo_cluster/internal/data"
	commonDb "github.com/cherry-game/examples/demo_cluster/internal/db"
	"github.com/cherry-game/examples/demo_cluster/internal/event"
	tableModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	sessionKey "github.com/cherry-game/examples/demo_cluster/internal/session_key"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/module/online"
)

type (
	// actorPlayer 每位登录的玩家对应一个子actor
	// 作为玩家数据的中心，管理玩家的所有核心数据
	actorPlayer struct {
		pomelo.ActorBase
		isOnline bool  // 玩家是否在线
		userId   int64 //这就是userId
		uid      int64

		// 玩家核心数据（从数据库加载，内存缓存）
		playerData *PlayerData
	}

	// PlayerData 玩家核心数据
	PlayerData struct {
		tableModel.SlotsUser
	}
)

func (p *actorPlayer) OnInit() {
	clog.Debugf("[actorPlayer] path = %s init!", p.PathString())

	// 注册 session关闭的remote函数(网关触发连接断开后，会调用RPC发送该消息)
	p.Remote().Register("sessionClose", p.sessionClose)

	//处理节点之间的actor消息，比如gate->game
	p.Local().Register("select", p.playerSelect) // 注册 查看角色
	p.Local().Register("create", p.playerCreate) // 注册 创建角色
	p.Local().Register("enter", p.playerEnter)   // 注册 进入角色

	// 注册玩家数据访问方法（供其他Actor RPC调用）
	p.Remote().Register("getPlayerData", p.getPlayerData)
	// p.Remote().Register("updateMoney", p.UpdateMoney)
	// p.Remote().Register("getMoney", p.GetMoney)
	// p.Remote().Register("getLevel", p.GetLevel)
}

// sessionClose 接收角色session关闭处理
func (p *actorPlayer) sessionClose(ctx context.Context) {
	online.UnBindPlayer(p.uid)
	p.isOnline = false
	p.Exit()

	clog.Debugf("[actorPlayer] exit! uis = %d", p.uid)
}

// playerSelect 玩家查询角色列表
func (p *actorPlayer) playerSelect(ctx context.Context, session *cproto.Session, _ *pb.None) {
	done := metrics.TrackRequest("game.player.select")
	defer done(false)

	response := &pb.PlayerSelectResponse{}
	//这里改为userId
	userId := session.Uid
	if userId > 0 {
		// 游戏设定单服单角色，协议设计成可返回多角色
		playerTable, found := db.GetPlayerTable(userId)
		if found {
			playerInfo := buildPBPlayer(playerTable)
			response.List = append(response.List, &playerInfo)
		}
	}

	p.Response(session, response)
}

// playerCreate 玩家创角
func (p *actorPlayer) playerCreate(ctx context.Context, session *cproto.Session, req *pb.PlayerCreateRequest) {
	done := metrics.TrackRequest("game.player.create")
	hasError := false
	defer func() { done(hasError) }()

	if req.Gender > 1 {
		hasError = true
		p.ResponseCode(session, code.PlayerCreateFail)
		return
	}

	// 检查玩家昵称
	if len(req.PlayerName) < 1 {
		p.ResponseCode(session, code.PlayerCreateFail)
		return
	}

	// 帐号是否已经在当前游戏服存在角色
	if db.GetPlayerIdWithUID(session.Uid) > 0 {
		p.ResponseCode(session, code.PlayerCreateFail)
		return
	}

	// 获取创角初始化配置
	playerInitRow, found := data.PlayerInitConfig.Get(req.Gender)
	if found == false {
		p.ResponseCode(session, code.PlayerCreateFail)
		return
	}

	// 创建角色&添加角色初始的资产
	serverId := session.GetInt32(sessionKey.ServerID)
	newPlayerTable, errCode := db.CreatePlayer(session, req.PlayerName, serverId, playerInitRow)
	if code.IsFail(errCode) {
		p.ResponseCode(session, errCode)
		return
	}

	// TODO 更新最后一次登陆的角色信息到中心节点

	// 抛出角色创建事件
	playerCreateEvent := event.NewPlayerCreate(newPlayerTable.UserId, req.PlayerName, req.Gender)
	p.PostEvent(&playerCreateEvent)

	playerInfo := buildPBPlayer(newPlayerTable)
	response := &pb.PlayerCreateResponse{
		Player: &playerInfo,
	}

	p.Response(session, response)
}

// playerEnter 玩家进入游戏
func (p *actorPlayer) playerEnter(ctx context.Context, session *cproto.Session, req *pb.Int64) {
	done := metrics.TrackRequest("game.player.enter")
	hasError := false
	defer func() { done(hasError) }()

	startTime := time.Now()
	userId := req.Value
	if userId < 1 {
		hasError = true
		p.ResponseCode(session, code.PlayerIDError)
		return
	}

	// 检查并查找该用户下的该角色
	playerTable, found := db.GetPlayerTable(req.GetValue())
	if found == false {
		p.ResponseCode(session, code.PlayerIDError)
		return
	}

	// 保存进入游戏的玩家对应的agentPath
	online.BindPlayer(userId, playerTable.UID, session.AgentPath)

	// 设置网关节点session的PlayerID属性
	if session.ActorPath() != "" {
		p.Call(session.ActorPath(), "setSession", "", &pb.StringKeyValue{
			Key:   sessionKey.PlayerID,
			Value: cstring.ToString(userId),
		})
	}

	p.uid = playerTable.UID
	p.userId = playerTable.UserId
	p.isOnline = true // 设置为在线状态

	// 加载玩家数据到内存
	// if err := p.loadPlayerData(); err != nil {
	// 	clog.Errorf("[actorPlayer] 加载玩家数据失败: %v", err)
	// }

	// 这里改为客户端主动请求更佳
	// [01]推送角色 道具数据
	// module.Item.ListPush(session, userId)
	// [02]推送角色 英雄数据
	//module.Hero.ListPush(session, userId)
	// [03]推送角色 成就数据
	//module.Achieve.CheckNewAndPush(userId, true, true)
	// [04]推送角色 邮件数据
	//module.Mail.ListPush(session, userId)

	//查找游戏玩家数据
	err := p.loadPlayerData(int32(userId))
	if err != nil {
		clog.Errorf("user info is nil")
		hasError = true
		p.ResponseCode(session, code.PlayerIDError)
		return
	}

	// [99]最后推送 角色进入游戏响应结果
	response := &pb.PlayerEnterResponse{}
	response.GuideMaps = map[int32]int32{}

	p.Response(session, response)
	elapsed := time.Since(startTime)
	clog.Warnf("[playerEnter] 代码执行耗时: %s ", elapsed)
	// 角色登录事件
	loginEvent := event.NewPlayerLogin(p.ActorID(), userId)
	p.PostEvent(&loginEvent)
}

func buildPBPlayer(playerTable *db.PlayerTable) pb.Player {
	return pb.Player{
		UserId:     playerTable.UserId,
		PlayerName: playerTable.Name,
		Level:      playerTable.Level,
		CreateTime: playerTable.CreateTime,
		Exp:        playerTable.Exp,
		Gender:     playerTable.Gender,
	}
}

// ========== 玩家数据访问方法 ==========

// loadPlayerData 从数据库加载玩家数据到内存
func (p *actorPlayer) loadPlayerData(userId int32) error {
	//查找游戏玩家数据
	userInfo := commonDb.GetUserAllInfo(userId)
	if userInfo == nil {
		clog.Errorf("user info is nil")
		return fmt.Errorf("user info is nil")
	}
	//这里是指针copy
	p.playerData = &PlayerData{}
	//这里是值copy
	p.playerData.SlotsUser = *userInfo

	return nil
}

// GetPlayerData 获取玩家数据（Remote方法，供其他Actor调用）
func (p *actorPlayer) getPlayerData(ctx context.Context, msg *pb.Int32) (*pb.GetUserInfoResponse, int32) {
	if p.playerData == nil || p.playerData.UserID == 0 {
		err := p.loadPlayerData(msg.Value)
		if err != nil {
			clog.Errorf("[actorPlayer] 加载玩家数据失败: %v", err)
			return nil, code.PlayerIDError
		}
	}

	CurExp, _ := strconv.ParseInt(p.playerData.CurExp, 10, 64)
	return &pb.GetUserInfoResponse{
		UserId:  p.playerData.UserID,
		Money:   float32(p.playerData.Money),
		Diamond: p.playerData.Diamond,
		Level:   p.playerData.UserLevel,
		CurExp:  CurExp,
	}, code.OK
}

// // UpdateMoney 更新玩家金币（Remote方法）
// func (p *actorPlayer) UpdateMoney(delta int64) (newMoney int64, err error) {
// 	if p.playerData == nil {
// 		return 0, cerror.Error("player data not loaded")
// 	}

// 	p.playerData.Money += delta

// 	// TODO: 持久化到数据库
// 	// db.UpdatePlayerMoney(p.userId, p.playerData.Money)

// 	clog.Infof("[actorPlayer] 更新金币: userId=%d, delta=%d, newMoney=%d",
// 		p.userId, delta, p.playerData.Money)

// 	return p.playerData.Money, nil
// }

// // GetMoney 获取玩家金币（Remote方法）
// func (p *actorPlayer) GetMoney() int64 {
// 	if p.playerData == nil {
// 		p.loadPlayerData()
// 	}
// 	return p.playerData.Money
// }

// // GetLevel 获取玩家等级（Remote方法）
// func (p *actorPlayer) GetLevel() int32 {
// 	if p.playerData == nil {
// 		p.loadPlayerData()
// 	}
// 	return p.playerData.Level
// }
