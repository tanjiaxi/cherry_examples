/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 21:32:30
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-02 22:38:39
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/colloect/collect_factory.go
 * @Description: collect 的工厂
 */
package collect

import (
	clog "github.com/cherry-game/cherry/logger"
)

type CollectFactoryFunc func() CollectInterface

var collectRegistry = make(map[int32]CollectFactoryFunc)

func RegisterCollect(ruleId int32, factory CollectFactoryFunc) {
	if factoryFunc, ok := collectRegistry[ruleId]; ok {
		if factoryFunc != nil { // 已经注册过了
			clog.Panic("ruleId %d collect is registered ", ruleId)
		}

	}
	collectRegistry[ruleId] = factory
}
func CreatCollect(ruleId int32) CollectInterface {
	if factory, ok := collectRegistry[ruleId]; ok {
		return factory()
	}
	return nil
}
func RegisteAllCollect() {
	// 注册所有的收集器
	RegisterCollect(1, func() CollectInterface {
		collectBase := NewCollectBase()
		return NewCollect1(*collectBase)
	})

}
