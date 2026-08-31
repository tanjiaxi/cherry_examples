package cherryActor

import (
	"context"
	"strings"
	"sync/atomic"

	ctime "github.com/cherry-game/cherry/extend/time"
	cutils "github.com/cherry-game/cherry/extend/utils"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

/**
- 每个Actor独立运行在一个goroutine中，所有的逻辑都是串行处理
- Actor接收三种消息：本地消息(Local)、远程消息(Remote)、事件消息(Event)
	- 三种消息都有自己的队列(Queue)，每个队列依据FIFO原则进行消费
	- 本地消息(Local)，用于接收游戏客户端发送过来的本地消息
	- 远程消息(Remote)，用于Actor之间调用的远程消息
	- 事件消息(Event)，通过订阅/发布进行的事件消息
- Actor可以创建多个子Actor(ChildActor)，子Actor的消息由父Actor进行路由转发
- Actor可以创建多个定时器(Timer)进行定时业务的处理
- 通过cluster集群组件、discovery发现服务组件，进行跨节点的actor通信
*/

var (
	_nilActor = &Actor{}
)

var (
	InitState   State = 0
	WorkerState State = 1
	FreeState   State = 2
	StopState   State = 3
)

type (
	State int

	Actor struct {
		system *System            // actor system
		path   *cfacade.ActorPath // actor path
		// state 用 atomic.Int32 存储：会被创建者goroutine(onInit/loop写入)与
		// 任意调用方goroutine(System.PostRemote/PostLocal/PostEvent等读取)
		// 并发访问。此前是裸的 State(int) 字段，属于教科书级别的数据竞争——
		// `go test -race` 一跑就能抓到（本次修复系统性排查时通过
		// TestSystem_CallWait_* 回归测试意外触发并确认）。
		// 数据竞争在 Go 内存模型下不只是"理论上的未定义行为"：没有同步的写入
		// 对其他goroutine的可见性没有任何保证，可能导致其他actor把这个
		// 刚创建完成、其实已经 WorkerState 的actor长期误判为非WorkerState，
		// 从而让 PostRemote/PostLocal 静默丢弃消息（见下方状态判断处的
		// 关联问题：即使消息被丢弃，PostRemote仍返回true视为"已投递"）。
		state            atomic.Int32          // actor state (State)
		close            chan struct{}         // close flag
		handler          cfacade.IActorHandler // actor handler
		localMail        *mailbox              // local message mailbox
		remoteMail       *mailbox              // remote message mailbox
		event            *actorEvent           // event
		child            *actorChild           // child actor
		timer            *actorTimer           // timer
		lastAt           int64                 // last process time (count of seconds)
		arrivalElapsed   int64                 // arrival elapsed for message
		executionElapsed int64                 // execution elapsed for message
	}
)

func (p *Actor) run() {
	p.onInit()
	defer p.onStop()

	for {
		if p.loop() {
			break
		}
	}
}

func (p *Actor) loop() bool {
	if State(p.state.Load()) == StopState {
		if p.localMail.Count() < 1 &&
			p.remoteMail.Count() < 1 &&
			p.event.Count() < 1 {
			return true
		}
	}

	select {
	case <-p.localMail.C:
		{
			p.processLocal()
		}
	case <-p.remoteMail.C:
		{
			p.processRemote()
		}
	case <-p.event.C:
		{
			p.processEvent()
		}
	case <-p.close:
		{
			p.state.Store(int32(StopState))
		}
	}

	return false
}

func (p *Actor) processLocal() {
	m := p.localMail.Pop()
	if m == nil {
		return
	}

	p.lastAt = ctime.Now().ToSecond()

	next, invoke := p.handler.OnLocalReceived(m)
	if invoke {
		p.invokeFunc(p.localMail, p.App(), p.system.localInvokeFunc, m)
	}

	if !next {
		return
	}

	if m.TargetPath().IsChild() {
		if p.path.IsChild() {
			p.invokeFunc(p.localMail, p.App(), p.system.localInvokeFunc, m)
		} else {
			if childActor, foundChild := p.findChildActor(m); foundChild {
				childActor.PostLocal(m)
			} else {
				// clog.Warnf("Child actor not found. path = %s", m.Target)
				clog.WarnContext(context.Background(), "Child actor not found", zap.String("path", m.Target))
			}
		}
	} else {
		p.invokeFunc(p.localMail, p.App(), p.system.localInvokeFunc, m)
	}
}

func (p *Actor) processRemote() {
	m := p.remoteMail.Pop()
	if m == nil {
		return
	}

	p.lastAt = ctime.Now().ToSecond()

	next, invoke := p.handler.OnRemoteReceived(m)
	if invoke {
		p.invokeFunc(p.remoteMail, p.App(), p.system.remoteInvokeFunc, m)
	}

	if !next {
		return
	}

	if m.TargetPath().IsChild() {
		//这里表示已经到达执行的actor了(找到最底层的child,直接执行函数了)
		if p.path.IsChild() {
			p.invokeFunc(p.remoteMail, p.App(), p.system.remoteInvokeFunc, m)
		} else {
			//这里查找child,然后把m,放在邮箱
			if childActor, foundChild := p.findChildActor(m); foundChild {
				childActor.PostRemote(m)
			} else {
				// clog.Warnf("Child actor not found. path = %s", m.Target)
				clog.WarnContext(context.Background(), "Child actor not found", zap.String("path", m.Target))
			}
		}
	} else {
		p.invokeFunc(p.remoteMail, p.App(), p.system.remoteInvokeFunc, m)
	}
}

func (p *Actor) processEvent() {
	eventData := p.event.Pop()
	if eventData == nil {
		return
	}

	p.lastAt = ctime.Now().ToSecond()
	p.event.invokeFunc(eventData)
}

func (p *Actor) invokeFunc(mb *mailbox, app cfacade.IApplication, fn cfacade.InvokeFunc, m *cfacade.Message) {
	funcInfo, found := mb.funcMap[m.FuncName]
	if !found {
		// clog.Warnf("[%s] Function not found. [source = %s, target = %s -> %s]",
		// 	mb.name,
		// 	m.Source,
		// 	m.Target,
		// 	m.FuncName,
		// )
		clog.ErrorContext(context.Background(), "Function not found", zap.String("boxName", mb.name), zap.String("source", m.Source), zap.String("path", m.Target), zap.String("function", m.FuncName))
		return
	}
	//这里是检测消息在队列中等待的时间超时
	p.arrivalElapsed = m.PostTime - m.BuildTime
	if p.arrivalElapsed > p.system.arrivalTimeOut {
		// clog.Warnf("[%s] Arrival timeout.[path = %s -> %s -> %s, postTime = %d, buildTime = %d, arrival = %dms]",
		// 	mb.name,
		// 	m.Source,
		// 	m.Target,
		// 	m.FuncName,
		// 	m.PostTime,
		// 	m.BuildTime,
		// 	p.arrivalElapsed,
		// )
		clog.WarnContext(context.Background(), "Arrival timeout", zap.String("boxName", mb.name), zap.String("source", m.Source), zap.String("target", m.Target), zap.String("path", m.Target), zap.String("function", m.FuncName), zap.Int64("arrival", p.arrivalElapsed))
	}

	now := ctime.Now().ToMillisecond()

	defer func() {
		//这是消息执行超时的
		p.executionElapsed = ctime.Now().ToMillisecond() - now
		if p.executionElapsed > p.system.executionTimeout {
			// clog.Warnf("[%s] Invoke timeout.[source = %s, target = %s->%s, execution = %dms]",
			// 	mb.name,
			// 	m.Source,
			// 	m.Target,
			// 	m.FuncName,
			// 	p.executionElapsed,
			// )
			clog.WarnContext(context.Background(), "Invoke timeout", zap.String("boxName", mb.name), zap.String("source", m.Source), zap.String("target", m.Target), zap.String("path", m.Target), zap.String("function", m.FuncName), zap.Int64("execution", p.executionElapsed))
		}

		if rev := recover(); rev != nil {
			clog.ErrorContext(
				context.Background(),
				"Invoke error",
				zap.String("boxName", mb.name),
				zap.String("source", m.Source),
				zap.String("target", m.Target),
				zap.String("path", m.Target),
				zap.String("function", m.FuncName),
				zap.Any("type", funcInfo.InArgs),
				zap.Any("panic", rev),
			)
		}
	}()

	fn(app, funcInfo, m)
}

func (p *Actor) findChildActor(m *cfacade.Message) (*Actor, bool) {
	// 如果当前actor为子actor,则终止本次消息处理
	if p.path.IsChild() {
		// clog.Warnf("[findChildActor] Child actor cannot be created again。[target = %s->%s]",
		// 	m.Target,
		// 	m.FuncName,
		// )
		clog.WarnContext(context.Background(), "[findChildActor] Child actor cannot be created again", zap.String("target", m.Target), zap.String("function", m.FuncName))
		return nil, false
	}

	// 寻找childActor
	childActor, found := p.child.Get(m.TargetPath().ChildID)
	if !found {
		childActor, found = p.handler.OnFindChild(m)
	}

	if found {
		if cActor, ok := childActor.(*Actor); ok {
			return cActor, true
		}
	}

	return nil, false
}

func (p *Actor) onInit() {
	p.state.Store(int32(WorkerState))
	//这里注册函数
	p.handler.OnInit()
}

func (p *Actor) onStop() {
	cutils.Try(func() {
		close(p.close)

		if p.path.IsParent() {
			p.system.removeActor(p.ActorID())
			p.child.onStop()
		} else {
			if parent, found := p.system.GetActor(p.path.ActorID); found {
				parent.child.Remove(p.path.ChildID)
			}
		}

		p.handler.OnStop()
		p.timer.onStop()
		p.event.onStop()
		p.localMail.onStop()
		p.remoteMail.onStop()
	}, func(errString string) {
		// clog.Error(errString)
		clog.ErrorContext(context.Background(), "actor stop error", zap.String("error", errString))
	})

	p.system.wg.Done()
}

func (p *Actor) State() State {
	return State(p.state.Load())
}

func (p *Actor) App() cfacade.IApplication {
	return p.system.app
}

func (p *Actor) ActorID() string {
	if p.path.IsChild() {
		return p.path.ChildID
	}

	return p.path.ActorID
}

func (p *Actor) Path() *cfacade.ActorPath {
	return p.path
}

func (p *Actor) PathString() string {
	return p.path.String()
}

func (p *Actor) Call(targetPath, funcName, traceId string, arg any) int32 {
	return p.system.Call(p.path.String(), targetPath, funcName, traceId, arg)
}

func (p *Actor) CallWait(targetPath, funcName, traceId string, arg, reply any) int32 {
	return p.system.CallWait(p.path.String(), targetPath, funcName, traceId, arg, reply)
}

func (p *Actor) CallType(nodeType, actorID, funcName, traceId string, arg any) int32 {
	return p.system.CallType(nodeType, actorID, funcName, traceId, arg)
}

// LastAt second
func (p *Actor) LastAt() int64 {
	return p.lastAt
}

func (p *Actor) Exit() {
	p.close <- struct{}{}

	if clog.PrintLevel(zapcore.DebugLevel) {
		// clog.Debugf("[Exit] actor exit! path = %s", p.path)
		clog.DebugContext(context.Background(), "actor exit", zap.String("path", p.path.String()))
	}
}

func (p *Actor) System() *System {
	return p.system
}

func (p *Actor) Local() IMailBox {
	return p.localMail
}

func (p *Actor) Remote() IMailBox {
	return p.remoteMail
}

func (p *Actor) Event() IEvent {
	return p.event
}

func (p *Actor) Child() cfacade.IActorChild {
	return p.child
}

func (p *Actor) Timer() ITimer {
	return p.timer
}

func (p *Actor) PostRemote(m *cfacade.Message) {
	p.remoteMail.Push(m)
}

func (p *Actor) PostLocal(m *cfacade.Message) {
	p.localMail.Push(m)
}

func (p *Actor) PostEvent(data cfacade.IEventData) {
	p.system.PostEvent(data)
}

func newActor(actorID, childID string, handler cfacade.IActorHandler, c *System) (*Actor, error) {
	if strings.TrimSpace(actorID) == "" {
		// clog.Error("[newActor] actor id is nil.")
		clog.ErrorContext(context.Background(), "[newActor] actor id is nil")
		return _nilActor, ErrActorIDIsNil
	}

	thisActor := Actor{
		path: &cfacade.ActorPath{
			NodeID:  c.NodeID(),
			ActorID: actorID,
			ChildID: childID,
		},
		// state 字段类型是 atomic.Int32，其零值(0)恰好等于 InitState，
		// 不需要（也不能，两者类型不同）在此显式赋值。
		system:  c,
		close:   make(chan struct{}, 1),
		handler: handler,
		lastAt:  ctime.Now().ToSecond(),
	}

	localMailbox := newMailbox(LocalName)
	thisActor.localMail = &localMailbox

	remoteMailbox := newMailbox(RemoteName)
	thisActor.remoteMail = &remoteMailbox

	event := newEvent(&thisActor)
	thisActor.event = &event

	child := newChild(&thisActor)
	thisActor.child = &child

	timer := newTimer(&thisActor)
	thisActor.timer = &timer

	// register update timer func
	thisActor.remoteMail.Register(updateTimerFuncName, thisActor.timer._updateTimer_)

	// spawn load!
	actorLoad, ok := handler.(IActorLoader)
	if ok {
		actorLoad.load(&thisActor)
	}

	c.wg.Add(1)

	return &thisActor, nil
}
