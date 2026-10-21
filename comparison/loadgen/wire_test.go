package main

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

// The fake peer splits every response byte at a different stream boundary.
// A corrupt final payload byte must invalidate the request for every protocol.
func TestFragmentedAndCorruptResponses(t *testing.T) {
	for _, protocol := range []string{"length32", "due", "pitaya"} {
		for _, corrupt := range []bool{false, true} {
			t.Run(protocol+map[bool]string{false: "/valid", true: "/corrupt"}[corrupt], func(t *testing.T) {
				client, server := net.Pipe()
				defer client.Close()
				done := make(chan error, 1)
				go func() {
					defer server.Close()
					server.SetDeadline(time.Now().Add(3 * time.Second))
					var header [4]byte
					if _, err := io.ReadFull(server, header[:]); err != nil {
						done <- err
						return
					}
					size := int(binary.BigEndian.Uint32(header[:]))
					if protocol == "pitaya" {
						size &= 0xFFFFFF
					}
					body := make([]byte, size)
					if _, err := io.ReadFull(server, body); err != nil {
						done <- err
						return
					}
					response := append(header[:], body...)
					if protocol == "pitaya" {
						// Response: packet Data, message Response, id=1, raw payload.
						response = append([]byte{4, 0, 0, 66, 4, 1}, body[len(body)-64:]...)
					}
					if corrupt {
						response[len(response)-1] ^= 0xFF
					}
					for _, value := range response {
						if _, err := server.Write([]byte{value}); err != nil {
							done <- err
							return
						}
					}
					done <- nil
				}()
				err := newWireCodec(protocol, 64).echo(client, 3*time.Second)
				if (err != nil) != corrupt {
					t.Fatalf("corrupt=%v, error=%v", corrupt, err)
				}
				if err := <-done; err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
