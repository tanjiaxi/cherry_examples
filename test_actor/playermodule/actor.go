package playermodule

import (
	"fmt"

	cherryFacade "github.com/cherry-game/cherry/facade"
	cherryActor "github.com/cherry-game/cherry/net/actor"
)

type Actor struct {
	cherryActor.Base // ⭐ 这是组合，不是继承
}

// ⭐ 重写AliasID方法
func (*Actor) AliasID() string {
	return "parentActor"
}

// ⭐ 重写OnInit方法
func (p *Actor) OnInit() {
	fmt.Println("[actor] Execute OnInit()")
	p.Remote().Register("callChildHello", p.callChildHello)
	childActorID := "1"
	p.Child().Create(childActorID, &childActor{})

	targetPath := cherryFacade.NewChildPath("", p.AliasID(), childActorID)
	targetFuncName := "hello"
	targetFuncName2 := "callPrarentHello"
	fmt.Println(targetPath)
	p.CallWait(targetPath, targetFuncName, nil, nil)
	//调用子actor，targetFuncName2 方法又回调用父actor
	p.CallWait(targetPath, targetFuncName2, nil, nil)
	//fmt.Println(reply)
}

func (*Actor) OnStop() {
}
func (*Actor) callChildHello() {
	text := "[childActor] Call callChildHello()"
	fmt.Println(text)
}
