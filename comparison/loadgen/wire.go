package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"time"
)

type wireCodec struct {
	protocol                 string
	payload, frame, response []byte
	seq                      uint32
}

func newWireCodec(protocol string, size int) *wireCodec {
	w := &wireCodec{protocol: protocol, payload: make([]byte, size), response: make([]byte, size+4096), frame: make([]byte, 0, size+256)}
	for i := range w.payload {
		w.payload[i] = byte(i % 251)
	}
	return w
}

func (w *wireCodec) handshake(conn net.Conn, timeout time.Duration) error {
	if w.protocol != "pitaya" {
		return nil
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	if err := writePacket(conn, 1, []byte(`{"sys":{"type":"benchmark","version":"1.0"},"user":{}}`)); err != nil {
		return err
	}
	kind, body, err := readPacket(conn, w.response)
	if err != nil {
		return err
	}
	var value struct {
		Code int `json:"code"`
	}
	if kind != 1 || json.Unmarshal(body, &value) != nil || value.Code != 200 {
		return fmt.Errorf("handshake rejected: %s", body)
	}
	return writePacket(conn, 2, nil)
}

func (w *wireCodec) echo(conn net.Conn, timeout time.Duration) error {
	w.seq++
	if w.seq == 0 || w.seq > 65535 {
		w.seq = 1
	}
	binary.BigEndian.PutUint64(w.payload, uint64(w.seq))
	w.frame = w.frame[:0]
	switch w.protocol {
	case "due":
		w.frame = append(w.frame, 0, 0, 0, 0, 0, 0, 1, byte(w.seq>>8), byte(w.seq))
		w.frame = append(w.frame, w.payload...)
		binary.BigEndian.PutUint32(w.frame[:4], uint32(len(w.frame)-4))
	case "pitaya":
		w.frame = append(w.frame, 4, 0, 0, 0, 0)
		w.frame = binary.AppendUvarint(w.frame, uint64(w.seq))
		route := "worker.echo.echo"
		w.frame = append(w.frame, byte(len(route)))
		w.frame = append(w.frame, route...)
		w.frame = append(w.frame, w.payload...)
		n := len(w.frame) - 4
		w.frame[1], w.frame[2], w.frame[3] = byte(n>>16), byte(n>>8), byte(n)
	default:
		w.frame = append(w.frame, 0, 0, 0, 0)
		w.frame = append(w.frame, w.payload...)
		binary.BigEndian.PutUint32(w.frame[:4], uint32(len(w.payload)))
	}
	if err := conn.SetDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	if err := writeFull(conn, w.frame); err != nil {
		return err
	}
	if w.protocol == "pitaya" {
		for {
			kind, body, err := readPacket(conn, w.response)
			if err != nil {
				return err
			}
			if kind == 3 {
				if err = writePacket(conn, 3, nil); err != nil {
					return err
				}
				continue
			}
			if kind != 4 || len(body) < 2 || body[0] != 4 {
				return fmt.Errorf("invalid Pitaya response type/flags")
			}
			seq, n := binary.Uvarint(body[1:])
			if n <= 0 || seq != uint64(w.seq) {
				return fmt.Errorf("Pitaya sequence mismatch")
			}
			return verifyPayload(body[1+n:], w.payload)
		}
	}
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return err
	}
	n := int(binary.BigEndian.Uint32(header[:]))
	expected := len(w.payload)
	if w.protocol == "due" {
		expected += 5
	}
	if n != expected {
		return fmt.Errorf("response length %d; want %d", n, expected)
	}
	body := w.response[:n]
	if _, err := io.ReadFull(conn, body); err != nil {
		return err
	}
	if w.protocol == "due" {
		if body[0] != 0 || binary.BigEndian.Uint16(body[1:3]) != 1 || binary.BigEndian.Uint16(body[3:5]) != uint16(w.seq) {
			return fmt.Errorf("Due route/sequence mismatch")
		}
		body = body[5:]
	}
	return verifyPayload(body, w.payload)
}

func verifyPayload(got, want []byte) error {
	if !bytes.Equal(got, want) {
		return fmt.Errorf("response payload mismatch")
	}
	return nil
}
func writeFull(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}
func writePacket(conn net.Conn, kind byte, payload []byte) error {
	n := len(payload)
	return writeFull(conn, append([]byte{kind, byte(n >> 16), byte(n >> 8), byte(n)}, payload...))
}
func readPacket(conn net.Conn, storage []byte) (byte, []byte, error) {
	var header [4]byte
	if _, err := io.ReadFull(conn, header[:]); err != nil {
		return 0, nil, err
	}
	n := int(header[1])<<16 | int(header[2])<<8 | int(header[3])
	if n > len(storage) {
		return 0, nil, fmt.Errorf("packet exceeds buffer: %d", n)
	}
	_, err := io.ReadFull(conn, storage[:n])
	return header[0], storage[:n], err
}
