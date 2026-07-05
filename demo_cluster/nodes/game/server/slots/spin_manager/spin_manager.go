/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 23:46:24
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-07-04 23:29:09
 * @FilePath: /examples/demo_cluster/nodes/game/server/ slots/component/spin_manager.go
 * @Description: 这是进入spin，前，后的数据获取和处理。 （玩家赔率的控制，产生的数据，处理，管理关卡的数据转换提供给关卡逻辑）
 */
package spinmanage

import (
	"context"

	configCacheSlots "github.com/cherry-game/examples/demo_cluster/internal/config_cache/slots"
	logicGameModel "github.com/cherry-game/examples/demo_cluster/internal/model/logic_model"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/db/dynamodb"
	slotsModel "github.com/cherry-game/examples/demo_cluster/nodes/game/model"
)

// 组件层（全局单例）
type SpinManager struct{}

// 为spin做准备
func ReadySPin(ctx context.Context, roomId, ruleId int32, isInit bool, bet int, roomCongfig configCacheSlots.IRoomListConfig, roomDataInfo *slotsModel.RoomDataInfo, roomDataManager *dynamodb.RoomDataManager) (*pb.SpinResult, error) {
	collectAddMoney, reelJsonObj, redBlackFluctuationVal, err := SpinBefore(ruleId)
	if err != nil {
		return nil, err
	}
	// startTime := time.Now()
	SpinResult, err := StarSPin(redBlackFluctuationVal, roomId, ruleId, isInit, bet, collectAddMoney, roomCongfig, reelJsonObj, roomDataInfo)
	// 3. 计算并打印执行时间
	// elapsed := time.Since(startTime)
	// fmt.Printf("ReadySPin代码执行耗时: %s\n", elapsed) // %s 格式化 time.Duration 打印例如 2s, 2.001s
	// 也可以格式化为纳秒、毫秒等
	// fmt.Printf("ReadySPin代码执行耗时 (纳秒): %d\n", elapsed.Nanoseconds())
	if err != nil {
		return nil, err
	}
	// 这里需要需要做数据存储
	SpinAfter(ctx, roomDataInfo)
	roomDataInfo.MarkDirty()
	roomDataInfo.SpinNum++
	roomDataManager.SaveData(ctx, roomDataInfo)
	return SpinResult, nil
}

func SpinBefore(ruleId int32) ([]int, *logicGameModel.N2CfgReelRoom, int, error) {
	collectAddMoney := make([]int, 0) // 这里都是假数据
	redBlackFluctuationVal := 0       // 这里都是假数据
	reelCofig, err := configCacheSlots.GetInstance().GetN2CfgReelRoom(ruleId)
	if err != nil {
		return nil, nil, 0, err
	}
	// 这里的collectAddMoney是假数据
	return collectAddMoney, reelCofig, redBlackFluctuationVal, nil
}

// 这里是操作,存储数据
func SpinAfter(ctx context.Context, roomDataInfo *slotsModel.RoomDataInfo) {
	roomDataInfo.CurBetNum = 1000000 // 临时测试数据
}
func SpinEnd() {}
