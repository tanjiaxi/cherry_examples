package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func main() {
	createStream()
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("连接 NATS 失败: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("获取 JetStream 上下文失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 目标主题，它匹配我们 Stream 中定义的 "ORDERS.*"
	subject := "ORDERS.new"
	payload := []byte(`{"order_id": 457, "item": "Golang Book"}`)

	// 使用 js.Publish 而不是 nc.Publish
	// 它会等待服务器确认消息已被持久化，并返回 PubAck
	ack, err := js.Publish(ctx, subject, payload)
	if err != nil {
		log.Fatalf("发布到 JetStream 失败: %v", err)
	}

	fmt.Printf("消息已成功发布并持久化。Stream: %s, Seq: %d\n", ack.Stream, ack.Sequence)
}

func createStream() {
	// 1. 连接到 NATS 服务器
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("连接 NATS 失败: %v", err)
	}
	defer nc.Close()

	// 2. 创建 JetStream 上下文，这是所有 JetStream 操作的入口
	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("获取 JetStream 上下文失败: %v", err)
	}

	// 使用 context 以设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 3. 定义 Stream 的配置
	cfg := jetstream.StreamConfig{
		Name:     "ORDERS_STREAM",
		Subjects: []string{"ORDERS.*"},  // 捕获所有以 "ORDERS." 开头的主题
		Storage:  jetstream.FileStorage, // 存储类型 (FileStorage 或 MemoryStorage)
	}

	// 4. 创建（或更新）Stream
	// 这个动作是幂等的，如果流已存在且配置相同，则不会重复创建
	stream, err := js.CreateStream(ctx, cfg)
	if err != nil {
		// 如果流已存在，会返回错误，我们可以检查并处理
		stream, _ = js.Stream(ctx, "ORDERS_STREAM")
		log.Printf("流已存在，获取流信息...")
	}

	log.Printf("成功创建或获取了流: %s", stream.CachedInfo().Config.Name)
}
