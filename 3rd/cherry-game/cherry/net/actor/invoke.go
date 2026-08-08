package cherryActor

import (
	"context"
	"reflect"

	"go.uber.org/zap"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	ccode "github.com/cherry-game/cherry/code"
	cerror "github.com/cherry-game/cherry/error"
	ccontext "github.com/cherry-game/cherry/extend/context"
	creflect "github.com/cherry-game/cherry/extend/reflect"
	cutils "github.com/cherry-game/cherry/extend/utils"
	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	cnats "github.com/cherry-game/cherry/net/nats"
	cproto "github.com/cherry-game/cherry/net/proto"
)

// protoToJSON 将 Proto 消息转为 JSON 字符串
func protoToJSON(msg interface{}) string {
	if msg == nil {
		return "{}"
	}

	// 尝试转换为 proto.Message
	if protoMsg, ok := msg.(proto.Message); ok {
		jsonBytes, err := protojson.Marshal(protoMsg)
		if err != nil {
			return "{}"
		}
		return string(jsonBytes)
	}

	return "{}"
}

func InvokeLocalFunc(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) {
	values := make([]reflect.Value, 3)
	cxt := context.Background()
	withCtx := ccontext.WithContext(cxt, &ccontext.CommonContext{
		TraceId: m.TraceId,
	})
	if app == nil {
		// clog.Errorf("[InvokeLocalFunc] app is nil. [message = %+v]", m)
		clog.ErrorContext(withCtx, "InvokeLocalFunc app is nil", zap.Any("message", m))
		return
	}

	EncodeLocalArgs(app, fi, m)
	// 记录请求日志（业务层）
	if m.Session != nil {
		// clog.Infof("[BIZ-IN] uid=%d, sid=%s, route=%s->%s",
		// 	m.Session.Uid, m.Session.Sid, m.Target, m.FuncName)
		// clog.Debugf("[BIZ-IN-DETAIL] uid=%d, sid=%s, route=%s->%s, args=%s",
		// 	m.Session.Uid, m.Session.Sid, m.Target, m.FuncName, protoToJSON(m.Args))
		clog.DebugContext(withCtx, "InvokeLocalFunc", zap.Int64("Uid", m.Session.Uid), zap.String("Sid", m.Session.Sid), zap.String("Target", m.Target), zap.String("FuncName", m.FuncName), zap.Any("Args", m.Args))
	}

	values[0] = reflect.ValueOf(withCtx)
	values[1] = reflect.ValueOf(m.Session) // session
	values[2] = reflect.ValueOf(m.Args)    // args
	fi.Value.Call(values)

	// Local 调用没有返回值，不记录响应日志
}

func InvokeRemoteFunc(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) {
	values := make([]reflect.Value, fi.InArgsLen)
	cxt := context.Background()
	withCtx := ccontext.WithContext(cxt, &ccontext.CommonContext{
		TraceId: m.TraceId,
	})
	if app == nil {
		clog.Errorf("[InvokeRemoteFunc] app is nil. [message = %+v]", m)
		clog.ErrorContext(withCtx, "InvokeRemoteFunc app is nil", zap.Any("message", m))
		return
	}

	EncodeRemoteArgs(app, fi, m)

	// 记录 Remote 请求日志（业务层 RPC 调用）
	// clog.Infof("[BIZ-RPC-IN] source=%s, target=%s->%s",
	// 	m.Source, m.Target, m.FuncName)
	// clog.Debugf("[BIZ-RPC-IN-DETAIL] source=%s, target=%s->%s, args=%s",
	// 	m.Source, m.Target, m.FuncName, protoToJSON(m.Args))
	clog.DebugContext(withCtx, "InvokeRemoteFunc", zap.String("Source", m.Source), zap.String("Target", m.Target), zap.String("FuncName", m.FuncName), zap.String("Args", protoToJSON(m.Args)))
	//构建一个contxt 传递公共参数
	values[0] = reflect.ValueOf(withCtx)
	if fi.InArgsLen > 1 {
		values[1] = reflect.ValueOf(m.Args) // args
	}

	if m.IsCluster {
		rets := fi.Value.Call(values)

		if m.Reply == "" {
			return
		}

		cutils.Try(func() {
			rsp := retValue(app.Serializer(), rets)

			// 记录 Remote 响应日志（业务层 RPC 响应）
			// clog.Infof("[BIZ-RPC-OUT] source=%s, target=%s->%s, code=%d",
			// 	m.Source, m.Target, m.FuncName, rsp.Code)
			// clog.Debugf("[BIZ-RPC-OUT-DETAIL] source=%s, target=%s->%s, code=%d, data=%s",
			// 	m.Source, m.Target, m.FuncName, rsp.Code, protoToJSON(rsp))
			clog.DebugContext(withCtx, "InvokeRemoteFunc response", zap.String("Source", m.Source), zap.String("Target", m.Target), zap.String("FuncName", m.FuncName), zap.Int32("Code", rsp.Code), zap.String("Rsp", protoToJSON(rsp)))

			retResponse(m, rsp)
		}, func(errString string) {
			retResponse(m, &cproto.Response{
				Code: ccode.RPCRemoteExecuteError,
			})
			// clog.Errorf("[InvokeRemoteFunc] invoke error. [message = %+v, err = %s]", m, errString)
			clog.ErrorContext(withCtx, "[InvokeRemoteFunc] invoke error", zap.Any("message", m), zap.String("err", errString))
		})
	} else {
		cutils.Try(func() {
			if m.ChanResult == nil {
				fi.Value.Call(values)
			} else {
				rets := fi.Value.Call(values)
				rsp := retValue(app.Serializer(), rets)

				// 记录 Remote 响应日志（非集群调用）
				// clog.Infof("[BIZ-RPC-OUT] source=%s, target=%s->%s, code=%d",
				// 	m.Source, m.Target, m.FuncName, rsp.Code)
				// clog.Debugf("[BIZ-RPC-OUT-DETAIL] source=%s, target=%s->%s, code=%d, data=%s",
				// 	m.Source, m.Target, m.FuncName, rsp.Code, protoToJSON(rsp))
				clog.DebugContext(withCtx, "InvokeRemoteFunc response", zap.String("Source", m.Source), zap.String("Target", m.Target), zap.String("FuncName", m.FuncName), zap.Int32("Code", rsp.Code), zap.String("Rsp", protoToJSON(rsp)))
				sendChanResult(m.ChanResult, rsp)
			}
		}, func(errString string) {
			if m.ChanResult != nil {
				sendChanResult(m.ChanResult, nil)
			}

			// clog.Errorf("[InvokeRemoteFunc] invoke error.[source = %s, target = %s -> %s, funcType = %v, err = %+v]",
			// 	m.Source,
			// 	m.Target,
			// 	m.FuncName,
			// 	fi.InArgs,
			// 	errString,
			// )
			clog.ErrorContext(withCtx, "[InvokeRemoteFunc] invoke error", zap.String("source", m.Source), zap.String("target", m.Target), zap.String("FuncName", m.FuncName), zap.Any("funcType", fi.InArgs), zap.String("err", errString))
		})
	}
}

func EncodeRemoteArgs(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) error {
	if m.IsCluster {
		if fi.InArgsLen == 0 || m.Args == nil {
			return nil
		}
		if buf, ok := m.Args.([]uint8); ok && buf == nil {
			return nil
		}
		//因为这里0的位子被ctx,占用了
		return EncodeArgs(app, fi, 1, m)
	}

	return nil
}

func EncodeLocalArgs(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) error {
	return EncodeArgs(app, fi, 2, m)
}

func EncodeArgs(app cfacade.IApplication, fi *creflect.FuncInfo, index int, m *cfacade.Message) error {
	argBytes, ok := m.Args.([]byte)
	if !ok {
		return cerror.Errorf("Encode args error.[source = %s, target = %s -> %s, funcType = %v]",
			m.Source,
			m.Target,
			m.FuncName,
			fi.InArgs,
		)
	}
	argValue := reflect.New(fi.InArgs[index].Elem()).Interface()
	err := app.Serializer().Unmarshal(argBytes, argValue)
	if err != nil {
		return cerror.Errorf("Encode args unmarshal error.[source = %s, target = %s -> %s, funcType = %v]",
			m.Source,
			m.Target,
			m.FuncName,
			fi.InArgs,
		)
	}

	m.Args = argValue

	return nil
}

func retValue(serializer cfacade.ISerializer, rets []reflect.Value) *cproto.Response {
	rsp := &cproto.Response{
		Code: ccode.OK,
	}

	retsLen := len(rets)
	switch retsLen {
	case 1:
		if val := rets[0].Interface(); val != nil {
			if c, ok := val.(int32); ok {
				rsp.Code = c
			}
		}
	case 2:
		if !rets[0].IsNil() {
			data, err := serializer.Marshal(rets[0].Interface())
			if err != nil {
				rsp.Code = ccode.RPCRemoteExecuteError
				// clog.Error(err)
				clog.ErrorContext(context.Background(), "retValue", zap.Any("err", err))
			} else {
				rsp.Data = data
			}
		}

		if val := rets[1].Interface(); val != nil {
			if c, ok := val.(int32); ok {
				rsp.Code = c
			}
		}
	}

	return rsp
}

// sendChanResult 以非阻塞方式回写 CallWait 的结果。
//
// 为什么必须是非阻塞的：ChanResult 目前是容量为1的缓冲channel（见 system.go
// CallWait），这只能保证"迟到一次"的回复不会卡住发送方。但只要业务的
// OnLocalReceived/OnRemoteReceived 返回的 (next, invoke) 组合被误用成同时
// 为 true（框架里这只是两个裸bool，没有任何编译期约束防止误用），就会对
// 同一条Message触发两次invokeFunc，从而产生两次发送——第二次发送时缓冲区
// 已满且CallWait早已经放弃等待，同样会把当前actor的串行处理协程永久卡死。
// 用 select+default 的非阻塞发送，把"发送失败"退化成"静默丢弃"，
// 无论是超时晚到、还是重复invoke，都不可能再阻塞 actor 自身的goroutine。
func sendChanResult(ch chan interface{}, v interface{}) {
	select {
	case ch <- v:
	default:
		clog.Warnf("[sendChanResult] receiver gone or duplicate invoke, result dropped")
	}
}

func retResponse(m *cfacade.Message, rsp *cproto.Response) {
	rspData, _ := proto.Marshal(rsp)

	rspMsg := cnats.GetMsg()
	rspMsg.Header = m.Header
	rspMsg.Subject = m.Reply
	rspMsg.Data = rspData
	// send id = 4, reqID = 21
	// clog.Debugf("[retResponse] Sending response: subject = %s, id = %s, reqID = %s, code = %d",
	// 	m.Reply, m.Header.Get("conID"), m.Header.Get("reqID"), int(rsp.Code))
	// clog.DebugContext(withCtx, "retResponse", zap.String("subject", m.Reply), zap.String("id", m.Header.Get("conID")), zap.String("reqID", m.Header.Get("reqID")), zap.Int32("code", rsp.Code)
	if err := cnats.GetConnect().PublishMsg(rspMsg); err != nil {
		// clog.Warn(err)
		clog.WarnContext(context.Background(), "retResponse", zap.Any("err", err))
	}

	cnats.ReleaseMsg(rspMsg)
	m.Destory()
}
