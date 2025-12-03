/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 21:32:35
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 22:37:28
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/bonus/bonus_factory.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package bonus

import (
	clog "github.com/cherry-game/cherry/logger"
)

type bonusFactoryFunc func() BonusInterface

var bonusRegistry = make(map[int32]bonusFactoryFunc)

func RegisteBonus(ruleId int32, factoryFunc bonusFactoryFunc) {
	if _, ok := bonusRegistry[ruleId]; ok {
		clog.Panic("ruleId %d bonus is registered ", ruleId)
	}
	bonusRegistry[ruleId] = factoryFunc
}
func CreatBonus(ruleId int32) BonusInterface {
	if factory, ok := bonusRegistry[ruleId]; ok {
		return factory()
	}
	return nil
}
func RegisteAllBonus() {
	RegisteBonus(1, func() BonusInterface {
		bonusBase := NewBonusBaseInfo()
		return NewBonus1(*bonusBase)
	})
}
