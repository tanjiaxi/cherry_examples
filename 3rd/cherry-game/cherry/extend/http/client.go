package cherryHttp

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"strings"
	"time"

	clog "github.com/cherry-game/cherry/logger"
)

var (
	postContentType = "application/x-www-form-urlencoded"
	jsonContentType = "application/json"

	DefaultTimeout = 10 * time.Second
)

var globalClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:          1000,
		MaxIdleConnsPerHost:   1000,
		MaxConnsPerHost:       1000, // ← 添加这个，限制对单个host的最大连接数
		IdleConnTimeout:       90 * time.Second,
		DisableKeepAlives:     false, // ← 确保启用Keep-Alive
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second, // ← 新增：等待响应头超时
	},
	Timeout: 10 * time.Second,
}

func GlobalClientGet(httpURL string, values ...map[string]string) ([]byte, *http.Response, error) {
	if len(values) > 0 {
		rst := ToUrlValues(values[0])
		httpURL = AddParams(httpURL, rst)
	}
	rsp, err := globalClient.Get(httpURL)
	if err != nil {
		return nil, rsp, err
	}

	defer func(body io.ReadCloser) {
		e := body.Close()
		if e != nil {
			clog.Warnf("HTTP GET [url = %s], error = %s", httpURL, e)
		}
	}(rsp.Body)

	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rsp, err
	}

	return bodyBytes, rsp, nil
}

// 创建带详细追踪的 GET 请求
func GlobalClientGetWithTrace(httpURL string, values ...map[string]string) ([]byte, *http.Response, error, *TraceInfo) {
	trace := &TraceInfo{}
	if len(values) > 0 {
		rst := ToUrlValues(values[0])
		httpURL = AddParams(httpURL, rst)
	}
	req, err := http.NewRequest("GET", httpURL, nil)
	if err != nil {
		return nil, nil, err, trace
	}

	// 创建 trace
	clientTrace := &httptrace.ClientTrace{
		GetConn: func(hostPort string) {
			trace.GetConnStart = time.Now()
		},
		GotConn: func(info httptrace.GotConnInfo) {
			trace.GotConnTime = time.Now()
			trace.Reused = info.Reused
			trace.WasIdle = info.WasIdle
			trace.IdleTime = info.IdleTime
		},
		DNSStart: func(_ httptrace.DNSStartInfo) {
			trace.DNSStart = time.Now()
		},
		DNSDone: func(_ httptrace.DNSDoneInfo) {
			trace.DNSDone = time.Now()
		},
		ConnectStart: func(_, _ string) {
			trace.ConnectStart = time.Now()
		},
		ConnectDone: func(_, _ string, err error) {
			trace.ConnectDone = time.Now()
		},
		TLSHandshakeStart: func() {
			trace.TLSStart = time.Now()
		},
		TLSHandshakeDone: func(_ tls.ConnectionState, _ error) {
			trace.TLSDone = time.Now()
		},
		WroteRequest: func(_ httptrace.WroteRequestInfo) {
			trace.WroteRequest = time.Now()
		},
		GotFirstResponseByte: func() {
			trace.FirstByte = time.Now()
		},
	}

	ctx := httptrace.WithClientTrace(context.Background(), clientTrace)
	req = req.WithContext(ctx)

	rsp, err := globalClient.Do(req)
	// BUG FIX 2: 先检查err，再defer close
	if err != nil {
		return nil, rsp, err, trace
	}

	defer func(body io.ReadCloser) {
		e := body.Close()
		if e != nil {
			clog.Warnf("HTTP GET [url = %s], error = %s", httpURL, e)
		}
	}(rsp.Body)

	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rsp, err, trace
	}

	return bodyBytes, rsp, nil, trace
}

type TraceInfo struct {
	GetConnStart time.Time
	GotConnTime  time.Time
	DNSStart     time.Time
	DNSDone      time.Time
	ConnectStart time.Time
	ConnectDone  time.Time
	TLSStart     time.Time
	TLSDone      time.Time
	WroteRequest time.Time
	FirstByte    time.Time
	Reused       bool
	WasIdle      bool
	IdleTime     time.Duration
}

func (t *TraceInfo) Print(traceId string) {
	// 计算总耗时
	var totalTime time.Duration
	if !t.GetConnStart.IsZero() && !t.FirstByte.IsZero() {
		totalTime = t.FirstByte.Sub(t.GetConnStart)
	}

	clog.Warnf("========== HTTP Trace [%s] (总耗时: %v) ==========", traceId, totalTime)

	// 1. GetConn等待时间（获取连接）
	if !t.GetConnStart.IsZero() && !t.GotConnTime.IsZero() {
		wait := t.GotConnTime.Sub(t.GetConnStart)
		percent := float64(wait) / float64(totalTime) * 100
		status := "✓"
		if wait > 100*time.Millisecond {
			status = "⚠️ SLOW"
		}
		clog.Warnf("Trace [%s]  [1] GetConn等待:    %8v (%5.1f%%) %s", traceId, wait, percent, status)
	}

	// 2. DNS解析时间
	if !t.DNSStart.IsZero() && !t.DNSDone.IsZero() {
		dns := t.DNSDone.Sub(t.DNSStart)
		percent := float64(dns) / float64(totalTime) * 100
		status := "✓"
		if dns > 50*time.Millisecond {
			status = "⚠️ SLOW"
		}
		clog.Warnf("Trace [%s]  [2] DNS解析:        %8v (%5.1f%%) %s", traceId, dns, percent, status)
	} else if !t.GetConnStart.IsZero() {
		clog.Warnf("Trace [%s]  [2] DNS解析:        已缓存 (连接复用: %v)", traceId, t.Reused)
	}

	// 3. TCP连接建立时间
	if !t.ConnectStart.IsZero() && !t.ConnectDone.IsZero() {
		tcp := t.ConnectDone.Sub(t.ConnectStart)
		percent := float64(tcp) / float64(totalTime) * 100
		status := "✓"
		if tcp > 100*time.Millisecond {
			status = "⚠️ SLOW"
		}
		clog.Warnf("Trace [%s]  [3] TCP连接建立:    %8v (%5.1f%%) %s", traceId, tcp, percent, status)
	} else if t.Reused {
		clog.Warnf("Trace [%s]  [3] TCP连接建立:    复用连接 (空闲: %v)", traceId, t.IdleTime)
	}

	// 4. TLS握手时间（如果是HTTPS）
	if !t.TLSStart.IsZero() && !t.TLSDone.IsZero() {
		tls := t.TLSDone.Sub(t.TLSStart)
		percent := float64(tls) / float64(totalTime) * 100
		status := "✓"
		if tls > 200*time.Millisecond {
			status = "⚠️ SLOW"
		}
		clog.Warnf("Trace [%s]  [4] TLS握手:        %8v (%5.1f%%) %s", traceId, tls, percent, status)
	}

	// 5. 发送请求时间
	if !t.WroteRequest.IsZero() && !t.GotConnTime.IsZero() {
		send := t.WroteRequest.Sub(t.GotConnTime)
		percent := float64(send) / float64(totalTime) * 100
		status := "✓"
		if send > 10*time.Millisecond {
			status = "⚠️ SLOW"
		}
		clog.Warnf("Trace [%s]  [5] 发送请求:       %8v (%5.1f%%) %s", traceId, send, percent, status)
	}

	// 6. 等待响应头时间（网络RTT + 服务器处理）- 修正描述
	if !t.FirstByte.IsZero() && !t.WroteRequest.IsZero() {
		wait := t.FirstByte.Sub(t.WroteRequest)
		percent := float64(wait) / float64(totalTime) * 100
		status := "✓"
		if wait > 1000*time.Millisecond {
			status = "⚠️ SLOW"
		} else if wait > 500*time.Millisecond {
			status = "⚠️"
		}
		// 修正：明确说明包含网络和服务器处理
		clog.Warnf("Trace [%s]  [6] 等待响应头:     %8v (%5.1f%%) %s ← 网络RTT+服务器处理", traceId, wait, percent, status)
	}

	// 连接复用信息
	clog.Warnf("Trace [%s]  连接状态: 复用=%v, 曾空闲=%v, 空闲时长=%v", traceId, t.Reused, t.WasIdle, t.IdleTime)

	// 诊断建议 - 更精确的描述
	if !t.GetConnStart.IsZero() && !t.GotConnTime.IsZero() {
		wait := t.GotConnTime.Sub(t.GetConnStart)
		if wait > 500*time.Millisecond {
			clog.Errorf("Trace [%s]  ❌ 诊断: GetConn等待过长(%v)", traceId, wait)
			clog.Errorf("Trace [%s]     → 可能原因: 1.连接池耗尽 2.MaxConnsPerHost限制 3.所有连接都在使用中", traceId)
		}
	}

	if !t.ConnectStart.IsZero() && !t.ConnectDone.IsZero() {
		tcp := t.ConnectDone.Sub(t.ConnectStart)
		if tcp > 200*time.Millisecond {
			clog.Errorf("Trace [%s]  ❌ 诊断: TCP连接建立慢(%v)", traceId, tcp)
			clog.Errorf("Trace [%s]     → 可能原因: 1.服务器accept队列满 2.网络延迟高 3.服务器负载高", traceId)
		}
	}

	// 修正：更准确地诊断"等待响应头"阶段
	if !t.FirstByte.IsZero() && !t.WroteRequest.IsZero() {
		wait := t.FirstByte.Sub(t.WroteRequest)
		if wait > 1000*time.Millisecond {
			clog.Errorf("Trace [%s]  ❌ 诊断: 等待响应头时间长(%v) - 包含网络+服务器处理", traceId, wait)
			clog.Errorf("Trace [%s]     → 需结合服务器日志判断:", traceId)
			clog.Errorf("Trace [%s]      - 若服务器日志显示处理快(<10ms): 问题在网络层", traceId)
			clog.Errorf("Trace [%s]       - 若服务器日志显示处理慢(>500ms): 问题在服务器性能", traceId)
			clog.Errorf("Trace [%s]       - 若服务器无日志: 请求可能未到达服务器", traceId)
		}
	}

	clog.Warnf("========================================")
}

func GET(httpURL string, values ...map[string]string) ([]byte, *http.Response, error) {
	client := http.Client{Timeout: DefaultTimeout}

	if len(values) > 0 {
		rst := ToUrlValues(values[0])
		httpURL = AddParams(httpURL, rst)
	}

	rsp, err := client.Get(httpURL)
	if err != nil {
		return nil, rsp, err
	}

	defer func(body io.ReadCloser) {
		e := body.Close()
		if e != nil {
			clog.Warnf("HTTP GET [url = %s], error = %s", httpURL, e)
		}
	}(rsp.Body)

	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rsp, err
	}

	return bodyBytes, rsp, nil
}

func POST(httpURL string, values map[string]string) ([]byte, *http.Response, error) {
	client := http.Client{Timeout: DefaultTimeout}

	rst := ToUrlValues(values)
	rsp, err := client.Post(httpURL, postContentType, strings.NewReader(rst.Encode()))
	if err != nil {
		return nil, rsp, err
	}

	defer func(body io.ReadCloser) {
		e := body.Close()
		if e != nil {
			clog.Warnf("HTTP POST [url = %s], error = %s", httpURL, e)
		}
	}(rsp.Body)

	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rsp, err
	}

	return bodyBytes, rsp, nil
}

func PostJSON(httpURL string, values interface{}) ([]byte, *http.Response, error) {
	client := http.Client{Timeout: DefaultTimeout}

	jsonBytes, err := json.Marshal(values)
	if err != nil {
		return nil, nil, err
	}

	rsp, err := client.Post(httpURL, jsonContentType, bytes.NewBuffer(jsonBytes))
	if err != nil {
		return nil, rsp, err
	}

	defer func(body io.ReadCloser) {
		e := body.Close()
		if e != nil {
			clog.Warnf("HTTP PostJSON [url = %s], error = %s", httpURL, e)
		}
	}(rsp.Body)

	bodyBytes, err := io.ReadAll(rsp.Body)
	if err != nil {
		return nil, rsp, err
	}

	return bodyBytes, rsp, nil
}

func AddParams(httpURL string, params url.Values) string {
	if len(params) == 0 {
		return httpURL
	}

	if !strings.Contains(httpURL, "?") {
		httpURL += "?"
	}

	if strings.HasSuffix(httpURL, "?") || strings.HasSuffix(httpURL, "&") {
		httpURL += params.Encode()
	} else {
		httpURL += "&" + params.Encode()
	}

	return httpURL
}

func ToUrlValues(values map[string]string) url.Values {
	rst := make(url.Values)
	for k, v := range values {
		rst.Add(k, v)
	}
	return rst
}
