package middleware

import (
	"encoding/json"
	"time"

	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/cherry/net/parser/pomelo"
	cproto "github.com/cherry-game/cherry/net/proto"
)

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
			t.Route, t.UID, string(reqData))
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
