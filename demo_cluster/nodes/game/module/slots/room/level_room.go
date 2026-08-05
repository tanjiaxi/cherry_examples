/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 22:24:38
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-07-14 17:53:05
 * @FilePath: /examples/demo_cluster/nodes/game/module/slots/room/level_room.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package room

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/bytedance/sonic"
	ccontext "github.com/cherry-game/cherry/extend/context"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
	asset "github.com/cherry-game/examples/demo_cluster/internal/asset"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	"github.com/cherry-game/examples/demo_cluster/internal/component/metrics"
	"github.com/cherry-game/examples/demo_cluster/internal/component/outbox"
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	rpcGame "github.com/cherry-game/examples/demo_cluster/internal/rpc/game"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db/dynamodb"
	spinEngine "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_engine/machine"
	spinManager "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_manager"
	"github.com/google/uuid"
)

// 关卡房间 cactor
// 一个玩家对应一个actor,包含了这个玩家的所有房间，
type (
	ActorRoom struct {
		pomelo.ActorBase
		curRoomId int32

		roomDataManager *dynamodb.RoomDataManager
		levelMutex      *sync.RWMutex
		// 同步控制
		syncTimer  *time.Timer
		spinCount  int
		settlement *asset.Repository
	}
)

func NewActorRoom(app cfacade.IApplication) *ActorRoom {
	a := &ActorRoom{}
	a.levelMutex = &sync.RWMutex{}
	a.roomDataManager = dynamodb.NewRoomDataManager(app)
	a.settlement, _ = asset.NewRepository(db.GetDB())
	return a
}

func (r *ActorRoom) OnInit() {
	r.Remote().Register("sessionClose", r.sessionClose)
	// r.Remote().Register("dbQriteQueue", r.HandleSaveMsg)
	// clog.Debugf("[actorRoom] path = %s init!", r.PathString())
	// 处理gate的节点actor消息
	r.Local().Register("enterMachine", r.enterMachine) // 进入关卡
	r.Local().Register("machineInfo", r.machineinfo)   // 初始化关卡数据
	r.Local().Register("spin", r.spin)                 // 关卡spin
	r.Local().Register("bonus", r.bonus)               // 关卡bonus请求
	r.Local().Register("collect", r.collect)           // 关卡collect 请求

	// 初始化玩家定时落地，使用时间轮 (5 分钟定时间隔，并加入随机扰动错开波峰)
	// delay := time.Duration(rand.Intn(20)) * time.Second
	// r.Timer().Add(delay, r.onTimerSaveTrigger, true)
}

// 定时器触发落地
func (r *ActorRoom) onTimerSaveTrigger() {
	// 扔一条 Save 消息给玩家 Actor 自己，排队处理，防止与其它玩家 Actor 外部消息发生并发冲突
	message := cfacade.GetMessage()
	message.Target = r.PathString()
	message.FuncName = "dbQriteQueue"
	// targetPath := cfacade.NewChildPath(, "dbQriteQueue","")
	// pomelo.LocalDataRoute(agent, session, route, msg, targetPath)
	r.PostRemote(&message)
}

func (r *ActorRoom) sessionClose(ctx context.Context) {
	// online.UnBindPlayer(r.uid)
	// r.isOnline = false
	// r.Exit()

	clog.Debugf("[actorPlayer] exit! uis = %d", 10)
}

func (r *ActorRoom) enterMachine(ctx context.Context, session *cproto.Session, req *pb.EnterMachine) {
	done := metrics.TrackRequest("game.slots.enterMachine")
	defer done(false)

	roomId := req.Id
	n2CfgRoomlist, error := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	response := &pb.EnterMachineResponse{
		Id:      roomId,
		Succeed: true,
	}
	if error != nil || n2CfgRoomlist == nil {
		response.Succeed = false
		r.Response(session, response)
		return
	}
	r.Response(session, response)
}

func (r *ActorRoom) machineinfo(ctx context.Context, session *cproto.Session, req *pb.MachineInfo) {
	done := metrics.TrackRequest("game.slots.machineinfo")
	hasError := false
	defer func() { done(hasError) }()

	roomId := req.Id

	// 1. 验证房间配置
	n2CfgRoomlist, error := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	if error != nil || n2CfgRoomlist == nil {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    code.NoRoomConfig,
			Message: "no room config",
		}
		r.Response(session, response)
		return
	}
	traceId := ccontext.GetTraceId(ctx)
	// 2. 获取用户信息
	userInfo, errCode := rpcGame.GetUserInfo(r.Actor, session, traceId)
	if code.IsFail(errCode) {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    errCode,
			Message: "no user info",
		}
		r.Response(session, response)
		return
	}

	// 3. 获取或初始化房间数据
	roomDataInfo := r.roomDataManager.GetData(ctx, userInfo.UserId, roomId)
	if roomDataInfo == nil {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    code.NoRoomPlayerData,
			Message: "no room data info",
		}
		r.Response(session, response)
		return
	}
	ruleId := roomId / 1000
	// 4. 使用工厂创建对应的 Machine（根据 roomId 自动选择 MachineInfo1 或 MachineInfo2）
	machine := spinEngine.CreateMachineByType(ruleId, roomId, session, roomDataInfo, userInfo)
	if machine == nil {
		clog.Errorf("创建 Machine 失败: roomId=%d, err=%v", roomId)
		response := &pb.ErrorResponse{
			Code:    code.NoRoomConfig,
			Message: "no room config",
		}
		r.Response(session, response)
		return
	}
	machine.InitData()
	// 5. 获取机器基础信息
	baseInfo, err := machine.GetBase()
	if err != nil {
		clog.Errorf("获取机器基础信息失败: roomId=%d, err=%v", roomId, err)
		response := &pb.ErrorResponse{
			Code:    110006,
			Message: "get machine info failed",
		}
		r.Response(session, response)
		return
	}

	// 6. 获取游戏阶段
	gameStage, err := machine.ConvertStage()
	if err != nil {
		clog.Errorf("获取游戏阶段失败: roomId=%d, err=%v", roomId, err)
	}
	initReels, err := machine.GetReelsInfo()
	if err != nil {
		clog.Errorf("获取轴数据失败: roomId=%d, err=%v", roomId, err)
	}
	payTable, err := machine.GetPayTable()
	if err != nil {
		clog.Errorf("获取PayTable数据失败: roomId=%d, err=%v", roomId, err)
	}
	feature, err := machine.GetFeature()
	if err != nil {
		clog.Errorf("获取fature数据失败: roomId=%d, err=%v", roomId, err)
	}
	// 7. 构造响应
	response := &pb.MachineInfoResponse{
		// 根据实际 protobuf 定义填充字段
		// 示例: Base: baseInfo, GameStage: gameStage
		Base:      baseInfo,
		Stage:     gameStage,
		InitReels: initReels,
		PayTable:  payTable,
		Feature:   feature,
		// 其他字段...
	}
	clog.Infof("获取机器信息成功: userId=%d, roomId=%d, version=%d ,feature=%v",
		userInfo.UserId, roomId, n2CfgRoomlist.GetVersion(), feature)
	r.Response(session, response)
}

func (r *ActorRoom) spin(ctx context.Context, session *cproto.Session, req *pb.Spin) {
	startTime := time.Now()
	if req.GetRequestId() == "" {
		r.ResponseCode(session, code.InvalidRequest)
		return
	}
	done := metrics.TrackRequest("game.slots.spin")
	hasError := false
	defer func() { done(hasError) }()
	userID := session.Uid
	// A. 首先回放，不能在这里之后重新跑 RNG。
	old, err := r.settlement.FindSpinOperation(ctx, userID, req.GetRequestId())
	if err != nil {
		r.ResponseCode(session, code.AssetSettleFailed)
		return
	}
	if old != nil && old.Status == "COMPLETED" {
		response := new(pb.SpinResponse)
		if err := r.App().Serializer().Unmarshal(old.ResponsePayload, response); err != nil {
			r.ResponseCode(session, code.AssetSettleFailed)
			return
		}
		r.Response(session, response)
		return
	}
	roomId := req.Id
	ruleId := roomId / 1000
	curBet := 10000 // req.CurBet
	traceId := ccontext.GetTraceId(ctx)
	// 2. 获取用户信息
	userInfo, errCode := rpcGame.GetUserInfo(r.Actor, session, traceId)
	if code.IsFail(errCode) {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    errCode,
			Message: "no user info",
		}
		r.Response(session, response)
		return
	}
	if userInfo.Money < int64(curBet) {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    code.NotEnoughMoney,
			Message: "not enough money",
		}
		r.Response(session, response)
		return
	}
	// start := time.Now()
	roomDataInfo := r.roomDataManager.GetData(ctx, userInfo.UserId, roomId)
	if roomDataInfo == nil {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    code.NoRoomPlayerData,
			Message: "no room data info",
		}
		r.Response(session, response)
		return
	}
	// 1. 验证房间配置
	n2CfgRoomlist, error := configCacheSlots.GetInstance().GetRoomConfig(roomId)
	if error != nil || n2CfgRoomlist == nil {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    code.NoRoomConfig,
			Message: "no room config",
		}
		r.Response(session, response)
		return
	}
	SpinResult, err := spinManager.ReadySPin(ctx, roomId, ruleId, false, int(curBet), n2CfgRoomlist, roomDataInfo, r.roomDataManager)
	if err != nil {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    code.GetRulstInfoError,
			Message: "spin rrsults error",
		}
		r.Response(session, response)
		return
	}
	roomDataInfo.Version++

	SpinResponse := &pb.SpinResponse{
		SpinResult:   SpinResult,
		Id:           roomId,
		SeedInfo:     nil,
		UserBet:      int64(curBet),
		Jackpot:      nil,
		MultInfo:     nil,
		SpinUserInfo: nil,
	}
	responseBytes, err := r.App().Serializer().Marshal(SpinResponse)
	if err != nil {
		r.ResponseCode(session, code.AssetSettleFailed)
		return
	}
	if err != nil {
		r.ResponseCode(session, code.AssetSettleFailed)
		return
	}
	eventID := uuid.NewString()
	eventBytes, err := sonic.Marshal(asset.BuildSpinCompletedEvent(eventID, userID, roomId, SpinResult.Bet, SpinResult.AllWin))
	if err != nil {
		r.ResponseCode(session, code.AssetSettleFailed)
		return
	}
	// // D. PG 成功是唯一可提交点。
	settled, err := r.settlement.SettleSpin(ctx, asset.SettleSpinCommand{
		OperationID: uuid.NewString(),
		EventID:     eventID,
		UserID:      userID,
		RequestID:   req.GetRequestId(),
		Bet:         SpinResult.Bet,
		Win:         SpinResult.AllWin,
		Response:    responseBytes,
		OutboxJSON:  eventBytes,
	})
	if errors.Is(err, asset.ErrInsufficientGold) {
		r.ResponseCode(session, code.NotEnoughMoney)
		return
	}
	if err != nil {
		r.ResponseCode(session, code.AssetSettleFailed)
		return
	}
	// This only places a token in a buffered channel.  It neither sends NATS
	// nor waits for any IO, so it is safe on the latency-sensitive Spin path.
	if component := r.App().Find(outbox.ComponentName); component != nil {
		component.(*outbox.Component).Wake()
	}
	//保存数据
	if err := r.roomDataManager.SaveData(ctx, roomId); err != nil {
		hasError = true
		response := &pb.ErrorResponse{
			Code:    code.GetRulstInfoError,
			Message: "save room data error",
		}
		r.Response(session, response)
		return
	}
	SpinResponse.Balance = settled.Balance
	clog.Infof("spin: userId=%d, roomId=%d, version=%d ,feature=%v",
		userInfo.UserId, roomId, roomDataInfo.Version, roomDataInfo)
	elapsed := time.Since(startTime)
	clog.Warnf("[spin] 代码执行耗时: %v", elapsed)
	r.Response(session, SpinResponse)
}

func (r *ActorRoom) bonus(ctx context.Context, session *cproto.Session, _ *pb.Bonus) {
}

func (r *ActorRoom) collect(ctx context.Context, session *cproto.Session, _ *pb.CollectDone) {
}
