package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	// 1. 打印当前进程真实的 PID，解决 Mac 上 go run 的中间商问题
	fmt.Printf("=== 程序已启动 ===\n")
	fmt.Printf("当前程序真实的 PID 是: %d\n", os.Getpid())
	fmt.Printf("你可以打开新终端执行: kill -15 %d\n", os.Getpid())
	fmt.Printf("或者在当前终端直接按: Ctrl + C\n\n")

	// 模拟你代码中的 dieChan，正常情况下它不会被关闭
	dieChan := make(chan struct{})

	// 2. 监听系统关闭信号
	sg := make(chan os.Signal, 1)
	signal.Notify(sg, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM)

	// 3. 阻塞等待信号
	select {
	case <-dieChan:
		fmt.Println("【日志】invoke shutdown().")
	case s := <-sg:
		fmt.Printf("【日志】receive shutdown signal = %v.\n", s)
	}

	// 4. 模拟释放资源（如关闭数据库、等待未完成的请求）
	fmt.Println("正在清理资源并退出程序...")
	time.Sleep(1 * time.Millisecond) // 留出 1 秒查看效果
	fmt.Println("=== 程序已安全退出 ===")
}
