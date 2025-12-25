/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-15 22:06:17
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-12-23 15:58:23
 * @FilePath: /examples/vendor/github.com/cherry-game/cherry/net/actor/facade.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package cherryActor

import (
	"time"

	creflect "github.com/cherry-game/cherry/extend/reflect"
	cfacade "github.com/cherry-game/cherry/facade"
)

type (
	IActorLoader interface {
		load(actor *Actor)
	}
)

type (
	IEvent interface {
		Register(name string, fn IEventFunc, uniqueID ...int64)     // 注册事件
		Registers(names []string, fn IEventFunc, uniqueID ...int64) // 注册多个事件
		Unregister(name string)                                     // 注销事件
	}

	IEventFunc func(cfacade.IEventData) // 接收事件数据时的处理函数
)

type (
	IMailBox interface {
		Register(funcName string, fn interface{}) // 注册执行函数
		GetFuncInfo(funcName string) (*creflect.FuncInfo, bool)
		Count() int32 // 当前队列数量
	}
)

type (
	ITimer interface {
		Add(d time.Duration, fn func(), async ...bool) uint64                   // 添加定时器,循环执行
		AddOnce(d time.Duration, fn func(), async ...bool) uint64               // 添加定时器,执行一次
		AddFixedHour(hour, minute, second int, fn func(), async ...bool) uint64 // 固定x小时x分x秒,循环执行
		AddFixedMinute(minute, second int, fn func(), async ...bool) uint64     // 固定x分x秒,循环执行
		AddSchedule(s ITimerSchedule, f func(), async ...bool) uint64           // 添加自定义调度
		Remove(id uint64)                                                       // 移除定时器
		RemoveAll()                                                             // 移除所有定时器
	}

	ITimerSchedule interface {
		Next(time.Time) time.Time
	}
)
