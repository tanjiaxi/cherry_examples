package concurrent_cluster

import (
	"reflect"
	"sync"
	"time"

	ccode "github.com/cherry-game/cherry/code"
	creflect "github.com/cherry-game/cherry/extend/reflect"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cnats "github.com/cherry-game/cherry/net/nats"
	cproto "github.com/cherry-game/cherry/net/proto"
	cprofile "github.com/cherry-game/cherry/profile"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	Name = "cluster_component" // 使用与原 cluster 相同的名称，以便替换
)

// HandlerInfo 存储处理函数信息
type HandlerInfo struct {
	FuncInfo *creflect.FuncInfo
	Handler  interface{}
}

// Component 包装 ConcurrentCluster，实现 IComponent 接口
type Component struct {
	cfacade.Component
	*ConcurrentCluster
}

// ConcurrentCluster 并发集群组件 - 支持并发处理的集群实现
// 对于注册为并发的 handler，直接在 goroutine 中处理，绕过 Actor mailbox
type ConcurrentCluster struct {
	app               cfacade.IApplication
	prefix            string
	localSubject      string
	remoteSubject     string
	replySubject      string
	remoteTypeSubject string

	// 并发服务相关
	concurrentHandlers sync.Map // key: "actorID.funcName" -> *HandlerInfo
	workerPool         chan struct{}
	maxWorkers         int
}

// New 创建并发集群组件
func New() *Component {
	return &Component{
		ConcurrentCluster: &ConcurrentCluster{
			maxWorkers: 1000, // 默认最大并发数
		},
	}
}

func (c *Component) Name() string {
	return Name
}

func (c *Component) Init() {
	c.ConcurrentCluster.app = c.App()
	c.ConcurrentCluster.Init()
}

func (c *Component) OnStop() {
	c.ConcurrentCluster.Stop()
}

// SetMaxWorkers 设置最大并发数
func (c *Component) SetMaxWorkers(max int) *Component {
	c.ConcurrentCluster.maxWorkers = max
	return c
}

// RegisterConcurrent 注册并发处理函数
// actorID: Actor 的 AliasID，如 "account"
// funcName: 函数名，如 "getUID"
// handler: 处理函数，签名需要与原 Actor Remote 函数一致
func (c *Component) RegisterConcurrent(actorID, funcName string, handler interface{}) *Component {
	key := actorID + "." + funcName

	// 解析函数信息
	funcInfo, err := creflect.GetFuncInfo(handler)
	if err != nil {
		clog.Errorf("[RegisterConcurrent] parse handler error. key=%s, err=%v", key, err)
		return c
	}

	c.ConcurrentCluster.concurrentHandlers.Store(key, &HandlerInfo{
		FuncInfo: &funcInfo,
		Handler:  handler,
	})

	clog.Infof("[RegisterConcurrent] registered concurrent handler: %s", key)
	return c
}

// isConcurrentHandler 检查是否是并发处理函数
func (c *ConcurrentCluster) isConcurrentHandler(actorID, funcName string) (*HandlerInfo, bool) {
	key := actorID + "." + funcName
	if v, ok := c.concurrentHandlers.Load(key); ok {
		return v.(*HandlerInfo), true
	}
	return nil, false
}

// ========== ICluster 接口实现 ==========

func (c *ConcurrentCluster) Init() {
	c.loadNatsConfig()
	c.workerPool = make(chan struct{}, c.maxWorkers)

	c.localProcess()
	c.remoteProcess()
	c.remoteTypeProcess()

	clog.Info("ConcurrentCluster execute Init().")
}

func (c *ConcurrentCluster) Stop() {
	cnats.ConnectClose()
	clog.Info("ConcurrentCluster execute Stop().")
}

func (c *ConcurrentCluster) SetApp(app cfacade.IApplication) {
	c.app = app
}

func (c *ConcurrentCluster) loadNatsConfig() {
	natsConfig := cprofile.GetConfig("cluster").GetConfig("nats")
	if natsConfig.LastError() != nil {
		panic("cluster->nats config not found.")
	}

	c.prefix = natsConfig.GetString("prefix", "node")
	c.localSubject = GetLocalSubject(c.prefix, c.app.NodeType(), c.app.NodeID())
	c.remoteSubject = GetRemoteSubject(c.prefix, c.app.NodeType(), c.app.NodeID())
	c.remoteTypeSubject = GetRemoteTypeSubject(c.prefix, c.app.NodeType())
	c.replySubject = GetReplySubject(c.prefix, c.app.NodeType(), c.app.NodeID())

	cnats.NewPool(c.replySubject, natsConfig, true)
}

func (c *ConcurrentCluster) localProcess() {
	process := func(natsMsg *nats.Msg) {
		packet, err := cproto.UnmarshalPacket(natsMsg.Data)
		defer packet.Recycle()

		if err != nil {
			clog.Warnf("[localProcess] Unmarshal fail. [subject = %s, err = %s]", natsMsg.Subject, err)
			return
		}

		message := cfacade.BuildClusterMessage(packet)
		c.app.ActorSystem().PostLocal(&message)
	}

	conn := cnats.GetConnect()
	err := conn.Subscribe(c.localSubject, process)
	if err != nil {
		clog.Errorf("[localProcess] Create subscribe fail. [subject = %s, err = %v]", c.localSubject, err)
	}
}

// remoteProcess 核心：判断是否并发处理
func (c *ConcurrentCluster) remoteProcess() {
	process := func(natsMsg *nats.Msg) {
		packet, err := cproto.UnmarshalPacket(natsMsg.Data)
		if err != nil {
			clog.Warnf("[remoteProcess] Unmarshal fail. [subject = %s, err = %v]", natsMsg.Subject, err)
			packet.Recycle()
			return
		}

		// 解析目标 Actor
		targetPath, err := cfacade.ToActorPath(packet.TargetPath)
		if err != nil {
			clog.Warnf("[remoteProcess] parse target path fail. path=%s, err=%v", packet.TargetPath, err)
			packet.Recycle()
			return
		}

		// 检查是否是并发处理函数
		if handlerInfo, ok := c.isConcurrentHandler(targetPath.ActorID, packet.FuncName); ok {
			// 并发处理：直接开 goroutine
			go c.handleConcurrent(natsMsg, packet, handlerInfo)
			return
		}

		// 非并发：走原有 Actor 逻辑
		defer packet.Recycle()
		message := cfacade.BuildClusterMessage(packet)
		if len(natsMsg.Reply) > 0 {
			message.Header = natsMsg.Header
			message.Reply = natsMsg.Reply
		}
		c.app.ActorSystem().PostRemote(&message)
	}

	conn := cnats.GetConnect()
	err := conn.Subscribe(c.remoteSubject, process)
	if err != nil {
		clog.Errorf("[remoteProcess] Create subscribe fail. [subject = %s, err = %v]", c.remoteSubject, err)
	}
}

// handleConcurrent 并发处理请求
func (c *ConcurrentCluster) handleConcurrent(natsMsg *nats.Msg, packet *cproto.ClusterPacket, handlerInfo *HandlerInfo) {
	defer packet.Recycle()

	// 获取 worker
	select {
	case c.workerPool <- struct{}{}:
		defer func() { <-c.workerPool }()
	case <-time.After(3 * time.Second):
		clog.Warnf("[handleConcurrent] worker pool exhausted, timeout")
		c.sendErrorResponse(natsMsg, ccode.RPCRemoteExecuteError)
		return
	}

	startTime := time.Now()

	// 调用处理函数
	rsp := c.invokeHandler(handlerInfo, packet)

	elapsed := time.Since(startTime)
	if elapsed > 100*time.Millisecond {
		clog.Warnf("[handleConcurrent] slow request: %s.%s took %v",
			packet.TargetPath, packet.FuncName, elapsed)
	}

	// 发送响应
	if natsMsg.Reply != "" {
		c.sendResponse(natsMsg, rsp)
	}
}

// invokeHandler 调用处理函数
func (c *ConcurrentCluster) invokeHandler(handlerInfo *HandlerInfo, packet *cproto.ClusterPacket) *cproto.Response {
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
		if err := c.app.Serializer().Unmarshal(packet.ArgBytes, argPtr); err != nil {
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
			data, err := c.app.Serializer().Marshal(rets[0].Interface())
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

func (c *ConcurrentCluster) sendResponse(natsMsg *nats.Msg, rsp *cproto.Response) {
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

func (c *ConcurrentCluster) sendErrorResponse(natsMsg *nats.Msg, code int32) {
	rsp := &cproto.Response{Code: code}
	c.sendResponse(natsMsg, rsp)
}

func (c *ConcurrentCluster) remoteTypeProcess() {
	process := func(natsMsg *nats.Msg) {
		packet, err := cproto.UnmarshalPacket(natsMsg.Data)
		defer packet.Recycle()

		if err != nil {
			clog.Warnf("[remoteTypeProcess] Unmarshal fail. [subject = %s, err = %v]", natsMsg.Subject, err)
			return
		}

		message := cfacade.BuildClusterMessage(packet)
		c.app.ActorSystem().PostRemote(&message)
	}

	conn := cnats.GetConnect()
	err := conn.Subscribe(c.remoteTypeSubject, process)
	if err != nil {
		clog.Errorf("[remoteTypeProcess] Create subscribe fail. [subject = %s, err = %v]", c.remoteSubject, err)
	}
}

func (c *ConcurrentCluster) PublishLocal(nodeID string, cpacket *cproto.ClusterPacket) error {
	defer cpacket.Recycle()

	nodeType, err := c.app.Discovery().GetType(nodeID)
	if err != nil {
		return err
	}

	bytes, err := proto.Marshal(cpacket)
	if err != nil {
		return err
	}

	subject := GetLocalSubject(c.prefix, nodeType, nodeID)
	return cnats.GetConnect().Publish(subject, bytes)
}

func (c *ConcurrentCluster) PublishRemote(nodeID string, cpacket *cproto.ClusterPacket) error {
	defer cpacket.Recycle()

	nodeType, err := c.app.Discovery().GetType(nodeID)
	if err != nil {
		return err
	}

	bytes, err := proto.Marshal(cpacket)
	if err != nil {
		return err
	}

	subject := GetRemoteSubject(c.prefix, nodeType, nodeID)
	return cnats.GetConnect().Publish(subject, bytes)
}

func (c *ConcurrentCluster) PublishRemoteType(nodeType string, cpacket *cproto.ClusterPacket) error {
	defer cpacket.Recycle()

	bytes, err := proto.Marshal(cpacket)
	if err != nil {
		return err
	}

	subject := GetRemoteTypeSubject(c.prefix, nodeType)
	return cnats.GetConnect().Publish(subject, bytes)
}

func (c *ConcurrentCluster) RequestRemote(nodeID string, cpacket *cproto.ClusterPacket, timeout ...time.Duration) ([]byte, int32) {
	defer cpacket.Recycle()

	nodeType, err := c.app.Discovery().GetType(nodeID)
	if err != nil {
		return nil, ccode.DiscoveryNotFoundNode
	}

	msg, err := proto.Marshal(cpacket)
	if err != nil {
		return nil, ccode.RPCMarshalError
	}

	subject := GetRemoteSubject(c.prefix, nodeType, nodeID)
	natsData, err := cnats.GetConnect().RequestSync(subject, msg, timeout...)
	if err != nil {
		return nil, ccode.RPCRemoteExecuteError
	}

	rsp := &cproto.Response{}
	if err = proto.Unmarshal(natsData, rsp); err != nil {
		return nil, ccode.RPCUnmarshalError
	}

	return rsp.Data, rsp.Code
}

// ========== Subject 生成函数 ==========

func GetLocalSubject(prefix, nodeType, nodeID string) string {
	return prefix + ".local." + nodeType + "." + nodeID
}

func GetRemoteSubject(prefix, nodeType, nodeID string) string {
	return prefix + ".remote." + nodeType + "." + nodeID
}

func GetRemoteTypeSubject(prefix, nodeType string) string {
	return prefix + ".remote." + nodeType
}

func GetReplySubject(prefix, nodeType, nodeID string) string {
	return prefix + ".reply." + nodeType + "." + nodeID
}
