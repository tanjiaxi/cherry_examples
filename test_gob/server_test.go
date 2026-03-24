package main

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"testing"
)

func main() {

}

type MathService struct {
}

type Args struct {
	A, B int
}

func (m *MathService) Add(args Args, reply *int) error {
	*reply = args.A + args.B
	return nil
}

func TestServerT(t *testing.T) {
	rpc.RegisterName("MathService", new(MathService))
	l, err := net.Listen("tcp", ":8088") //注意 “：” 不要忘了写
	if err != nil {
		log.Fatal("listen error", err)
	}
	rpc.Accept(l)
}

func TestClient(t *testing.T) {
	client, err := rpc.Dial("tcp", "localhost:8088")
	if err != nil {
		log.Fatal("dialing")
	}
	args := Args{A: 1, B: 3}
	var reply int
	err = client.Call("MathService.Add", args, &reply)
	if err != nil {
		log.Fatal("MathService.Add error", err)
	}
	fmt.Printf("MathService.Add: %d+%d=%d", args.A, args.B, reply)
}
func TestServerNet(t *testing.T) {
	connectChan := make(chan net.Conn, 10)
	//接受conn
	go acceptConnect(connectChan)
	l, err := net.Listen("tcp", ":8088") //注意 “：” 不要忘了写
	if err != nil {
		log.Fatal("listen error", err)
	}
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Fatal("accept error", err)
			continue
		}
		connectChan <- conn
	}
	log.Println("over")
}
func accectMsg(conn net.Conn) {
	for {
		buf := make([]byte, 1024)
		n, err := conn.Read(buf)
		if err != nil {
			log.Println("read error", err)
			return
		}
		log.Println(string(buf[:n]))
	}
}
func acceptConnect(connectChan <-chan net.Conn) {
	for conn := range connectChan {
		go accectMsg(conn)
	}
}
func TestClientNet(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:8088")
	if err != nil {
		log.Fatal("dialing", err)
	}
	conn.Write([]byte("hello world"))
	conn.Close()
}
