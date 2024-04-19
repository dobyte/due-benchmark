package main

import (
	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/registry/consul/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/component/pprof"
	"github.com/dobyte/due/v2/transport/drpc"
)

func main() {
	// 创建容器
	container := due.NewContainer()
	// 创建服务器
	server := tcp.NewServer()
	// 创建用户定位器
	locator := redis.NewLocator()
	// 创建服务发现
	registry := consul.NewRegistry()
	// 创建RPC传输器
	transporter := drpc.NewTransporter()
	// 创建网关组件
	component := gate.NewGate(
		gate.WithServer(server),
		gate.WithLocator(locator),
		gate.WithRegistry(registry),
		gate.WithTransporter(transporter),
	)
	// 添加网关组件
	container.Add(component, pprof.NewPProf())
	// 启动容器
	container.Serve()
}
