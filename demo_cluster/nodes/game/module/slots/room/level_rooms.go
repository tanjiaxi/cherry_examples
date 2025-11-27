/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-11-20 22:24:34
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-26 11:32:44
 * @FilePath: /examples/demo_cluster/nodes/game/module/slots/room/level_rooms.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package slots

import (
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	"github.com/cherry-game/cherry/net/parser/pomelo"
)

// 关卡 房间管理actor
type (
	// 玩家房间总管理actor
	ActorRooms struct {
		pomelo.ActorBase
		childExitTime time.Duration
	}
)

func (p *ActorRooms) AliasID() string {
	return "slots"
}
func (p *ActorRooms) OnInit() {
	p.childExitTime = time.Minute * 30
}
func (p *ActorRooms) OnFindChild(msg *cfacade.Message) (cfacade.IActor, bool) {
	// 动态创建 player child actor
	childID := msg.TargetPath().ChildID
	childActor, err := p.Child().Create(childID, NewActorRoom())

	if err != nil {
		return nil, false
	}

	return childActor, true
}
