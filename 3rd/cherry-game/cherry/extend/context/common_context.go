package cherryContxt

import "context"

type CommonContext struct {
	TraceId string //链路追踪Id,在cherry中应该就是mid
	UserId  string //用户id
}

const contextKey string = "common_context"

func FromContext(ctx context.Context) *CommonContext {
	if ctx == nil {
		return nil
	}
	if logCtx, ok := ctx.Value(contextKey).(*CommonContext); ok {
		return logCtx
	}
	return nil
}
func GetTraceId(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceId := ""
	rawData := FromContext(ctx)
	if rawData != nil {
		traceId = rawData.TraceId
	}
	return traceId
}
func WithContext(ctx context.Context, commonCtx *CommonContext) context.Context {
	if ctx == nil || commonCtx == nil {
		return ctx
	}
	return context.WithValue(ctx, contextKey, commonCtx)
}
