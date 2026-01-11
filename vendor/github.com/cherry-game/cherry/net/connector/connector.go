/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-26 17:52:15
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-01-09 18:05:04
 * @FilePath: /examples/vendor/github.com/cherry-game/cherry/net/connector/connector.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package cherryConnector

import (
	"crypto/tls"
	"net"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
)

type (
	Connector struct {
		listener      net.Listener
		onConnectFunc cfacade.OnConnectFunc
		connChan      chan net.Conn
		running       bool
	}
)

func NewConnector(size int) Connector {
	connector := Connector{
		connChan: make(chan net.Conn, size),
		running:  true,
	}
	return connector
}

func (p *Connector) OnConnect(fn cfacade.OnConnectFunc) {
	if fn != nil {
		p.onConnectFunc = fn
	}
}

func (p *Connector) InChan(conn net.Conn) {
	p.connChan <- conn
}

func (p *Connector) Start() {
	if p.onConnectFunc == nil {
		panic("onConnectFunc is nil.")
	}

	go func() {
		for conn := range p.connChan {
			p.onConnectFunc(conn)
		}
	}()
}

func (p *Connector) Stop() {
	p.running = false

	if err := p.listener.Close(); err != nil {
		clog.Errorf("Failed to stop: %s", err)
	}
}

func (p *Connector) Running() bool {
	return p.running
}

func (p *Connector) GetListener(certFile, keyFile, address string) (net.Listener, error) {
	var err error
	if certFile == "" || keyFile == "" {
		p.listener, err = net.Listen("tcp", address)
		return p.listener, err
	}

	crt, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		clog.Fatalf("failed to listen: %s", err.Error())
	}

	tlsCfg := &tls.Config{
		Certificates: []tls.Certificate{crt},
	}

	p.listener, err = tls.Listen("tcp", address, tlsCfg)
	return p.listener, err
}
