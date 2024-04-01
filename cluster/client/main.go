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
	"github.com/dobyte/due/v2/utils/xtime"
	"sync"
	"sync/atomic"
	"time"
)

const greet = 1

var (
	wg        sync.WaitGroup
	startTime int64
	totalSent int64
	totalRecv int64
	message   = fmt.Sprintf("I'm client, and the current time is: %s", xtime.Now().Format(xtime.DatetimeLayout))
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
	doPressureTest(proxy, 100, 10000, 1024)
}

// 消息回复处理器
func greetHandler(ctx *client.Context) {
	res := &greetRes{}

	if err := ctx.Parse(res); err != nil {
		log.Errorf("invalid response message, err: %v", err)
		return
	}

	total := atomic.AddInt64(&totalRecv, 1)

	if total%1000 == 0 {
		fmt.Println("total recv: ", total)
	}

	wg.Done()
}

// 执行压力测试
func doPressureTest(proxy *client.Proxy, c int, n int, size int) {
	wg = sync.WaitGroup{}

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

					total := atomic.AddInt64(&totalSent, 1)

					if total%1000 == 0 {
						fmt.Println("total sent: ", total)
					}
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
	fmt.Printf("data size            : %dkb\n", size/1024)
	fmt.Printf("sent requests        : %d\n", totalSent)
	fmt.Printf("received requests    : %d\n", totalRecv)
	fmt.Printf("throughput (TPS)     : %d\n", int64(float64(totalRecv)/totalTime))
	fmt.Printf("--------------------------------\n")
}
