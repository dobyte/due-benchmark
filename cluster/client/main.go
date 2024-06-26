package main

import (
	"due-benchmark/cluster/quota"
	"fmt"
	"github.com/dobyte/due/eventbus/nats/v2"
	"github.com/dobyte/due/network/ws/v2"
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
		client.WithClient(ws.NewClient()),
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
	doPressureTest(proxy)
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
func doPressureTest(proxy *client.Proxy) {
	wg = &sync.WaitGroup{}
	message = xrand.Letters(quota.Size)

	atomic.StoreInt64(&totalSent, 0)
	atomic.StoreInt64(&totalRecv, 0)

	wg.Add(quota.Requests)

	chSeq := make(chan int32, quota.Requests)

	// 创建连接
	for i := 0; i < quota.Concurrency; i++ {
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

	for i := 1; i <= quota.Requests; i++ {
		chSeq <- int32(i)
	}

	wg.Wait()

	close(chSeq)

	totalTime := float64(time.Now().UnixNano()-startTime) / float64(time.Second)

	fmt.Printf("server               : %s\n", quota.Protocol)
	fmt.Printf("concurrency          : %d\n", quota.Concurrency)
	fmt.Printf("latency              : %fs\n", totalTime)
	fmt.Printf("data size            : %s\n", quota.ConvBytes(quota.Size))
	fmt.Printf("sent requests        : %d\n", totalSent)
	fmt.Printf("received requests    : %d\n", totalRecv)
	fmt.Printf("throughput (TPS)     : %d\n", int64(float64(totalRecv)/totalTime))
	fmt.Printf("--------------------------------\n")
}
