package main

import (
	"due-benchmark/cluster/quota"
	"fmt"
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/registry/consul/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xtime"
	"sync"
	"sync/atomic"
	"time"
)

const greet = 1

func main() {
	// 创建容器
	container := due.NewContainer()
	// 创建用户定位器
	locator := redis.NewLocator()
	// 创建服务发现
	registry := consul.NewRegistry()
	// 创建节点组件
	component := node.NewNode(
		node.WithLocator(locator),
		node.WithRegistry(registry),
	)
	// 初始化监听
	initListen(component.Proxy())
	// 添加节点组件
	container.Add(component)
	// 启动容器
	container.Serve()
}

// 初始化监听
func initListen(proxy *node.Proxy) {
	proxy.Router().AddRouteHandler(greet, false, greetHandler)
}

type greetReq struct {
	Message string `json:"message"`
}

type greetRes struct {
	Message string `json:"message"`
}

var reqPool = sync.Pool{New: func() any {
	return &greetReq{}
}}

var resPool = sync.Pool{New: func() any {
	return &greetRes{}
}}

func greetHandler(ctx node.Context) {
	ctx = ctx.Clone()

	//task.AddTask(func() {
	req := reqPool.Get().(*greetReq)
	res := resPool.Get().(*greetRes)
	defer reqPool.Put(req)
	defer resPool.Put(res)
	defer func() {
		if err := ctx.Response(res); err != nil {
			log.Errorf("response message failed: %v", err)
		}
	}()

	if err := ctx.Parse(req); err != nil {
		log.Errorf("parse request message failed: %v", err)
		return
	}

	res.Message = req.Message
	//})
}

var (
	startTime int64
	totalRecv int64
)

// TPS数据数据分析
func analyze() {
	switch atomic.AddInt64(&totalRecv, 1) {
	case 1:
		startTime = xtime.Now().UnixNano()
	case quota.Requests:
		totalTime := float64(time.Now().UnixNano()-startTime) / float64(time.Second)

		fmt.Printf("server               : %s\n", quota.Protocol)
		fmt.Printf("concurrency          : %d\n", quota.Concurrency)
		fmt.Printf("latency              : %fs\n", totalTime)
		fmt.Printf("data size            : %s\n", quota.ConvBytes(quota.Size))
		fmt.Printf("received requests    : %d\n", totalRecv)
		fmt.Printf("throughput (TPS)     : %d\n", int64(float64(totalRecv)/totalTime))
		fmt.Printf("--------------------------------\n")

		atomic.StoreInt64(&totalRecv, 0)
	}
}
