package pomelo

import (
	"context"
	"net"
	"time"

	ccode "github.com/cherry-game/cherry/code"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cactor "github.com/cherry-game/cherry/net/actor"
	pomeloMessage "github.com/cherry-game/cherry/net/parser/pomelo/message"
	ppacket "github.com/cherry-game/cherry/net/parser/pomelo/packet"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/nats-io/nuid"
	"go.uber.org/zap/zapcore"
)

// OnConnectMetricsFunc 连接建立时的metrics回调函数类型
// startTime: 连接开始时间
// isComplete: true表示连接建立完成，false表示连接开始
type OnConnectMetricsFunc func(startTime time.Time, isComplete bool)

type (
	Actor struct {
		cactor.Base
		agentActorID         string
		connectors           []cfacade.IConnector
		onNewAgentFunc       OnNewAgentFunc
		onInitFunc           func()
		onConnectMetricsFunc OnConnectMetricsFunc // 连接metrics回调
	}

	OnNewAgentFunc func(newAgent *Agent)
)

func NewActor(agentActorID string) *Actor {
	if agentActorID == "" {
		panic("agentActorID is empty.")
	}

	parser := &Actor{
		agentActorID: agentActorID,
		connectors:   make([]cfacade.IConnector, 0),
		onInitFunc:   nil,
	}

	return parser
}

// OnInit Actor初始化前触发该函数
func (p *Actor) OnInit() {
	p.Remote().Register(ResponseFuncName, p.response)
	p.Remote().Register(PushFuncName, p.push)
	p.Remote().Register(KickFuncName, p.kick)
	p.Remote().Register(BroadcastName, p.broadcast)

	if p.onInitFunc != nil {
		p.onInitFunc()
	}
}

func (p *Actor) SetOnInitFunc(fn func()) {
	p.onInitFunc = fn
}

// SetOnConnectMetrics 设置连接建立时的metrics回调
func (p *Actor) SetOnConnectMetrics(fn OnConnectMetricsFunc) {
	p.onConnectMetricsFunc = fn
}

// 这里的Load函数是在vendor/github.com/cherry-game/cherry/application.go 中执行的，
func (p *Actor) Load(app cfacade.IApplication) {
	if len(p.connectors) < 1 {
		panic("connectors is nil. Please call the AddConnector(...) method add IConnector.")
	}

	cmd.init(app)

	//  Create agent actor
	if _, err := app.ActorSystem().CreateActor(p.agentActorID, p); err != nil {
		clog.Panicf("Create agent actor fail. err = %+v", err)
	}

	for _, connector := range p.connectors {
		//这里设置新的连接时，执行的函数
		connector.OnConnect(p.defaultOnConnectFunc)
		//这里开始监听ip：端口 是否有新的连接
		go connector.Start() // start connector!
	}
}

func (p *Actor) AddConnector(connector cfacade.IConnector) {
	p.connectors = append(p.connectors, connector)
}

func (p *Actor) Connectors() []cfacade.IConnector {
	return p.connectors
}

// defaultOnConnectFunc 创建新连接时，通过当前agentActor创建child agent actor
func (p *Actor) defaultOnConnectFunc(conn net.Conn) {
	start := time.Now()

	// 通过回调记录连接开始
	if p.onConnectMetricsFunc != nil {
		p.onConnectMetricsFunc(start, false)
	}

	session := &cproto.Session{
		Sid:       nuid.Next(),
		AgentPath: p.Path().String(),
		Data:      map[string]string{},
	}

	agent := NewAgent(p.App(), conn, session)

	if p.onNewAgentFunc != nil {
		p.onNewAgentFunc(&agent)
	}

	BindSID(&agent)
	agent.Run()

	elapsed := time.Since(start)

	// 通过回调记录连接完成
	if p.onConnectMetricsFunc != nil {
		p.onConnectMetricsFunc(start, true)
	}

	if elapsed > 10*time.Millisecond {
		clog.Warnf("[sid = %s] OnConnect slow: %v [address = %s]",
			agent.SID(),
			elapsed,
			agent.RemoteAddr(),
		)
	}
}

func (*Actor) SetDictionary(dict map[string]uint16) {
	pomeloMessage.SetDictionary(dict)
}

func (*Actor) SetDataCompression(compression bool) {
	pomeloMessage.SetDataCompression(compression)
}

func (*Actor) SetWriteBacklog(size int) {
	cmd.writeBacklog = size
}

func (*Actor) SetHeartbeat(t time.Duration) {
	if t.Seconds() < 1 {
		t = 60 * time.Second
	}
	cmd.heartbeatTime = t
}

func (*Actor) SetSysData(key string, value interface{}) {
	cmd.sysData[key] = value
}

func (p *Actor) SetOnNewAgent(fn OnNewAgentFunc) {
	p.onNewAgentFunc = fn
}

func (*Actor) SetOnDataRoute(fn DataRouteFunc) {
	if fn != nil {
		cmd.onDataRouteFunc = fn
	}
}

func (*Actor) SetOnPacket(typ ppacket.Type, fn PacketFunc) {
	cmd.onPacketFuncMap[typ] = fn
}

func (p *Actor) response(ctx context.Context, rsp *cproto.PomeloResponse) {
	agent, found := GetAgentWithSID(rsp.Sid)
	if !found {
		if clog.PrintLevel(zapcore.DebugLevel) {
			clog.Debugf("[response] Not found agent. [rsp = %+v]", rsp)
		}
		return
	}

	if ccode.IsOK(rsp.Code) {
		agent.ResponseMID(rsp.Mid, rsp.Data, false)
	} else {
		errRsp := &cproto.Response{
			Code: rsp.Code,
		}
		agent.ResponseMID(rsp.Mid, errRsp, true)
	}
}

func (p *Actor) push(ctx context.Context, rsp *cproto.PomeloPush) {
	if rsp.Sid != "" || rsp.Uid > 0 {
		if agent, found := GetAgent(rsp.Sid, rsp.Uid); found {
			agent.Push(rsp.Route, rsp.Data)
		}

		return
	}
}

func (p *Actor) kick(ctx context.Context, rsp *cproto.PomeloKick) {
	if rsp.Sid != "" || rsp.Uid > 0 {
		if agent, found := GetAgent(rsp.Sid, rsp.Uid); found {
			agent.Kick(rsp.Reason, rsp.Close)
		}

		return
	}
}

func (p *Actor) broadcast(ctx context.Context, rsp *cproto.PomeloBroadcast) {
	switch rsp.PushType {
	case cproto.PomeloBroadcast_AllUID:
		{
			ForeachAgent(func(agent *Agent) {
				if agent.IsBind() {
					agent.Push(rsp.Route, rsp.Data)
				}
			})

			return
		}
	case cproto.PomeloBroadcast_UID:
		{
			for _, uid := range rsp.UidList {
				if agent, found := GetAgentWithUID(uid); found {
					agent.Push(rsp.Route, rsp.Data)
				}
			}

			return
		}
	}
}
