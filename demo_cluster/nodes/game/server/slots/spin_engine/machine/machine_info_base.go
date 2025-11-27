/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-26 14:19:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-27 16:10:05
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/machine/machine_info_base.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package machine

import (
	"fmt"

	cproto "github.com/cherry-game/cherry/net/proto"
	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	spinManager "github.com/cherry-game/examples/demo_cluster/nodes/game/server/slots/spin_manager"
)

type BaseMachine struct {
	roomId       int32
	session      *cproto.Session
	roomDataInfo *spinManager.RoomDataInfo
	roomConfig   *gameModel.N2CfgRoomlist
	reelCofig    *logicGameModel.N2CfgReelRoom
	isInit       bool
}

func NewBaseMachine(roomId int32, session *cproto.Session, roomDataInfo *spinManager.RoomDataInfo) *BaseMachine {
	return &BaseMachine{
		roomId:       roomId,
		session:      session,
		roomDataInfo: roomDataInfo,
		isInit:       false,
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
	reelCofig, err := configCacheSlots.GetInstance().GetN2CfgReelRoom(b.roomId)
	if err != nil {
		return fmt.Errorf("room %d no room config ", b.roomId)
	}
	b.reelCofig = reelCofig
	b.isInit = true
	return nil
	// do something
}
func (b *BaseMachine) GetInitSpinResult() {
	// do something
}
func (b *BaseMachine) GetSpinResult() {
	// do something
}
func (b *BaseMachine) GetBase() (*pb.BaseInfo, error) {
	// pb.BaseInfo
	// do something//
	baseInfo := &pb.BaseInfo{}
	//需要获取levelconfig
	//getUserLevelConfig
	//需要获取betResult
	speBetNum := int64(1000)
	curBetNum := int64(1000)

	baseInfo.Id = b.roomId
	baseInfo.BetArray = []int32{1000, 10000, 100000}
	baseInfo.BaseMoney = b.roomConfig.Betbaseamount
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
func (b *BaseMachine) GetReelsInfo() {
	// do something
}
func (b *BaseMachine) GetPayTable() {
	// do something
}
func (b *BaseMachine) GetFeature() {
	// do something
}
func (b *BaseMachine) GetJackpot() {
	// do something
}
