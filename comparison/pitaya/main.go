package main

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"

	"github.com/sirupsen/logrus"
	pitaya "github.com/topfreegames/pitaya/v3/pkg"
	"github.com/topfreegames/pitaya/v3/pkg/acceptor"
	"github.com/topfreegames/pitaya/v3/pkg/cluster"
	"github.com/topfreegames/pitaya/v3/pkg/component"
	"github.com/topfreegames/pitaya/v3/pkg/config"
	"github.com/topfreegames/pitaya/v3/pkg/constants"
	"github.com/topfreegames/pitaya/v3/pkg/logger"
	logruswrapper "github.com/topfreegames/pitaya/v3/pkg/logger/logrus"
	"github.com/topfreegames/pitaya/v3/pkg/modules"
)

type Echo struct{ component.Base }

// Echo is registered only on the backend. Pitaya routes worker.echo.echo
// from the TCP frontend across its native gRPC transport to this process.
func (e *Echo) Echo(ctx context.Context, payload []byte) ([]byte, error) {
	return append([]byte(nil), payload...), nil
}

func main() {
	frontend := flag.Bool("frontend", false, "run TCP frontend instead of backend")
	port := flag.Int("rpc-port", 17002, "gRPC listener port")
	flag.Parse()
	logrus.SetLevel(logrus.ErrorLevel)
	frameworkLogger := logrus.New()
	frameworkLogger.SetLevel(logrus.ErrorLevel)
	logger.SetLogger(logruswrapper.NewWithFieldLogger(frameworkLogger))
	conf := config.NewDefaultPitayaConfig()
	conf.Handler.Messages.Compression = false
	conf.Conn.RateLimiting.ForceDisable = true
	conf.Cluster.SD.Etcd.Endpoints = []string{"127.0.0.1:17379"}
	conf.Cluster.SD.Etcd.Prefix = "framework-benchmark/pitaya/"
	conf.Modules.BindingStorage.Etcd.Endpoints = []string{"127.0.0.1:17379"}
	conf.Modules.BindingStorage.Etcd.Prefix = "framework-benchmark/pitaya-bindings/"
	serverType := "worker"
	if *frontend {
		serverType = "gateway"
	}
	builder := pitaya.NewDefaultBuilder(*frontend, serverType, pitaya.Cluster, map[string]string{
		constants.GRPCHostKey: "127.0.0.1", constants.GRPCPortKey: strconv.Itoa(*port),
	}, *conf)
	serverConfig := builder.Config.Cluster.RPC.Server.Grpc
	serverConfig.Port = *port
	server, err := cluster.NewGRPCServer(serverConfig, builder.Server, builder.MetricsReporters)
	if err != nil {
		panic(err)
	}
	builder.RPCServer = server
	bindings := modules.NewETCDBindingStorage(builder.Server, builder.SessionPool, builder.Config.Modules.BindingStorage.Etcd)
	client, err := cluster.NewGRPCClient(builder.Config.Cluster.RPC.Client.Grpc, builder.Server, builder.MetricsReporters, bindings, cluster.NewInfoRetriever(builder.Config.Cluster.Info))
	if err != nil {
		panic(err)
	}
	builder.RPCClient = client
	if *frontend {
		builder.AddAcceptor(acceptor.NewTCPAcceptor("127.0.0.1:17001"))
	}
	app := builder.Build()
	app.RegisterModule(bindings, "benchmark-bindings")
	if !*frontend {
		app.Register(&Echo{}, component.WithName("echo"), component.WithNameFunc(strings.ToLower))
	}
	fmt.Printf("benchmark role=%s rpc=%d\n", serverType, *port)
	defer app.Shutdown()
	app.Start()
}
