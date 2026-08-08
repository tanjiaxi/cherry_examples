package server

import (
	"sync"
	"time"

	clog "github.com/cherry-game/cherry/logger"
)

// NodeHealthChecker 节点健康检测器
// 通过心跳检测节点是否健康
//
// 并发模型：UpdateHeartbeat 由各连接/RPC goroutine 高频并发写入，
// GetUnhealthyNodes/IsHealthy 等由健康检查定时器goroutine及查询方并发读取。
// heartbeats 是裸 map，Go map 不是并发安全的：并发读写会被 runtime 的
// map 写屏障检测到并直接 fatal panic（"concurrent map read and map write"），
// 这是进程级别的崩溃，不是可以recover的普通panic，line 34/42等处原先注释掉的
// mu 锁必须补回。
type NodeHealthChecker struct {
	heartbeats map[string]int64 // nodeId -> lastHeartbeat (unix timestamp)
	mu         sync.RWMutex     // 保护 heartbeats：读多写多，用 RWMutex 而非 Mutex 以降低读路径的争用
	timeout    int64            // 心跳超时时间（秒）
}

// NewNodeHealthChecker 创建节点健康检测器
func NewNodeHealthChecker(timeoutSeconds int64) *NodeHealthChecker {
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10 // 默认10秒超时
	}
	return &NodeHealthChecker{
		heartbeats: make(map[string]int64),
		timeout:    timeoutSeconds,
	}
}

// UpdateHeartbeat 更新节点心跳
func (c *NodeHealthChecker) UpdateHeartbeat(nodeId string) {
	if nodeId == "" {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.heartbeats[nodeId] = time.Now().Unix()
}

// IsHealthy 检查节点是否健康
func (c *NodeHealthChecker) IsHealthy(nodeId string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lastHeartbeat, exists := c.heartbeats[nodeId]
	if !exists {
		return false
	}

	return (time.Now().Unix() - lastHeartbeat) <= c.timeout
}

// GetUnhealthyNodes 获取所有不健康的节点
func (c *NodeHealthChecker) GetUnhealthyNodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now().Unix()
	var unhealthy []string

	for nodeId, lastHeartbeat := range c.heartbeats {
		if (now - lastHeartbeat) > c.timeout {
			unhealthy = append(unhealthy, nodeId)
		}
	}

	return unhealthy
}

// GetHealthyNodes 获取所有健康的节点
func (c *NodeHealthChecker) GetHealthyNodes() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now().Unix()
	var healthy []string

	for nodeId, lastHeartbeat := range c.heartbeats {
		if (now - lastHeartbeat) <= c.timeout {
			healthy = append(healthy, nodeId)
		}
	}

	return healthy
}

// FilterHealthyNodes 从给定列表中过滤出健康的节点
func (c *NodeHealthChecker) FilterHealthyNodes(nodes []string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now().Unix()
	var healthy []string

	for _, nodeId := range nodes {
		if lastHeartbeat, exists := c.heartbeats[nodeId]; exists {
			if (now - lastHeartbeat) <= c.timeout {
				healthy = append(healthy, nodeId)
			}
		}
	}

	return healthy
}

// RemoveNode 移除节点（节点下线时使用）
func (c *NodeHealthChecker) RemoveNode(nodeId string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.heartbeats, nodeId)
}

// GetLastHeartbeat 获取节点最后心跳时间
func (c *NodeHealthChecker) GetLastHeartbeat(nodeId string) (int64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	lastHeartbeat, exists := c.heartbeats[nodeId]
	return lastHeartbeat, exists
}

// GetAllHeartbeats 获取所有节点的心跳信息
func (c *NodeHealthChecker) GetAllHeartbeats() map[string]int64 {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]int64, len(c.heartbeats))
	for k, v := range c.heartbeats {
		result[k] = v
	}
	return result
}

// StartHealthCheck 启动定时健康检查
// 返回一个停止函数
func (c *NodeHealthChecker) StartHealthCheck(
	checkInterval time.Duration,
	onUnhealthyNode func(nodeId string),
) func() {
	ticker := time.NewTicker(checkInterval)
	stopCh := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				unhealthyNodes := c.GetUnhealthyNodes()
				for _, nodeId := range unhealthyNodes {
					clog.Warnf("[NodeHealthChecker] 检测到不健康节点: %s", nodeId)
					if onUnhealthyNode != nil {
						onUnhealthyNode(nodeId)
					}
				}
			case <-stopCh:
				ticker.Stop()
				return
			}
		}
	}()

	return func() {
		close(stopCh)
	}
}

// SetTimeout 设置超时时间
func (c *NodeHealthChecker) SetTimeout(timeoutSeconds int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.timeout = timeoutSeconds
}
