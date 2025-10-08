package main

import (
	"net/http"
	_ "net/http/pprof"

	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/packet"
)

func main() {
	server := ws.NewServer(ws.WithServerHeartbeatInterval(0))

	server.OnStart(func() {
		log.Info("server is started")
	})

	server.OnReceive(func(conn network.Conn, msg []byte) {
		_, err := packet.UnpackMessage(msg)
		if err != nil {
			log.Errorf("unpack message failed: %v", err)
			return
		}

		msg, err = packet.PackMessage(&packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: []byte("I'm fine~~"),
		})
		if err != nil {
			log.Errorf("pack message failed: %v", err)
			return
		}

		if err = conn.Push(msg); err != nil {
			log.Errorf("push message failed: %v", err)
			return
		}
	})

	server.OnUpgrade(func(w http.ResponseWriter, r *http.Request) (allowed bool) {
		return true
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
