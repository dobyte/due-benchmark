package main

import (
	"fmt"
	"github.com/dobyte/due/eventbus/nats/v2"
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/cluster/client"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xrand"
	"github.com/dobyte/due/v2/utils/xtime"
	"sync"
	"sync/atomic"
	"time"
)

const greet = 1

var (
	wg        *sync.WaitGroup
	startTime int64
	totalSent int64
	totalRecv int64
	message   string
)

type greetReq struct {
	Message string `json:"message"`
}

type greetRes struct {
	Message string `json:"message"`
}

func main() {
	// 初始化事件总线
	eventbus.SetEventbus(nats.NewEventbus())
	// 创建容器
	container := due.NewContainer()
	// 创建客户端组件
	component := client.NewClient(
		client.WithClient(tcp.NewClient()),
	)
	// 初始化监听
	initListen(component.Proxy())
	// 添加客户端组件
	container.Add(component)
	// 启动容器
	container.Serve()
}

// 初始化监听
func initListen(proxy *client.Proxy) {
	// 监听组件启动
	proxy.AddHookListener(cluster.Start, startHandler)
	// 监听消息回复
	proxy.AddRouteHandler(greet, greetHandler)
}

// 组件启动处理器
func startHandler(proxy *client.Proxy) {
	doPressureTest(proxy, 500, 1000000, 512)
}

// 消息回复处理器
func greetHandler(ctx *client.Context) {
	res := &greetRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("invalid response message, err: %v", err)
		return
	}

	atomic.AddInt64(&totalRecv, 1)

	wg.Done()
}

// 执行压力测试
func doPressureTest(proxy *client.Proxy, c int, n int, size int) {
	wg = &sync.WaitGroup{}
	message = xrand.Letters(size)

	atomic.StoreInt64(&totalSent, 0)
	atomic.StoreInt64(&totalRecv, 0)

	wg.Add(n)

	chSeq := make(chan int32, n)

	// 创建连接
	for i := 0; i < c; i++ {
		conn, err := proxy.Dial()
		if err != nil {
			log.Errorf("gate connect failed: %v", err)
			return
		}

		go func(conn *client.Conn) {
			defer func() {
				_ = conn.Close()
			}()

			for {
				select {
				case seq, ok := <-chSeq:
					if !ok {
						return
					}

					err := conn.Push(&cluster.Message{
						Route: 1,
						Seq:   seq,
						Data:  &greetReq{Message: message},
					})
					if err != nil {
						log.Errorf("push message failed: %v", err)
						return
					}

					atomic.AddInt64(&totalSent, 1)
				}
			}
		}(conn)
	}

	startTime = xtime.Now().UnixNano()

	for i := 1; i <= n; i++ {
		chSeq <- int32(i)
	}

	wg.Wait()

	close(chSeq)

	totalTime := float64(time.Now().UnixNano()-startTime) / float64(time.Second)

	fmt.Printf("server               : %s\n", "tcp")
	fmt.Printf("concurrency          : %d\n", c)
	fmt.Printf("latency              : %fs\n", totalTime)
	fmt.Printf("data size            : %s\n", convBytes(size))
	fmt.Printf("sent requests        : %d\n", totalSent)
	fmt.Printf("received requests    : %d\n", totalRecv)
	fmt.Printf("throughput (TPS)     : %d\n", int64(float64(totalRecv)/totalTime))
	fmt.Printf("--------------------------------\n")
}

func convBytes(bytes int) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes < KB:
		return fmt.Sprintf("%.2fB", float64(bytes))
	case bytes < MB:
		return fmt.Sprintf("%.2fKB", float64(bytes)/KB)
	case bytes < GB:
		return fmt.Sprintf("%.2fMB", float64(bytes)/MB)
	case bytes < TB:
		return fmt.Sprintf("%.2fGB", float64(bytes)/GB)
	default:
		return fmt.Sprintf("%.2fTB", float64(bytes)/TB)
	}
}
