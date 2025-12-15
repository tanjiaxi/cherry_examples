/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-14 18:38:54
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 关卡抽象对象
 */
package result

import (
	dbData "github.com/cherry-game/examples/demo_cluster/internal/db"
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

type SpinResult struct {
}
type ResultInterface interface {
	OnInit(roomId, ruleId int32, bet int, roomDataInfo *slotsModel.RoomDataInfo)
	GenResult(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig *gameModel.N2CfgRoomlist) (*pb.SpinResult, error)
	GetGameMap(reelsConfigBase *ReelsConfigBase) (allBalanceMap [][]int, posInSequence []int, err error)
	GetWinType() string
	GetWinLines() error
	GetAllLinesData(lineNum int) (*[][]dbData.Line, error)
	GetRoomSymbolCfg() (map[int32]*dbData.FormatCardConfig, error)
	SetSeedSave()
	InitReelLevel(roomDataInfo *slotsModel.RoomDataInfo, redBlackFluctuationVal int) int
	GetWinMoneyType(roomConfig gameModel.N2CfgRoomlist) []int
	GetReelObj(reelRoonConfig *logicGameModel.N2CfgReelRoom, reelLevel int, ruleId int32) *SingeReelsConfig
	BeforeSpin(redBlackFluctuationVal int, roomId, ruleId int32) error
	AfterSpin(roomDataInfo *slotsModel.RoomDataInfo) (*slotsModel.RoomDataInfo, error)
}
