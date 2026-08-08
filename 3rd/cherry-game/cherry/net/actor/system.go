package cherryActor

import (
	"context"
	"strings"
	"sync"
	"time"

	ccode "github.com/cherry-game/cherry/code"
	ccontext "github.com/cherry-game/cherry/extend/context"
	cutils "github.com/cherry-game/cherry/extend/utils"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cproto "github.com/cherry-game/cherry/net/proto"
)

type (
	// System Actor系统
	System struct {
		app              cfacade.IApplication
		actorMap         *sync.Map          // key:actorID, value:*actor
		localInvokeFunc  cfacade.InvokeFunc // default local func
		remoteInvokeFunc cfacade.InvokeFunc // default remote func
		wg               *sync.WaitGroup    // wait group
		callTimeout      time.Duration      // call调用超时
		arrivalTimeOut   int64              // message到达超时(毫秒)
		executionTimeout int64              // 消息执行超时(毫秒)
	}
)

func NewSystem() *System {
	system := &System{
		actorMap:         &sync.Map{},
		localInvokeFunc:  InvokeLocalFunc,
		remoteInvokeFunc: InvokeRemoteFunc,
		wg:               &sync.WaitGroup{},
		callTimeout:      3 * time.Second,
		arrivalTimeOut:   1000,
		executionTimeout: 1000,
	}

	return system
}

func (p *System) SetApp(app cfacade.IApplication) {
	p.app = app
}

func (p *System) NodeID() string {
	if p.app == nil {
		return ""
	}

	return p.app.NodeID()
}

func (p *System) Stop() {
	p.actorMap.Range(func(key, value any) bool {
		actor, ok := value.(*Actor)
		if ok {
			cutils.Try(func() {
				actor.Exit()
			}, func(err string) {
				clog.Warnf("[OnStop] - [actorID = %s, err = %s]", actor.path, err)
			})
		}
		return true
	})

	clog.Info("actor system stopping!")
	p.wg.Wait()
	clog.Info("actor system stopped!")
}

// GetIActor 根据ActorID获取IActor
func (p *System) GetIActor(id string) (cfacade.IActor, bool) {
	return p.GetActor(id)
}

// GetActor 根据ActorID获取*actor
func (p *System) GetActor(id string) (*Actor, bool) {
	actorValue, found := p.actorMap.Load(id)
	if !found {
		return nil, false
	}

	actor, found := actorValue.(*Actor)
	return actor, found
}

func (p *System) GetChildActor(actorID, childID string) (*Actor, bool) {
	parentActor, found := p.GetActor(actorID)
	if !found {
		return nil, found
	}

	return parentActor.child.GetActor(childID)
}

func (p *System) removeActor(actorID string) {
	p.actorMap.Delete(actorID)
}

// CreateActor 创建Actor
func (p *System) CreateActor(id string, handler cfacade.IActorHandler) (cfacade.IActor, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrActorIDIsNil
	}

	if actor, found := p.GetIActor(id); found {
		return actor, nil
	}

	thisActor, err := newActor(id, "", handler, p)
	if err != nil {
		return nil, err
	}

	p.actorMap.Store(id, thisActor) // add to map
	go thisActor.run()              // new actor is running!

	return thisActor, nil
}

// Call 发送远程消息(不回复)
func (p *System) Call(source, target, funcName, traceId string, arg any) int32 {
	if target == "" {
		clog.Warnf("[Call] Target path is nil. [source = %s, target = %s, funcName = %s]",
			source,
			target,
			funcName,
		)
		return ccode.ActorPathIsNil
	}

	if len(funcName) < 1 {
		clog.Warnf("[Call] FuncName error. [source = %s, target = %s, funcName = %s]",
			source,
			target,
			funcName,
		)
		return ccode.ActorFuncNameError
	}

	targetPath, err := cfacade.ToActorPath(target)
	if err != nil {
		clog.Warnf("[Call] Target path error. [source = %s, target = %s, funcName = %s, err = %v]",
			source,
			target,
			funcName,
			err,
		)
		return ccode.ActorConvertPathError
	}

	if targetPath.NodeID != "" && targetPath.NodeID != p.NodeID() {
		clusterPacket := cproto.GetClusterPacket()
		clusterPacket.SourcePath = source
		clusterPacket.TargetPath = target
		clusterPacket.FuncName = funcName
		clusterPacket.TraceId = traceId

		if arg != nil {
			argsBytes, err := p.app.Serializer().Marshal(arg)
			if err != nil {
				clog.Warnf("[Call] Marshal arg error. [targetPath = %s, error = %s]",
					target,
					err,
				)
				return ccode.ActorMarshalError
			}
			clusterPacket.ArgBytes = argsBytes
		}

		err = p.app.Cluster().PublishRemote(targetPath.NodeID, clusterPacket)
		if err != nil {
			clog.Warnf("[Call] Publish remote fail. [source = %s, target = %s, funcName = %s, err = %v]",
				source,
				target,
				funcName,
				err,
			)
			return ccode.ActorPublishRemoteError
		}
	} else {
		remoteMsg := cfacade.GetMessage()
		remoteMsg.Source = source
		remoteMsg.Target = target
		remoteMsg.FuncName = funcName
		remoteMsg.Args = arg
		remoteMsg.TraceId = traceId

		if !p.PostRemote(&remoteMsg) {
			clog.Warnf("[Call] Post remote fail. [source = %s, target = %s, funcName = %s]", source, target, funcName)
			return ccode.ActorCallFail
		}
	}

	return ccode.OK
}

// CallWait 发送远程消息(等待回复)
func (p *System) CallWait(source, target, funcName, traceId string, arg, reply any) int32 {
	sourcePath, err := cfacade.ToActorPath(source)
	if err != nil {
		clog.Warnf("[CallWait] Source path error. [source = %s, target = %s, funcName = %s, err = %v]",
			source,
			target,
			funcName,
			err,
		)
		return ccode.ActorConvertPathError
	}

	targetPath, err := cfacade.ToActorPath(target)
	if err != nil {
		clog.Warnf("[CallWait] Target path error. [source = %s, target = %s, funcName = %s, err = %v]",
			source,
			target,
			funcName,
			err,
		)
		return ccode.ActorConvertPathError
	}

	if source == target {
		clog.Warnf("[CallWait] Source path is equal target. [source = %s, target = %s, funcName = %s]",
			source,
			target,
			funcName,
		)
		return ccode.ActorSourceEqualTarget
	}

	if len(funcName) < 1 {
		clog.Warnf("[CallWait] FuncName error. [source = %s, target = %s, funcName = %s]",
			source,
			target,
			funcName,
		)
		return ccode.ActorFuncNameError
	}
	withCtx := ccontext.WithContext(context.Background(), &ccontext.CommonContext{
		TraceId: traceId,
	})
	clog.InfoContext(withCtx, "CallWait")
	// forward to remote actor
	if targetPath.NodeID != "" && targetPath.NodeID != sourcePath.NodeID {
		clusterPacket := cproto.BuildClusterPacket(source, target, funcName, traceId)

		if arg != nil {
			argsBytes, err := p.app.Serializer().Marshal(arg)
			if err != nil {
				clog.Warnf("[CallWait] Marshal arg error. [targetPath = %s, error = %s]", target, err)
				return ccode.ActorMarshalError
			}
			clusterPacket.ArgBytes = argsBytes
		}

		rspData, rspCode := p.app.Cluster().RequestRemote(targetPath.NodeID, clusterPacket, p.callTimeout)
		if ccode.IsFail(rspCode) {
			return rspCode
		}

		if reply != nil {
			if err = p.app.Serializer().Unmarshal(rspData, reply); err != nil {
				clog.Warnf("[CallWait] Marshal reply error. [targetPath = %s, error = %s]", target, err)
				return ccode.ActorMarshalError
			}
		}

	} else {
		message := cfacade.GetMessage()
		message.Source = source
		message.Target = target
		message.FuncName = funcName
		message.Args = arg
		// 容量为1的缓冲channel：即使本次调用因超时先返回，
		// 目标actor执行完毕后的回写(m.ChanResult <- rsp)也能立即写入缓冲区并返回，
		// 不会永久阻塞在无接收者的channel上，避免目标actor所在goroutine被"冻死"。
		message.ChanResult = make(chan interface{}, 1)
		message.TraceId = traceId

		var result interface{}
		// 相同节点的相同actor
		if sourcePath.ActorID == targetPath.ActorID {
			if sourcePath.ChildID == targetPath.ChildID {
				return ccode.ActorSourceEqualTarget
			}

			childActor, found := p.GetChildActor(targetPath.ActorID, targetPath.ChildID)
			if !found {
				return ccode.ActorChildIDNotFound
			}

			childActor.PostRemote(&message)
		} else {
			// 相同节点的不同actor
			if !p.PostRemote(&message) {
				clog.Warnf("[CallWait] Post remote fail. [source = %s, target = %s, funcName = %s]", source, target, funcName)
				return ccode.ActorCallFail
			}
		}

		// 使用 time.NewTimer + Stop 替代 time.After：
		// time.After 创建的 Timer 在未被触发的分支不会被回收，会一直挂在运行时的计时器堆里
		// 直到 callTimeout 到期才释放，高并发 CallWait 场景下会造成大量存活 Timer 常驻内存，
		// 加重 GC 标记阶段的扫描负担；命中正常分支后立即 Stop() 可以让 Timer 立刻可被回收。
		timer := time.NewTimer(p.callTimeout)
		defer timer.Stop()

		select {
		case result = <-message.ChanResult:
			{
				if result == nil {
					clog.Warnf("[CallWait] Response is nil. [targetPath = %s]", target)
					return ccode.ActorCallFail
				}

				rsp := result.(*cproto.Response)
				if rsp == nil {
					clog.Warnf("[CallWait] Response is nil. [targetPath = %s]", target)
					return ccode.ActorCallFail
				}

				if ccode.IsFail(rsp.Code) {
					return rsp.Code
				}

				if reply != nil {
					if rsp.Data == nil {
						clog.Warnf("[CallWait] rsp.Data is nil. [targetPath = %s, error = %s]", target, err)
					}

					err = p.app.Serializer().Unmarshal(rsp.Data, reply)
					if err != nil {
						clog.Warnf("[CallWait] Unmarshal reply error. [targetPath = %s, error = %s]", target, err)
						return ccode.ActorUnmarshalError
					}
				}
			}
		case <-timer.C:
			return ccode.ActorCallTimeout
		}
	}

	return ccode.OK
}

// Broadcast 根据节点类型发布消息
func (p *System) CallType(nodeType, actorID, funcName, traceId string, arg any) int32 {
	if actorID == "" {
		return ccode.ActorIDIsNil
	}

	if len(funcName) < 1 {
		clog.Warnf("[CallType] FuncName error. [nodeType = %s, actorID = %s, funcName = %s]",
			nodeType,
			actorID,
			funcName,
		)
		return ccode.ActorFuncNameError
	}

	clusterPacket := cproto.GetClusterPacket()
	clusterPacket.TargetPath = cfacade.NewPath("", actorID)
	clusterPacket.FuncName = funcName
	clusterPacket.TraceId = traceId
	if arg != nil {
		argsBytes, err := p.app.Serializer().Marshal(arg)
		if err != nil {
			clog.Warnf("[CallType] Marshal arg error. [nodeType = %s, actorID = %s, funcName = %s, error = %s]",
				nodeType,
				actorID,
				funcName,
				err,
			)
			return ccode.ActorMarshalError
		}
		clusterPacket.ArgBytes = argsBytes
	}

	err := p.app.Cluster().PublishRemoteType(nodeType, clusterPacket)
	if err != nil {
		clog.Warnf("[CallType] Publish remote fail. [nodeType = %s, actorID = %s, funcName = %s, err = %v]",
			nodeType,
			actorID,
			funcName,
			err,
		)
		return ccode.ActorPublishRemoteError
	}

	return ccode.OK
}

// PostRemote 提交远程消息
func (p *System) PostRemote(m *cfacade.Message) bool {
	if m == nil {
		clog.Error("Message is nil.")
		return false
	}

	if targetActor, found := p.GetActor(m.TargetPath().ActorID); found {
		if State(targetActor.state.Load()) == WorkerState {
			targetActor.PostRemote(m)
			return true
		}
		// 附带修复：actor存在但不处于WorkerState时，此前会直接 `return true`，
		// 消息被静默丢弃却对调用方报告"投递成功"。CallWait依赖这个返回值
		// 判断要不要走select等待分支；一旦被静默吞掉，调用方只能傻等到
		// callTimeout才超时返回，白白多付一次完整的超时等待时长。
		clog.Warnf("[PostRemote] actor not in WorkerState, message dropped. [source = %s, target = %s -> %s, state = %d]",
			m.Source, m.Target, m.FuncName, targetActor.state.Load())
		return false
	}

	clog.Warnf("[PostRemote] actor not found. [source = %s, target = %s -> %s]", m.Source, m.Target, m.FuncName)
	return false
}

// PostLocal 提交本地消息
func (p *System) PostLocal(m *cfacade.Message) bool {
	if m == nil {
		clog.Error("Message is nil.")
		return false
	}

	if targetActor, found := p.GetActor(m.TargetPath().ActorID); found {
		if State(targetActor.state.Load()) == WorkerState {
			targetActor.PostLocal(m)
			return true
		}
		clog.Warnf("[PostLocal] actor not in WorkerState, message dropped. [source = %s, target = %s -> %s, state = %d]",
			m.Source, m.Target, m.FuncName, targetActor.state.Load())
		return false
	}

	clog.Warnf("[PostLocal] actor not found. [source = %s, target = %s -> %s]", m.Source, m.Target, m.FuncName)

	return false
}

// PostEvent 提交事件
func (p *System) PostEvent(data cfacade.IEventData) {
	if data == nil {
		clog.Error("[PostEvent] Event is nil.")
		return
	}

	// range root actor
	p.actorMap.Range(func(key, value any) bool {
		if thisActor, found := value.(*Actor); found {
			if State(thisActor.state.Load()) == WorkerState {
				thisActor.event.Push(data)
			}

			// range child actor
			thisActor.Child().Each(func(iActor cfacade.IActor) {
				if childActor, ok := iActor.(*Actor); ok {
					childActor.event.Push(data)
				}
			})
		}
		return true
	})
}

func (p *System) SetLocalInvoke(fn cfacade.InvokeFunc) {
	if fn != nil {
		p.localInvokeFunc = fn
	}
}

func (p *System) SetRemoteInvoke(fn cfacade.InvokeFunc) {
	if fn != nil {
		p.remoteInvokeFunc = fn
	}
}

func (p *System) SetCallTimeout(d time.Duration) {
	p.callTimeout = d
}

func (p *System) SetArrivalTimeout(t int64) {
	if t > 1 {
		p.arrivalTimeOut = t
	}
}

func (p *System) SetExecutionTimeout(t int64) {
	if t > 1 {
		p.executionTimeout = t
	}
}
