/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-02 17:49:48
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-08 15:29:32
 * @FilePath: /examples/demo_cluster/nodes/game/server/slots/spin_engine/result/gen_result1.go
 * @Description: 关卡工厂模式
 */
package result

import (
	clog "github.com/cherry-game/cherry/logger"
)

type GenRusultFactoryFunc func() ResultInterface

var genRusultRegistry = make(map[int32]GenRusultFactoryFunc)

func CreatGenResltByRoomId(ruleId int32) ResultInterface {
	if factoryFunc, ok := genRusultRegistry[ruleId]; ok {
		return factoryFunc()
	}
	return nil
}
func RegisteGenResltFactory(ruleId int32, factoryFunc GenRusultFactoryFunc) {
	if factoryFunc, ok := genRusultRegistry[ruleId]; ok {
		if factoryFunc != nil { // 已经注册过了
			clog.Panic("ruleId %d gen result is registered ", ruleId)
		}
	}
	genRusultRegistry[ruleId] = factoryFunc
}

// 注册所有关卡逻辑
func RegisteAllGenReslt() {
	RegisteGenResltFactory(86, func() ResultInterface {
		GenResultBase := NewGenResultBase()
		return NewGenResult86(*GenResultBase)
	})
}
