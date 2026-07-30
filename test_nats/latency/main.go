package main

import (
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
)

type NatsPool struct {
	connections []*nats.Conn
	poolSize    int
	cursor      int
	mu          sync.Mutex
}

func NewNatsPool(url string, size int) (*NatsPool, error) {
	pool := &NatsPool{poolSize: size, connections: make([]*nats.Conn, size)}
	for i := 0; i < size; i++ {
		nc, err := nats.Connect(url)
		if err != nil {
			return nil, err
		}
		pool.connections[i] = nc
	}
	return pool, nil
}

func (p *NatsPool) Get() *nats.Conn {
	p.mu.Lock()
	defer p.mu.Unlock()
	nc := p.connections[p.cursor]
	p.cursor = (p.cursor + 1) % p.poolSize
	return nc
}

func (p *NatsPool) Close() {
	for _, nc := range p.connections {
		if nc != nil {
			nc.Close()
		}
	}
}

func main() {
	urls := nats.DefaultURL // 或者 "nats://10.10.10.251:4222"

	// 1. 初始化客户端发送连接池
	poolSize := 100
	pool, err := NewNatsPool(urls, poolSize)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer pool.Close()

	// 2. 模拟 8 个独立的服务器节点共同承担 slots 游戏核心逻辑
	logicPoolSize := 10
	subscribeCount := 0
	for i := 0; i < logicPoolSize; i++ {
		logicNc, _ := nats.Connect(urls)
		defer logicNc.Close()

		// 【核心修复】：使用 QueueSubscribe 代替普通的 Subscribe
		// 这样 8 个节点就会共享同一个队列分组，一条请求绝对只会被其中一台机器收到并处理
		_, _ = logicNc.QueueSubscribe("test.ping1", "slots-game-workers", func(m *nats.Msg) {
			go func(msg *nats.Msg, conn *nats.Conn) {
				subscribeCount++
				_ = conn.Publish(msg.Reply, []byte("pong"))
			}(m, logicNc)
		})
	}

	time.Sleep(200 * time.Millisecond)

	// 3. 配置模拟参数
	var wg sync.WaitGroup
	totalRequests := 100000
	testDuration := 1 * time.Second
	interval := testDuration / time.Duration(totalRequests) // 1ms 发射一个请求

	latencies := make([]time.Duration, totalRequests)

	fmt.Printf("开始安全压测（已开启队列均衡模式）：%d个请求将在 %v 内均匀发射，每 %v 发射一个...\n", totalRequests, testDuration, interval)

	startTest := time.Now()

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()

			localNc := pool.Get()

			start := time.Now()
			resp, err := localNc.Request("test.ping1", []byte("ping"), 2*time.Second)
			elapsed := time.Since(start)

			if err == nil && string(resp.Data) == "pong" {
				latencies[index] = elapsed
			} else {
				latencies[index] = 999 * time.Second
			}
		}(i)

		// 均匀散开请求
		time.Sleep(interval)
	}

	wg.Wait()
	totalTime := time.Since(startTest)

	// 4. 统计结果
	var totalLatency time.Duration
	successCount := 0
	for _, l := range latencies {
		if l < 5*time.Second {
			totalLatency += l
			successCount++
		}
	}

	if successCount > 0 {
		avgRTT := totalLatency / time.Duration(successCount)
		qps := float64(successCount) / totalTime.Seconds()

		fmt.Println("\n================ 真实模拟报告(队列组版) ================")
		fmt.Printf("接受请求数: %d\n", subscribeCount)
		fmt.Printf("成功收到回复的请求: %d / %d\n", successCount, totalRequests)
		fmt.Printf("整个压测任务总耗时: %v\n", totalTime)
		fmt.Printf("实际运行吞吐量: %.2f QPS\n", qps)
		fmt.Printf("=======> 真实平均 RTT 延迟: %v <=======\n", avgRTT)
		fmt.Println("======================================================")
	}
}
