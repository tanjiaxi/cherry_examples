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
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatalf("连接 NATS 失败: %v", err)
	}
	defer nc.Close()

	js, err := jetstream.New(nc)
	if err != nil {
		log.Fatalf("获取 JetStream 上下文失败: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. 获取流的句柄
	stream, err := js.Stream(ctx, "ORDERS_STREAM")
	if err != nil {
		log.Fatalf("获取流失败: %v", err)
	}

	// 2. 创建或获取一个持久化的 Pull Consumer
	// "order-processor-go" 是消费者的名字，NATS 会为它保存状态
	consumer, err := stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable: "order-processor-go",
	})
	if err != nil {
		log.Fatalf("创建消费者失败: %v", err)
	}

	fmt.Println("正在等待新消息...")

	// 3. 从消费者那里拉取一批消息 (这里是1条)，并设置超时
	msgs, err := consumer.Fetch(5, jetstream.FetchMaxWait(100*time.Second))
	if err != nil {
		if err == nats.ErrTimeout {
			log.Println("在10秒内没有新消息。")
			return
		}
		log.Fatalf("拉取消息失败: %v", err)
	}

	// 4. 遍历并处理消息
	for msg := range msgs.Messages() {
		fmt.Printf("收到消息: Subject='%s', Data='%s'\n", msg.Subject(), string(msg.Data()))

		// 5. 关键一步：处理完后，必须手动确认(ack)消息
		// 这样 JetStream 才知道这条消息被成功处理，不会再重复投递
		err := msg.Ack()
		if err != nil {
			log.Printf("确认消息失败: %v", err)
		}
	}
}
