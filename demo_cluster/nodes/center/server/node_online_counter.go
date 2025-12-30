package server

// import (
// 	"sync"
// )

// NodeOnlineCounter 节点在线人数统计
// 用于负载均衡，选择在线人数最少的节点
type NodeOnlineCounter struct {
	counts map[string]int32 // nodeId -> onlineCount
	// mu     sync.RWMutex
}

// NewNodeOnlineCounter 创建节点在线人数统计器
func NewNodeOnlineCounter() *NodeOnlineCounter {
	return &NodeOnlineCounter{
		counts: make(map[string]int32),
	}
}

// Increment 增加节点在线人数
func (c *NodeOnlineCounter) Increment(nodeId string) {
	if nodeId == "" {
		return
	}

	// c.mu.Lock()
	// defer c.mu.Unlock()

	c.counts[nodeId]++
}

// Decrement 减少节点在线人数
func (c *NodeOnlineCounter) Decrement(nodeId string) {
	if nodeId == "" {
		return
	}

	// c.mu.Lock()
	// defer c.mu.Unlock()

	if count, exists := c.counts[nodeId]; exists && count > 0 {
		c.counts[nodeId]--
	}
}

// GetCount 获取节点在线人数
func (c *NodeOnlineCounter) GetCount(nodeId string) int32 {
	// c.mu.RLock()
	// defer c.mu.RUnlock()

	return c.counts[nodeId]
}

// GetLeastLoadedNode 获取在线人数最少的节点
// nodeType: 节点类型（用于日志）
// nodes: 可选节点列表
func (c *NodeOnlineCounter) GetLeastLoadedNode(nodeType string, nodes []string) string {
	if len(nodes) == 0 {
		return ""
	}

	// c.mu.RLock()
	// defer c.mu.RUnlock()

	var bestNode string
	var minCount int32 = -1

	for _, nodeId := range nodes {
		count := c.counts[nodeId]
		if minCount < 0 || count < minCount {
			minCount = count
			bestNode = nodeId
		}
	}

	// clog.Debugf("[NodeOnlineCounter] 选择最优%s节点: %s (在线人数: %d)",
	// 	nodeType, bestNode, minCount)

	return bestNode
}

// SetCount 设置节点在线人数（用于初始化或同步）
func (c *NodeOnlineCounter) SetCount(nodeId string, count int32) {
	// c.mu.Lock()
	// defer c.mu.Unlock()

	c.counts[nodeId] = count
}

// GetAllCounts 获取所有节点的在线人数
func (c *NodeOnlineCounter) GetAllCounts() map[string]int32 {
	// c.mu.RLock()
	// defer c.mu.RUnlock()

	result := make(map[string]int32, len(c.counts))
	for k, v := range c.counts {
		result[k] = v
	}
	return result
}

// Reset 重置指定节点的计数（节点重启时使用）
func (c *NodeOnlineCounter) Reset(nodeId string) {
	// c.mu.Lock()
	// defer c.mu.Unlock()

	c.counts[nodeId] = 0
}

// Remove 移除节点（节点下线时使用）
func (c *NodeOnlineCounter) Remove(nodeId string) {
	// c.mu.Lock()
	// defer c.mu.Unlock()

	delete(c.counts, nodeId)
}

// GetTotalOnline 获取总在线人数
func (c *NodeOnlineCounter) GetTotalOnline() int32 {
	// c.mu.RLock()
	// defer c.mu.RUnlock()

	var total int32
	for _, count := range c.counts {
		total += count
	}
	return total
}
