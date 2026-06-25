/*
 * @Author: t 921865806@qq.com
 * @Date: 2025-12-21 18:55:52
 * @LastEditors: t 921865806@qq.com
 * @LastEditTime: 2026-06-23 16:27:17
 * @FilePath: /examples/demo_cluster/internal/component/heartbeat/component.go
 * @Description: 这是默认设置,请设置`customMade`, 打开koroFileHeader查看配置 进行设置: https://github.com/OBKoro1/koro1FileHeader/wiki/%E9%85%8D%E7%BD%AE
 */
package heartbeat

import (
	"time"

	cfacade "github.com/cherry-game/cherry/facade"
	clog "github.com/cherry-game/cherry/logger"
	"github.com/cherry-game/examples/demo_cluster/internal/code"
	rpcCenter "github.com/cherry-game/examples/demo_cluster/internal/rpc/center"
)

// Component 心跳组件
// 定时向Center发送心跳，用于健康检测
type Component struct {
	cfacade.Component
	nodeType string
	interval time.Duration
	stopCh   chan struct{}
}

// New 创建心跳组件
func New(nodeType string) *Component {
	return &Component{
		nodeType: nodeType,
		interval: 3 * time.Second, // 每3秒发送一次心跳
		stopCh:   make(chan struct{}),
	}
}

func (c *Component) Name() string {
	return "heartbeat_component"
}

func (c *Component) Init() {
}

func (c *Component) OnAfterInit() {
	// 启动心跳goroutine
	go c.heartbeatLoop()
	clog.Infof("[HeartbeatComponent] 心跳组件启动: nodeType=%s, interval=%v", c.nodeType, c.interval)
}

func (c *Component) OnStop() {
	close(c.stopCh)
	clog.Info("[HeartbeatComponent] 心跳组件停止")
}

func (c *Component) heartbeatLoop() {
	// 等待一小段时间，确保其他组件初始化完成
	time.Sleep(2 * time.Second)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	// 立即发送一次心跳
	c.sendHeartbeat()

	for {
		select {
		case <-ticker.C:
			c.sendHeartbeat()
		case <-c.stopCh:
			return
		}
	}
}

func (c *Component) sendHeartbeat() {
	nodeId := c.App().NodeID()
	errCode := rpcCenter.Heartbeat(c.App(), nodeId, c.nodeType, "")
	if code.IsFail(errCode) {
		clog.Warnf("[HeartbeatComponent] 发送心跳失败: nodeId=%s, errCode=%d", nodeId, errCode)
	}
}
