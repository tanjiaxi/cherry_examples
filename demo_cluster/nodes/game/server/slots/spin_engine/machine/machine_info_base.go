/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-26 14:19:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-17 22:46:30
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/machine/machine_info_base.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package machine

import (
	"fmt"

	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	gameDb "github.com/cherry-game/examples/demo_cluster/nodes/game/db"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

type BaseMachine struct {
	roomId       int32
	session      *cproto.Session
	roomDataInfo *slotsModel.RoomDataInfo
	roomConfig   configCacheSlots.IRoomListConfig
	reelCofig    *logicGameModel.N2CfgReelRoom
	userInfo     *pb.GetUserInfoResponse
	isInit       bool
	ruleId       int32
}

func NewBaseMachine(roomId int32, session *cproto.Session, roomDataInfo *slotsModel.RoomDataInfo, userInfo *pb.GetUserInfoResponse, ruleId int32) *BaseMachine {
	return &BaseMachine{
		roomId:       roomId,
		session:      session,
		roomDataInfo: roomDataInfo,
		isInit:       false,
		userInfo:     userInfo,
		ruleId:       ruleId,
	}
}

// 初始化数据
func (b *BaseMachine) InitData() error {
	if b.isInit {
		return nil
	}
	roomConfig, err := configCacheSlots.GetInstance().GetRoomConfig(int32(b.roomId))
	if err != nil {
		return fmt.Errorf("room %d no room config ", b.roomId)
	}
	b.roomConfig = roomConfig
	// 获取reel配置
	reelCofig, err := configCacheSlots.GetInstance().GetN2CfgReelRoom(b.ruleId)
	if err != nil {
		return fmt.Errorf("room %d no room config ", b.roomId)
	}
	b.reelCofig = reelCofig
	b.isInit = true
	return nil
	// do something
}
func (b *BaseMachine) GetInitSpinResult() (*pb.SpinResponse, error) {
	// 默认实现，子类可以重写
	return &pb.SpinResponse{}, nil
}

func (b *BaseMachine) GetSpinResult(bet int64) (*pb.SpinResponse, error) {
	// 默认实现，子类可以重写
	return &pb.SpinResponse{}, nil
}
func (b *BaseMachine) GetBase() (*pb.BaseInfo, error) {
	// pb.BaseInfo
	// do something//
	baseInfo := &pb.BaseInfo{}
	//需要获取levelconfig
	fromatN2CfgLevel, err := configCacheSlots.GetInstance().GetN2CLevel(b.userInfo.Level)
	if err != nil {
		return nil, err
	}
	//getUserLevelConfig
	//需要获取betResult
	//costCoinsM 临时为1
	bets, _, _, err := gameDb.FormatUserBetArr(fromatN2CfgLevel, b.roomId, code.CostCoinsM, code.Schama)
	if err != nil {
		return nil, err
	}
	speBetNum := b.getSpeNetNum()
	curBetNum := b.roomDataInfo.CurBetNum

	if curBetNum < int64(bets[0]) {
		curBetNum = int64(bets[0])
	} else if curBetNum > int64(bets[len(bets)-1]) {
		curBetNum = int64(bets[len(bets)-1])
	}

	baseInfo.Id = b.roomId
	baseInfo.BetArray = bets
	baseInfo.BaseMoney = b.roomConfig.GetBetbaseamount()
	baseInfo.ReelSpeed = 0
	baseInfo.HasPlayed = b.roomDataInfo.SpinNum > 0
	baseInfo.DefaultBet = speBetNum
	if b.roomDataInfo.RecommendBet > 0 {
		baseInfo.DefaultBet = b.roomDataInfo.RecommendBet
	}
	baseInfo.UserBet = speBetNum
	if curBetNum > 0 {
		baseInfo.UserBet = curBetNum
	}
	return baseInfo, nil
}
func (b *BaseMachine) ConvertStage() (gameStage *pb.GameStage, err error) {
	// do something
	gameStage = &pb.GameStage{
		CurGameStage:  int32(b.roomDataInfo.Stage),
		NextGameStage: int32(b.roomDataInfo.NextStage),
	}
	return gameStage, nil
}
func (b *BaseMachine) GetReelsInfo() (*pb.ReelsInfo, error) {
	version := b.reelCofig.Version
	reelArray := gameDb.ToArrList(&b.reelCofig.Reelsequences)
	reelsInfo := &pb.ReelsInfo{
		ReelVersion: version,
		ReelArray:   reelArray,
	}

	return reelsInfo, nil
}

func (b *BaseMachine) GetPayTable() ([]*pb.PayInfo, error) {
	// 默认实现，子类可以重写
	// TODO: 实现赔付表获取逻辑
	return nil, nil
}

func (b *BaseMachine) GetFeature() (*pb.FeatureInfo, error) {
	// 默认实现，子类可以重写
	// TODO: 实现特性信息获取逻辑
	return nil, nil
}

func (b *BaseMachine) GetJackpot() error {
	// 默认实现，子类可以重写
	// TODO: 实现 Jackpot 信息获取逻辑
	return nil
}
func (b *BaseMachine) getSpeNetNum() int64 {
	speNum := b.roomDataInfo.SpeSpinBet
	return speNum
}
