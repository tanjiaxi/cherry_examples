/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 15:36:55
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package result

import (
	gameModel "github.com/cherry-game/examples/demo_cluster/internal/model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

type GenResult86 struct {
	GenResultBase
	x int
	y int
}

func NewGenResult86(genResultBase GenResultBase) *GenResult86 {
	return &GenResult86{
		GenResultBase: genResultBase,
		x:             3,
		y:             3,
	}
}
func (g *GenResult86) GenResult(roomId, ruleId int32, isInit bool, bet int, collectAddMoney []int, roomCongfig *gameModel.N2CfgRoomlist, reelJsonObj *gameModel.N2CfgReelRoom, roomDataInfo *slotsModel.RoomDataInfo) (*pb.SpinResult, error) {
	g.SetSeedSave()
	return nil, nil
}
