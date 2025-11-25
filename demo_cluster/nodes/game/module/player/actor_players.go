/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-09-15 18:02:10
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2025-11-24 16:15:04
 * @FilePath: /examples/demo_cluster/nodes/game/module/player/actor_players.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package player

import (
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	"github.com/cherry-game/examples/demo_cluster/internal/event"
	"github.com/cherry-game/examples/demo_cluster/nodes/game/module/online"
)

type (
	// ActorPlayers 玩家总管理actor
	ActorPlayers struct {
		pomelo.ActorBase
		childExitTime time.Duration
	}
)

func (p *ActorPlayers) AliasID() string {
	return "player"
}

func (p *ActorPlayers) OnInit() {
	p.childExitTime = time.Minute * 30

	// 注册角色登陆事件
	p.Event().Register(event.PlayerLoginKey, p.onLoginEvent)
	p.Event().Register(event.PlayerLogoutKey, p.onLogoutEvent)
	p.Event().Register(event.PlayerCreateKey, p.onPlayerCreateEvent)
}

func (p *ActorPlayers) OnFindChild(msg *cfacade.Message) (cfacade.IActor, bool) {
	// 动态创建 player child actor
	childID := msg.TargetPath().ChildID
	childActor, err := p.Child().Create(childID, &actorPlayer{
		isOnline: false,
	})

	if err != nil {
		return nil, false
	}

	return childActor, true
}

// onLoginEvent 玩家登陆事件处理
func (p *ActorPlayers) onLoginEvent(e cfacade.IEventData) {
	evt, ok := e.(*event.PlayerLogin)
	if ok == false {
		return
	}

	clog.Infof("[PlayerLoginEvent] [playerId = %d, onlineCount = %d]",
		evt.PlayerId,
		online.Count(),
	)
}

// onLoginEvent 玩家登出事件处理
func (p *ActorPlayers) onLogoutEvent(e cfacade.IEventData) {
	evt, ok := e.(*event.PlayerLogout)
	if !ok {
		return
	}

	clog.Infof("[PlayerLogoutEvent] [playerId = %d, onlineCount = %d]",
		evt.PlayerId,
		online.Count(),
	)
}

// onPlayerCreateEvent 玩家创建事件
func (p *ActorPlayers) onPlayerCreateEvent(e cfacade.IEventData) {
	evt, ok := e.(*event.PlayerCreate)
	if !ok {
		return
	}

	clog.Infof("[PlayerCreateEvent] [%+v]", evt)
}

func (p *ActorPlayers) OnStop() {
	clog.Infof("onlineCount = %d", online.Count())
}
