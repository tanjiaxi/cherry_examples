package cherryActor

import (
	"reflect"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"

	ccode "github.com/cherry-game/cherry/code"
	cerror "github.com/cherry-game/cherry/error"
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
	if app == nil {
		clog.Errorf("[InvokeLocalFunc] app is nil. [message = %+v]", m)
		return
	}

	EncodeLocalArgs(app, fi, m)

	// 记录请求日志（业务层）
	if m.Session != nil {
		clog.Infof("[BIZ-IN] uid=%d, sid=%s, route=%s->%s",
			m.Session.Uid, m.Session.Sid, m.Target, m.FuncName)
		clog.Debugf("[BIZ-IN-DETAIL] uid=%d, sid=%s, route=%s->%s, args=%s",
			m.Session.Uid, m.Session.Sid, m.Target, m.FuncName, protoToJSON(m.Args))
	}

	values := make([]reflect.Value, 2)
	values[0] = reflect.ValueOf(m.Session) // session
	values[1] = reflect.ValueOf(m.Args)    // args
	fi.Value.Call(values)

	// Local 调用没有返回值，不记录响应日志
}

func InvokeRemoteFunc(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) {
	if app == nil {
		clog.Errorf("[InvokeRemoteFunc] app is nil. [message = %+v]", m)
		return
	}

	EncodeRemoteArgs(app, fi, m)

	// 记录 Remote 请求日志（业务层 RPC 调用）
	clog.Infof("[BIZ-RPC-IN] source=%s, target=%s->%s",
		m.Source, m.Target, m.FuncName)
	clog.Debugf("[BIZ-RPC-IN-DETAIL] source=%s, target=%s->%s, args=%s",
		m.Source, m.Target, m.FuncName, protoToJSON(m.Args))

	values := make([]reflect.Value, fi.InArgsLen)
	if fi.InArgsLen > 0 {
		values[0] = reflect.ValueOf(m.Args) // args
	}

	if m.IsCluster {
		rets := fi.Value.Call(values)

		if m.Reply == "" {
			return
		}

		cutils.Try(func() {
			rsp := retValue(app.Serializer(), rets)

			// 记录 Remote 响应日志（业务层 RPC 响应）
			clog.Infof("[BIZ-RPC-OUT] source=%s, target=%s->%s, code=%d",
				m.Source, m.Target, m.FuncName, rsp.Code)
			clog.Debugf("[BIZ-RPC-OUT-DETAIL] source=%s, target=%s->%s, code=%d, data=%s",
				m.Source, m.Target, m.FuncName, rsp.Code, protoToJSON(rsp))

			retResponse(m, rsp)
		}, func(errString string) {
			retResponse(m, &cproto.Response{
				Code: ccode.RPCRemoteExecuteError,
			})
			clog.Errorf("[InvokeRemoteFunc] invoke error. [message = %+v, err = %s]", m, errString)
		})
	} else {
		cutils.Try(func() {
			if m.ChanResult == nil {
				fi.Value.Call(values)
			} else {
				rets := fi.Value.Call(values)
				rsp := retValue(app.Serializer(), rets)

				// 记录 Remote 响应日志（非集群调用）
				clog.Infof("[BIZ-RPC-OUT] source=%s, target=%s->%s, code=%d",
					m.Source, m.Target, m.FuncName, rsp.Code)
				clog.Debugf("[BIZ-RPC-OUT-DETAIL] source=%s, target=%s->%s, code=%d, data=%s",
					m.Source, m.Target, m.FuncName, rsp.Code, protoToJSON(rsp))

				m.ChanResult <- rsp
			}
		}, func(errString string) {
			if m.ChanResult != nil {
				m.ChanResult <- nil
			}

			clog.Errorf("[InvokeRemoteFunc] invoke error.[source = %s, target = %s -> %s, funcType = %v, err = %+v]",
				m.Source,
				m.Target,
				m.FuncName,
				fi.InArgs,
				errString,
			)
		})
	}
}

func EncodeRemoteArgs(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) error {
	if m.IsCluster {
		if fi.InArgsLen == 0 || m.Args == nil {
			return nil
		}

		return EncodeArgs(app, fi, 0, m)
	}

	return nil
}

func EncodeLocalArgs(app cfacade.IApplication, fi *creflect.FuncInfo, m *cfacade.Message) error {
	return EncodeArgs(app, fi, 1, m)
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
				clog.Error(err)
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

func retResponse(m *cfacade.Message, rsp *cproto.Response) {
	rspData, _ := proto.Marshal(rsp)

	rspMsg := cnats.GetMsg()
	rspMsg.Header = m.Header
	rspMsg.Subject = m.Reply
	rspMsg.Data = rspData
	// send id = 4, reqID = 21
	clog.Debugf("[retResponse] Sending response: subject = %s, id = %s, reqID = %s, code = %d",
		m.Reply, m.Header.Get("conID"), m.Header.Get("reqID"), int(rsp.Code))

	if err := cnats.GetConnect().PublishMsg(rspMsg); err != nil {
		clog.Warn(err)
	}

	cnats.ReleaseMsg(rspMsg)
	m.Destory()
}
