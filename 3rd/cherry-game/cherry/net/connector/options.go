package cherryConnector

import (
	"time"

	clog "github.com/cherry-game/cherry/logger"
)

type (
	Options struct {
		address           string
		certFile          string
		keyFile           string
		chanSize          int
		readHeaderTimeout time.Duration
		readTimeout       time.Duration
		writeTimeout      time.Duration
		idleTimeout       time.Duration
	}

	Option func(*Options)
)

func WithCert(certFile, keyFile string) Option {
	return func(o *Options) {
		if certFile != "" && keyFile != "" {
			o.certFile = certFile
			o.keyFile = keyFile
		} else {
			clog.Errorf("Cert config error.[cert = %s,key = %s]", certFile, keyFile)
		}
	}
}

func WithChanSize(size int) Option {
	return func(o *Options) {
		if size > 1 {
			o.chanSize = size
		}
	}
}

// WithHTTPTimeouts 配置 WS 握手阶段的 http.Server 超时。
// 传 0 表示保持当前值不变；升级成功后的长连接空闲检测不在这里控制。
func WithHTTPTimeouts(readHeader, read, write, idle time.Duration) Option {
	return func(o *Options) {
		if readHeader > 0 {
			o.readHeaderTimeout = readHeader
		}
		if read > 0 {
			o.readTimeout = read
		}
		if write > 0 {
			o.writeTimeout = write
		}
		if idle > 0 {
			o.idleTimeout = idle
		}
	}
}
