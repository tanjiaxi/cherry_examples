package playermodule

import (
	"fmt"

	cherryFacade "github.com/cherry-game/cherry/facade"
	cherryActor "github.com/cherry-game/cherry/net/actor"
)

type childActor struct {
	cherryActor.Base
}

func (p *childActor) OnInit() {
	fmt.Println("[childActor] Execute OnInit()")

	p.Remote().Register("hello", p.hello)
	p.Remote().Register("localHello", p.localHello)
	p.Remote().Register("callPrarentHello", p.callPrarentHello)
}

func (p *childActor) hello() {
	text := "[childActor] Call hello()"
	fmt.Println(text)
}
func (p *childActor) localHello() {
	text := "[childActor] Call localHello()"
	fmt.Println(text)
}

func (p *childActor) callPrarentHello() {
	text := "[childActor] Call callPrarentHello()"
	parentPath := cherryFacade.NewPath(p.Path().NodeID, p.Path().ActorID)
	fmt.Println(parentPath)
	result := p.Call(parentPath, "callChildHello", nil)
	fmt.Println(text, result)
}
func (*childActor) OnStop() {
}
