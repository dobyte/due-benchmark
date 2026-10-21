package main

import (
	"flag"
	"fmt"

	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/registry/etcd/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/cluster/node"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/packet"
)

const echoRoute int32 = 1

// rawCodec keeps the common opaque byte payload independent of JSON/protobuf.
type rawCodec struct{}

func (rawCodec) Name() string { return "benchmark-raw" }
func (rawCodec) Marshal(value any) ([]byte, error) {
	payload, ok := value.([]byte)
	if !ok {
		return nil, fmt.Errorf("expected []byte")
	}
	return payload, nil
}
func (rawCodec) Unmarshal(data []byte, value any) error {
	target, ok := value.(*[]byte)
	if !ok {
		return fmt.Errorf("expected *[]byte")
	}
	*target = append((*target)[:0], data...)
	return nil
}

func main() {
	role := flag.String("role", "worker", "gateway or worker")
	flag.Parse()
	packet.SetPacker(packet.NewPacker(packet.WithRouteBytes(2), packet.WithSeqBytes(2), packet.WithBufferBytes(1<<20)))
	registry := etcd.NewRegistry(etcd.WithAddrs("127.0.0.1:17379"), etcd.WithNamespace("framework-benchmark/due"))
	locator := redis.NewLocator(redis.WithAddrs("127.0.0.1:17389"), redis.WithPrefix("framework-benchmark:due"))
	container := due.NewContainer()
	if *role == "gateway" {
		server := tcp.NewServer(tcp.WithServerAddr("127.0.0.1:17001"), tcp.WithServerHeartbeatInterval(0), tcp.WithServerMaxConnNum(10000))
		container.Add(gate.NewGate(gate.WithID("benchmark-gateway"), gate.WithServer(server), gate.WithRegistry(registry), gate.WithLocator(locator), gate.WithAddr("127.0.0.1:17003")))
	} else if *role == "worker" {
		component := node.NewNode(node.WithID("benchmark-worker"), node.WithName("benchmark-worker"), node.WithRegistry(registry), node.WithLocator(locator), node.WithCodec(rawCodec{}), node.WithAddr("127.0.0.1:17002"))
		component.Proxy().AddRouteHandler(echoRoute, func(ctx node.Context) {
			var payload []byte
			if err := ctx.Parse(&payload); err != nil {
				log.Error(err)
				return
			}
			if err := ctx.Response(payload); err != nil {
				log.Error(err)
			}
		})
		container.Add(component)
	} else {
		panic("unknown role")
	}
	container.Serve()
}
