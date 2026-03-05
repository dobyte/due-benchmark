package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/dobyte/due/network/tcp/v2"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

func main() {
	server := tcp.NewServer(
		tcp.WithServerHeartbeatInterval(0),
	)

	server.OnStart(func() {
		log.Info("server is started")
	})

	server.OnReceive(func(conn network.Conn, msg []byte) {
		message, err := packet.UnpackMessage(msg)
		if err != nil {
			log.Errorf("unpack message failed: %v", err)
			return
		}

		data, err := packet.PackMessage(&packet.Message{
			Seq:    message.Seq,
			Route:  message.Route,
			Buffer: message.Buffer,
		})
		if err != nil {
			log.Errorf("pack message failed: %v", err)
			return
		}

		if err = conn.Push(data); err != nil {
			log.Errorf("push message failed: %v", err)
			return
		}
	})

	if err := server.Start(); err != nil {
		log.Fatalf("start server failed: %v", err)
	}

	go func() {
		err := http.ListenAndServe(":8089", nil)
		if err != nil {
			log.Errorf("pprof server start failed: %v", err)
		}
	}()

	select {}
}
