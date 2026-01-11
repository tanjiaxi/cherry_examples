package cherryNatsCluster

import (
	"reflect"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"

	ccode "github.com/cherry-game/cherry/code"
	cerror "github.com/cherry-game/cherry/error"
	creflect "github.com/cherry-game/cherry/extend/reflect"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cnats "github.com/cherry-game/cherry/net/nats"
	cproto "github.com/cherry-game/cherry/net/proto"
	cprofile "github.com/cherry-game/cherry/profile"
	"github.com/nats-io/nats.go"
)

// HandlerInfo 存储并发处理函数信息
type HandlerInfo struct {
	FuncInfo *creflect.FuncInfo
	Handler  interface{}
}

// 全局并发处理器注册表
var (
	concurrentHandlers sync.Map // key: "actorID.funcName" -> *HandlerInfo
	workerPool         chan struct{}
	maxWorkers         = 1000
	poolOnce           sync.Once
)

// RegisterConcurrentHandler 注册并发处理函数（全局函数，在节点启动前调用）
// actorID: Actor 的 AliasID，如 "account"
// funcName: 函数名，如 "getUID"
// handler: 处理函数，签名需要与原 Actor Remote 函数一致
func RegisterConcurrentHandler(actorID, funcName string, handler interface{}) {
	key := actorID + "." + funcName

	funcInfo, err := creflect.GetFuncInfo(handler)
	if err != nil {
		clog.Errorf("[RegisterConcurrentHandler] parse handler error. key=%s, err=%v", key, err)
		return
	}

	concurrentHandlers.Store(key, &HandlerInfo{
		FuncInfo: &funcInfo,
		Handler:  handler,
	})

	clog.Infof("[RegisterConcurrentHandler] registered: %s", key)
}

// SetMaxConcurrentWorkers 设置最大并发数（在节点启动前调用）
func SetMaxConcurrentWorkers(max int) {
	maxWorkers = max
}

// isConcurrentHandler 检查是否是并发处理函数
func isConcurrentHandler(actorID, funcName string) (*HandlerInfo, bool) {
	key := actorID + "." + funcName
	if v, ok := concurrentHandlers.Load(key); ok {
		return v.(*HandlerInfo), true
	}
	return nil, false
}

type (
	Cluster struct {
		app               cfacade.IApplication
		prefix            string
		localSubject      string
		remoteSubject     string
		replySubject      string
		remoteTypeSubject string
	}
)

func New(app cfacade.IApplication) cfacade.ICluster {
	cluster := &Cluster{
		app: app,
	}

	return cluster
}

func (p *Cluster) loadNatsConfig() {
	natsConfig := cprofile.GetConfig("cluster").GetConfig("nats")
	if natsConfig.LastError() != nil {
		panic("cluster->nats config not found.")
	}

	p.prefix = natsConfig.GetString("prefix", "node")
	p.localSubject = GetLocalSubject(p.prefix, p.app.NodeType(), p.app.NodeID())
	p.remoteSubject = GetRemoteSubject(p.prefix, p.app.NodeType(), p.app.NodeID())
	p.remoteTypeSubject = GetRemoteTypeSubject(p.prefix, p.app.NodeType())
	p.replySubject = GetReplySubject(p.prefix, p.app.NodeType(), p.app.NodeID())

	cnats.NewPool(p.replySubject, natsConfig, true)
}

func (p *Cluster) Init() {
	p.loadNatsConfig()

	// 初始化 worker pool（只初始化一次）
	poolOnce.Do(func() {
		workerPool = make(chan struct{}, maxWorkers)
	})

	p.localProcess()
	p.remoteProcess()
	p.remoteTypeProcess()

	clog.Info("Nats cluster execute OnInit().")
}

func (p *Cluster) Stop() {
	cnats.ConnectClose()

	clog.Info("Nats cluster execute OnStop().")
}

func (p *Cluster) localProcess() {
	process := func(natsMsg *nats.Msg) {
		packet, err := cproto.UnmarshalPacket(natsMsg.Data)
		defer packet.Recycle()

		if err != nil {
			clog.Warnf("[localProcess] Unmarshal fail. [subject = %s, %s, err = %s]",
				natsMsg.Subject,
				packet.PrintLog(),
				err,
			)
			return
		}

		message := cfacade.BuildClusterMessage(packet)
		p.app.ActorSystem().PostLocal(&message)
	}

	conn := cnats.GetConnect()
	err := conn.Subscribe(p.localSubject, process)
	if err != nil {
		clog.Errorf("[localProcess] Create subscribe fail. [subject = %s, err = %v]",
			p.localSubject,
			err,
		)
	}
}

func (p *Cluster) remoteProcess() {
	process := func(natsMsg *nats.Msg) {
		packet, err := cproto.UnmarshalPacket(natsMsg.Data)
		if err != nil {
			clog.Warnf("[remoteProcess] Unmarshal fail. [subject = %s, err = %v]",
				natsMsg.Subject,
				err,
			)
			packet.Recycle()
			return
		}

		// 解析目标 Actor，检查是否需要并发处理
		targetPath, err := cfacade.ToActorPath(packet.TargetPath)
		if err == nil {
			if handlerInfo, ok := isConcurrentHandler(targetPath.ActorID, packet.FuncName); ok {
				// 并发处理：直接开 goroutine，绕过 Actor mailbox
				go p.handleConcurrent(natsMsg, packet, handlerInfo)
				return
			}
		}

		// 非并发：走原有 Actor 逻辑
		defer packet.Recycle()

		message := cfacade.BuildClusterMessage(packet)

		if len(natsMsg.Reply) > 0 {
			message.Header = natsMsg.Header
			message.Reply = natsMsg.Reply
		}
		clog.Debugf("rec : subject = %s, id = %s, reqID = %s",
			message.Reply, message.Header.Get("conID"), message.Header.Get("reqID"))
		p.app.ActorSystem().PostRemote(&message)
	}

	conn := cnats.GetConnect()
	err := conn.Subscribe(p.remoteSubject, process)
	if err != nil {
		clog.Errorf("[remoteProcess] Create subscribe fail. [subject = %s, err = %v]",
			p.remoteSubject,
			err,
		)
	}
}

// handleConcurrent 并发处理请求
func (p *Cluster) handleConcurrent(natsMsg *nats.Msg, packet *cproto.ClusterPacket, handlerInfo *HandlerInfo) {
	defer packet.Recycle()

	// 获取 worker（带超时）
	select {
	case workerPool <- struct{}{}:
		defer func() { <-workerPool }()
	case <-time.After(3 * time.Second):
		clog.Warnf("[handleConcurrent] worker pool exhausted, timeout. target=%s, func=%s",
			packet.TargetPath, packet.FuncName)
		p.sendErrorResponse(natsMsg, ccode.RPCRemoteExecuteError)
		return
	}

	startTime := time.Now()

	// 调用处理函数
	rsp := p.invokeHandler(handlerInfo, packet)

	elapsed := time.Since(startTime)
	if elapsed > 100*time.Millisecond {
		clog.Warnf("[handleConcurrent] timeout request: %s.%s took %v",
			packet.TargetPath, packet.FuncName, elapsed)
	}

	// 发送响应
	if natsMsg.Reply != "" {
		p.sendResponse(natsMsg, rsp)
	}
}

// invokeHandler 调用处理函数
func (p *Cluster) invokeHandler(handlerInfo *HandlerInfo, packet *cproto.ClusterPacket) *cproto.Response {
	rsp := &cproto.Response{Code: ccode.OK}

	defer func() {
		if r := recover(); r != nil {
			clog.Errorf("[invokeHandler] panic: %v, target=%s, func=%s", r, packet.TargetPath, packet.FuncName)
			rsp.Code = ccode.RPCRemoteExecuteError
		}
	}()

	fi := handlerInfo.FuncInfo

	// 解析参数
	var argValue reflect.Value
	if fi.InArgsLen > 0 && len(packet.ArgBytes) > 0 {
		argPtr := reflect.New(fi.InArgs[0].Elem()).Interface()
		if err := p.app.Serializer().Unmarshal(packet.ArgBytes, argPtr); err != nil {
			clog.Errorf("[invokeHandler] unmarshal args error: %v", err)
			rsp.Code = ccode.RPCUnmarshalError
			return rsp
		}
		argValue = reflect.ValueOf(argPtr)
	}

	// 调用函数
	var rets []reflect.Value
	if fi.InArgsLen > 0 {
		rets = fi.Value.Call([]reflect.Value{argValue})
	} else {
		rets = fi.Value.Call(nil)
	}

	// 处理返回值
	retsLen := len(rets)
	switch retsLen {
	case 1:
		// 只返回 code
		if val := rets[0].Interface(); val != nil {
			if code, ok := val.(int32); ok {
				rsp.Code = code
			}
		}
	case 2:
		// 返回 data 和 code
		if !rets[0].IsNil() {
			data, err := p.app.Serializer().Marshal(rets[0].Interface())
			if err != nil {
				clog.Errorf("[invokeHandler] marshal response error: %v", err)
				rsp.Code = ccode.RPCMarshalError
			} else {
				rsp.Data = data
			}
		}
		if val := rets[1].Interface(); val != nil {
			if code, ok := val.(int32); ok {
				rsp.Code = code
			}
		}
	}

	return rsp
}

func (p *Cluster) sendResponse(natsMsg *nats.Msg, rsp *cproto.Response) {
	rspData, _ := proto.Marshal(rsp)

	rspMsg := cnats.GetMsg()
	rspMsg.Header = natsMsg.Header
	rspMsg.Subject = natsMsg.Reply
	rspMsg.Data = rspData

	if err := cnats.GetConnect().PublishMsg(rspMsg); err != nil {
		clog.Warn(err)
	}
	cnats.ReleaseMsg(rspMsg)
}

func (p *Cluster) sendErrorResponse(natsMsg *nats.Msg, code int32) {
	rsp := &cproto.Response{Code: code}
	p.sendResponse(natsMsg, rsp)
}

func (p *Cluster) remoteTypeProcess() {
	process := func(natsMsg *nats.Msg) {
		packet, err := cproto.UnmarshalPacket(natsMsg.Data)
		defer packet.Recycle()

		if err != nil {
			clog.Warnf("[remoteTypeProcess] Unmarshal fail. [subject = %s, %s, err = %v]",
				natsMsg.Subject,
				packet.PrintLog(),
				err,
			)
			return
		}

		message := cfacade.BuildClusterMessage(packet)

		p.app.ActorSystem().PostRemote(&message)
	}

	conn := cnats.GetConnect()
	err := conn.Subscribe(p.remoteTypeSubject, process)
	if err != nil {
		clog.Errorf("[remoteTypeProcess] Create subscribe fail. [subject = %s, err = %v]",
			p.remoteSubject,
			err,
		)
	}
}

func (p *Cluster) PublishLocal(nodeID string, cpacket *cproto.ClusterPacket) error {
	defer cpacket.Recycle()

	nodeType, err := p.app.Discovery().GetType(nodeID)
	if err != nil {
		clog.Warnf("[PublishLocal] Get node type fail. [nodeID = %s, packet = %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)
		return cerror.DiscoveryNotFoundNode
	}

	bytes, err := proto.Marshal(cpacket)
	if err != nil {
		clog.Warnf("[PublishLocal] Marshal error. [nodeID = %s, packet = %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)
		return cerror.ClusterPacketMarshalFail
	}

	subject := GetLocalSubject(p.prefix, nodeType, nodeID)
	err = cnats.GetConnect().Publish(subject, bytes)
	if err != nil {
		clog.Warnf("[PublishLocal] Nats publish fail. [nodeID = %s, %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)

		return cerror.ClusterPublishFail
	}

	return nil
}

func (p *Cluster) PublishRemote(nodeID string, cpacket *cproto.ClusterPacket) error {
	defer cpacket.Recycle()

	nodeType, err := p.app.Discovery().GetType(nodeID)
	if err != nil {
		clog.Warnf("[PublishRemote] Get node type fail. [nodeID = %s, %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)
		return cerror.DiscoveryNotFoundNode
	}

	bytes, err := proto.Marshal(cpacket)
	if err != nil {
		clog.Warnf("[PublishRemote] Marshal error. [nodeID = %s, packet = %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)
		return cerror.ClusterPacketMarshalFail
	}

	subject := GetRemoteSubject(p.prefix, nodeType, nodeID)
	err = cnats.GetConnect().Publish(subject, bytes)
	if err != nil {
		clog.Warnf("[PublishRemote] Nats publish fail. [nodeID = %s, %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)

		return cerror.ClusterPublishFail
	}

	return nil
}

func (p *Cluster) PublishRemoteType(nodeType string, cpacket *cproto.ClusterPacket) error {
	defer cpacket.Recycle()

	bytes, err := proto.Marshal(cpacket)
	if err != nil {
		clog.Warnf("[PublishRemoteType] Marshal error. [nodeType = %s, packet = %s, err = %v]",
			nodeType,
			cpacket.PrintLog(),
			err,
		)
		return cerror.ClusterPacketMarshalFail
	}

	if nodeType == "" {
		return cerror.ClusterNodeTypeIsNil
	}

	if members := p.app.Discovery().ListByType(nodeType); len(members) < 1 {
		return cerror.ClusterNodeTypeMemberNotFound
	}

	subject := GetRemoteTypeSubject(p.prefix, nodeType)
	err = cnats.GetConnect().Publish(subject, bytes)
	if err != nil {
		clog.Warnf("[PublishRemoteType] Nats publish fail. [nodeType = %s, %s, err = %v]",
			nodeType,
			cpacket.PrintLog(),
			err,
		)

		return cerror.ClusterPublishFail
	}

	return nil
}

func (p *Cluster) RequestRemote(nodeID string, cpacket *cproto.ClusterPacket, timeout ...time.Duration) ([]byte, int32) {
	defer cpacket.Recycle()

	nodeType, err := p.app.Discovery().GetType(nodeID)
	if err != nil {
		clog.Warnf("[RequestRemote] Get node type fail. [nodeID = %s, %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)

		return nil, ccode.DiscoveryNotFoundNode
	}

	msg, err := proto.Marshal(cpacket)
	if err != nil {
		clog.Warnf("[RequestRemote] Marshal fail. [nodeID = %s, %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)

		return nil, ccode.RPCMarshalError
	}

	subject := GetRemoteSubject(p.prefix, nodeType, nodeID)
	natsData, err := cnats.GetConnect().RequestSync(subject, msg, timeout...)
	if err != nil {
		clog.Warnf("[RequestRemote] Nats request fail. [nodeID = %s, %s, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			err,
		)

		return nil, ccode.RPCRemoteExecuteError
	}

	rsp := &cproto.Response{}
	if err = proto.Unmarshal(natsData, rsp); err != nil {
		clog.Warnf("[RequestRemote] unmarshal fail. [nodeID = %s, %s, rsp = %v, err = %v]",
			nodeID,
			cpacket.PrintLog(),
			rsp,
			err,
		)

		return nil, ccode.RPCUnmarshalError
	}

	return rsp.Data, rsp.Code
}
