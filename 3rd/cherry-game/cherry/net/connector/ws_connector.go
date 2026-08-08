package cherryConnector

import (
	"context"
	"io"
	"net/http"
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/gorilla/websocket"
)

type (
	WSConnector struct {
		cfacade.Component
		Connector
		Options
		upgrade    *websocket.Upgrader
		httpServer *http.Server
	}

	// WSConn is an adapter to t.INetConn, which implements all INetConn
	// interface base on *websocket.INetConn
	WSConn struct {
		*websocket.Conn
		typ    int // message type
		reader io.Reader
	}
)

func (*WSConnector) Name() string {
	return "websocket_connector"
}

func (w *WSConnector) OnAfterInit() {
}

func (w *WSConnector) OnStop() {
	w.Stop()
}

func NewWS(address string, opts ...Option) *WSConnector {
	if address == "" {
		clog.Warn("create websocket fail. address is null.")
		return nil
	}

	ws := &WSConnector{
		Options: Options{
			address:           address,
			certFile:          "",
			keyFile:           "",
			chanSize:          256,
			readHeaderTimeout: 5 * time.Second,
			readTimeout:       10 * time.Second,
			writeTimeout:      10 * time.Second,
			idleTimeout:       60 * time.Second,
		},
		upgrade: &websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			// 游戏客户端多为原生 App/非浏览器；若对浏览器开放，请用 SetCheckOrigin 收紧来源校验，
			// 否则任意站点可借用户浏览器连到你的 WS（CSRF 类风险）。
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
	}

	for _, opt := range opts {
		opt(&ws.Options)
	}

	ws.Connector = NewConnector(ws.chanSize)

	return ws
}

func (w *WSConnector) Start() {
	listener, err := w.GetListener(w.certFile, w.keyFile, w.address)
	if err != nil {
		clog.Fatalf("failed to listen: %s", err)
	}

	clog.Infof("Websocket connector listening at Address %s", w.address)
	if w.certFile != "" || w.keyFile != "" {
		clog.Infof("certFile = %s, keyFile = %s", w.certFile, w.keyFile)
	}

	w.Connector.Start()

	// 使用显式 http.Server，而不是 http.Serve(listener, handler)：
	// 后者超时全是 0，恶意客户端可以慢速滴灌请求头占满连接（Slowloris）。
	// ReadHeaderTimeout 专门限制“读完 HTTP 头”的时间，是 WS 握手前最关键的防护。
	// Read/WriteTimeout 只约束握手阶段；gorilla Upgrade 成功后会清空 conn deadline，
	// 不会把长连接 WebSocket 误杀。升级后的空闲检测应靠业务层 SetReadDeadline + Ping/Pong。
	w.httpServer = &http.Server{
		Handler:           w,
		ReadHeaderTimeout: w.readHeaderTimeout,
		ReadTimeout:       w.readTimeout,
		WriteTimeout:      w.writeTimeout,
		IdleTimeout:       w.idleTimeout,
	}

	if err := w.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
		clog.Errorf("Websocket connector serve error: %s", err)
	}
}

func (w *WSConnector) Stop() {
	if w.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := w.httpServer.Shutdown(ctx); err != nil {
			clog.Warnf("Websocket connector shutdown error: %s", err)
			_ = w.httpServer.Close()
		}
	}
	w.Connector.Stop()
}

func (w *WSConnector) SetUpgrade(upgrade *websocket.Upgrader) {
	if upgrade != nil {
		w.upgrade = upgrade
	}
}

// SetCheckOrigin 设置浏览器 Origin 校验；对网页客户端开放时不应无条件 return true。
func (w *WSConnector) SetCheckOrigin(fn func(*http.Request) bool) {
	if fn != nil && w.upgrade != nil {
		w.upgrade.CheckOrigin = fn
	}
}

func (w *WSConnector) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	wsConn, err := w.upgrade.Upgrade(rw, r, nil)
	if err != nil {
		clog.Infof("Upgrade failure, URI=%s, Error=%s", r.RequestURI, err.Error())
		return
	}

	conn := NewWSConn(wsConn)
	w.InChan(&conn)
}

// NewWSConn return an initialized *WSConn
func NewWSConn(conn *websocket.Conn) WSConn {
	c := WSConn{
		Conn: conn,
	}
	return c
}

func (c *WSConn) Read(b []byte) (int, error) {
	if c.reader == nil {
		t, r, err := c.NextReader()
		if err != nil {
			return 0, err
		}
		c.typ = t
		c.reader = r
	}
	n, err := c.reader.Read(b)
	if err != nil && err != io.EOF {
		return n, err
	} else if err == io.EOF {
		_, r, err := c.NextReader()
		if err != nil {
			return 0, err
		}
		c.reader = r
	}

	return n, nil
}

func (c *WSConn) Write(b []byte) (int, error) {
	err := c.WriteMessage(websocket.BinaryMessage, b)
	if err != nil {
		return 0, err
	}

	return len(b), nil
}

func (c *WSConn) SetDeadline(t time.Time) error {
	if err := c.SetReadDeadline(t); err != nil {
		return err
	}

	return c.SetWriteDeadline(t)
}
