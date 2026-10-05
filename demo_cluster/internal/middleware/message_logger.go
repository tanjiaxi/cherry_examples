package middleware

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
	"github.com/cherry-game/examples/demo_cluster/internal/pb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const maxClientPayloadDump = 2048

// clientReqFactory 客户端 route → 请求 proto。Gate 在转发前没有 handler 类型，需要这份表才能把 msg.Data 打成 JSON。
var clientReqFactory = map[string]func() proto.Message{
	"gate.user.login":         func() proto.Message { return new(pb.LoginRequest) },
	"game.player.select":      func() proto.Message { return new(pb.None) },
	"game.player.create":      func() proto.Message { return new(pb.PlayerCreateRequest) },
	"game.player.enter":       func() proto.Message { return new(pb.Int64) },
	"game.slots.enterMachine": func() proto.Message { return new(pb.EnterMachine) },
	"game.slots.machineInfo":  func() proto.Message { return new(pb.MachineInfo) },
	"game.slots.spin":         func() proto.Message { return new(pb.Spin) },
	"game.slots.bonus":        func() proto.Message { return new(pb.Bonus) },
	"game.slots.collect":      func() proto.Message { return new(pb.CollectDone) },
}

var clientPayloadJSON = protojson.MarshalOptions{
	EmitUnpopulated: true,
	UseProtoNames:   true,
}

// FormatClientPayload 把客户端 Pomelo msg.Data（protobuf 字节）解成可读 JSON。
// 未知 route 或解包失败则输出 hex，避免 string(bytes) 打出乱码。
func FormatClientPayload(route string, data []byte) string {
	if len(data) == 0 {
		return "{}"
	}
	factory, ok := clientReqFactory[route]
	if !ok {
		return dumpPayloadHex(data)
	}
	msg := factory()
	if err := proto.Unmarshal(data, msg); err != nil {
		return fmt.Sprintf("unmarshal_err=%v %s", err, dumpPayloadHex(data))
	}
	raw, err := clientPayloadJSON.Marshal(msg)
	if err != nil {
		return dumpPayloadHex(data)
	}
	s := string(raw)
	if len(s) > maxClientPayloadDump {
		return s[:maxClientPayloadDump] + "...(truncated)"
	}
	return s
}

func dumpPayloadHex(data []byte) string {
	n := len(data)
	if n > 256 {
		n = 256
		return "hex=" + hex.EncodeToString(data[:n]) + "...(truncated)"
	}
	return "hex=" + hex.EncodeToString(data)
}

// LogResponse 统一打印响应消息（包装 Agent.Response）
func LogResponse(agent *pomelo.Agent, session *cproto.Session, v interface{}, isError ...bool) {
	// 简要日志（Info 级别）
	clog.Infof("[GATE-OUT] uid=%d, sid=%s, mid=%d",
		session.Uid, session.Sid, session.GetMID())

	// 详细日志（Debug 级别）
	respJSON, _ := json.Marshal(v)
	clog.Debugf("[GATE-OUT-DETAIL] uid=%d, sid=%s, mid=%d, resp=%s",
		session.Uid, session.Sid, session.GetMID(), string(respJSON))

	// 调用原始的 Response 方法
	agent.Response(session, v, isError...)
}

// LogPush 统一打印推送消息（包装 Agent.Push）
func LogPush(agent *pomelo.Agent, route string, val interface{}) {
	// 简要日志（Info 级别）
	clog.Infof("[GATE-PUSH] uid=%d, sid=%s, route=%s",
		agent.UID(), agent.SID(), route)

	// 详细日志（Debug 级别）
	pushJSON, _ := json.Marshal(val)
	clog.Debugf("[GATE-PUSH-DETAIL] uid=%d, sid=%s, route=%s, data=%s",
		agent.UID(), agent.SID(), route, string(pushJSON))

	// 调用原始的 Push 方法
	agent.Push(route, val)
}

// LogKick 统一打印踢人消息（包装 Agent.Kick）
func LogKick(agent *pomelo.Agent, reason interface{}, closed bool) {
	reasonJSON, _ := json.Marshal(reason)
	clog.Infof("[GATE-KICK] uid=%d, sid=%s, reason=%s, closed=%v",
		agent.UID(), agent.SID(), string(reasonJSON), closed)

	// 调用原始的 Kick 方法
	agent.Kick(reason, closed)
}

// WrapAgent 包装 Agent，自动打印所有消息
type AgentWrapper struct {
	*pomelo.Agent
}

// Response 重写 Response 方法，自动打印
func (w *AgentWrapper) Response(session *cproto.Session, v interface{}, isError ...bool) {
	LogResponse(w.Agent, session, v, isError...)
}

// Push 重写 Push 方法，自动打印
func (w *AgentWrapper) Push(route string, val interface{}) {
	LogPush(w.Agent, route, val)
}

// Kick 重写 Kick 方法，自动打印
func (w *AgentWrapper) Kick(reason interface{}, closed bool) {
	LogKick(w.Agent, reason, closed)
}

// WrapAgentWithLog 包装 Agent 以自动打印消息
func WrapAgentWithLog(agent *pomelo.Agent) *AgentWrapper {
	return &AgentWrapper{Agent: agent}
}

// TrackMessage 跟踪消息处理的完整生命周期
type MessageTracker struct {
	Route     string
	UID       int64
	SID       string
	MID       uint32
	StartTime time.Time
}

// NewMessageTracker 创建消息跟踪器
func NewMessageTracker(route string, uid int64, sid string, mid uint32) *MessageTracker {
	return &MessageTracker{
		Route:     route,
		UID:       uid,
		SID:       sid,
		MID:       mid,
		StartTime: time.Now(),
	}
}

// LogRequest 记录请求
func (t *MessageTracker) LogRequest(reqData []byte) {
	clog.Infof("[MSG-IN] route=%s, uid=%d, sid=%s, mid=%d, size=%d bytes",
		t.Route, t.UID, t.SID, t.MID, len(reqData))

	if len(reqData) > 0 {
		clog.Debugf("[MSG-IN-DETAIL] route=%s, uid=%d, data=%s",
			t.Route, t.UID, FormatClientPayload(t.Route, reqData))
	}
}

// LogResponse 记录响应
func (t *MessageTracker) LogResponse(respData interface{}) {
	elapsed := time.Since(t.StartTime)

	clog.Infof("[MSG-OUT] route=%s, uid=%d, sid=%s, mid=%d, elapsed=%v",
		t.Route, t.UID, t.SID, t.MID, elapsed)

	respJSON, _ := json.Marshal(respData)
	clog.Debugf("[MSG-OUT-DETAIL] route=%s, uid=%d, elapsed=%v, resp=%s",
		t.Route, t.UID, elapsed, string(respJSON))

	// 慢请求告警
	if elapsed > 100*time.Millisecond {
		clog.Warnf("[MSG-SLOW] route=%s, uid=%d, elapsed=%v",
			t.Route, t.UID, elapsed)
	}
}

// LogError 记录错误
func (t *MessageTracker) LogError(errCode int32, err error) {
	elapsed := time.Since(t.StartTime)
	clog.Warnf("[MSG-ERROR] route=%s, uid=%d, sid=%s, mid=%d, elapsed=%v, errCode=%d, err=%v",
		t.Route, t.UID, t.SID, t.MID, elapsed, errCode, err)
}
