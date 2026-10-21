// Dedicated service discovery for the benchmark. It uses separate loopback
// ports and a data directory passed by the orchestrator.
package main

import (
	"flag"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.etcd.io/etcd/server/v3/embed"
)

func main() {
	data := flag.String("data", "benchmark-etcd-data", "etcd data directory")
	flag.Parse()
	cfg := embed.NewConfig()
	cfg.Dir = *data
	cfg.LogLevel = "error"
	client, _ := url.Parse("http://127.0.0.1:17379")
	peer, _ := url.Parse("http://127.0.0.1:17380")
	cfg.ListenClientUrls = []url.URL{*client}
	cfg.AdvertiseClientUrls = cfg.ListenClientUrls
	cfg.ListenPeerUrls = []url.URL{*peer}
	cfg.AdvertisePeerUrls = cfg.ListenPeerUrls
	cfg.InitialCluster = "default=" + peer.String()
	server, err := embed.StartEtcd(cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer server.Close()
	select {
	case <-server.Server.ReadyNotify():
	case <-time.After(30 * time.Second):
		fmt.Fprintln(os.Stderr, "etcd startup timeout")
		return
	}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
	select {
	case <-stop:
	case err = <-server.Err():
		fmt.Fprintln(os.Stderr, err)
	}
}
