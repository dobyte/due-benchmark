package main

import (
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/registry/consul/v2"
	"github.com/dobyte/due/transport/rpcx/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/utils/xcall"
)

const greet = 1

func main() {
	// 创建容器
	container := due.NewContainer()
	// 创建用户定位器
	locator := redis.NewLocator()
	// 创建服务发现
	registry := consul.NewRegistry()
	// 创建RPC传输器
	transporter := rpcx.NewTransporter()
	// 创建节点组件
	component := node.NewNode(
		node.WithLocator(locator),
		node.WithRegistry(registry),
		node.WithTransporter(transporter),
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

var (
	totalRecv int64
	totalSent int64
)

func greetHandler(ctx node.Context) {
	ctx = ctx.Clone()

	xcall.Go(func() {
		req := &greetReq{}
		res := &greetRes{}
		defer func() {
			if err := ctx.Response(res); err != nil {
				log.Errorf("response message failed: %v", err)
			}

			//v := atomic.AddInt64(&totalSent, 1)
			//
			////if v > 999999 {
			//fmt.Println("total send: ", v)
			////}
		}()

		if err := ctx.Parse(req); err != nil {
			log.Errorf("parse request message failed: %v", err)
			return
		}

		res.Message = req.Message
	})
}
