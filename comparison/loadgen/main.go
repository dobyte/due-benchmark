package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"net"
	"os"
	"sort"
	"sync"
	"time"
)

type result struct {
	Framework    string         `json:"framework"`
	Protocol     string         `json:"protocol"`
	PayloadBytes int            `json:"payload_bytes"`
	Concurrency  int            `json:"concurrency"`
	Requests     int            `json:"requests"`
	Warmup       int            `json:"warmup_requests"`
	Success      int            `json:"success"`
	Failed       int            `json:"failed"`
	ElapsedMS    float64        `json:"elapsed_ms"`
	QPS          float64        `json:"qps"`
	P50US        float64        `json:"p50_us"`
	P99US        float64        `json:"p99_us"`
	P999US       float64        `json:"p999_us"`
	Errors       map[string]int `json:"errors,omitempty"`
	StartedAt    string         `json:"started_at"`
}

type worker struct {
	conn      net.Conn
	codec     *wireCodec
	success   int
	errors    map[string]int
	latencies []int64
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	addr := flag.String("addr", "127.0.0.1:17001", "gateway TCP address")
	framework := flag.String("framework", "unknown", "framework label")
	protocol := flag.String("protocol", "length32", "length32, due, or pitaya")
	concurrency := flag.Int("concurrency", 32, "persistent connections; one outstanding request per connection")
	requests := flag.Int("requests", 100000, "total measured requests")
	warmup := flag.Int("warmup", 10000, "total unmeasured warmup requests")
	size := flag.Int("bytes", 1024, "application payload bytes")
	timeout := flag.Duration("timeout", time.Second, "per request deadline")
	out := flag.String("out", "-", "JSONL file (append), or stdout")
	flag.Parse()
	if *concurrency < 1 || *requests < *concurrency || *warmup < 0 || *size < 8 || *size > 1<<20 || *timeout <= 0 {
		return fmt.Errorf("require requests >= concurrency > 0, warmup >= 0, bytes in [8,1048576], timeout > 0")
	}
	if *protocol != "length32" && *protocol != "due" && *protocol != "pitaya" {
		return fmt.Errorf("unsupported protocol %q", *protocol)
	}
	workers := make([]*worker, *concurrency)
	defer func() {
		for _, w := range workers {
			if w != nil {
				w.conn.Close()
			}
		}
	}()
	for i := range workers {
		conn, err := net.DialTimeout("tcp", *addr, *timeout)
		if err != nil {
			return fmt.Errorf("connect worker %d: %w", i, err)
		}
		workers[i] = &worker{conn: conn, codec: newWireCodec(*protocol, *size), errors: make(map[string]int)}
		if err = workers[i].codec.handshake(conn, *timeout); err != nil {
			return fmt.Errorf("handshake worker %d: %w", i, err)
		}
	}
	errCh := make(chan error, len(workers))
	var wg sync.WaitGroup
	for i, w := range workers {
		count := share(*warmup, len(workers), i)
		wg.Add(1)
		go func(w *worker, count int) {
			defer wg.Done()
			for n := 0; n < count; n++ {
				if err := w.codec.echo(w.conn, *timeout); err != nil {
					errCh <- err
					return
				}
			}
		}(w, count)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		return fmt.Errorf("warmup failed: %w", err)
	}
	startGate := make(chan struct{})
	var ready sync.WaitGroup
	ready.Add(len(workers))
	for i, w := range workers {
		count := share(*requests, len(workers), i)
		w.latencies = make([]int64, 0, count)
		wg.Add(1)
		go func(w *worker, count int) {
			defer wg.Done()
			ready.Done()
			<-startGate
			for n := 0; n < count; n++ {
				begin := time.Now()
				err := w.codec.echo(w.conn, *timeout)
				elapsed := time.Since(begin).Nanoseconds()
				if err != nil {
					// Framing is uncertain after a timeout. Count the remainder as failed
					// or skipped and invalidate this point instead of reusing the stream.
					w.errors[err.Error()] += count - n
					return
				}
				w.success++
				w.latencies = append(w.latencies, elapsed)
			}
		}(w, count)
	}
	ready.Wait()
	fmt.Fprintln(os.Stderr, "BENCH_MEASURE_BEGIN")
	start := time.Now()
	close(startGate)
	wg.Wait()
	elapsed := time.Since(start)
	fmt.Fprintln(os.Stderr, "BENCH_MEASURE_END")
	r := result{Framework: *framework, Protocol: *protocol, PayloadBytes: *size, Concurrency: *concurrency, Requests: *requests, Warmup: *warmup, ElapsedMS: float64(elapsed.Nanoseconds()) / 1e6, Errors: make(map[string]int), StartedAt: start.UTC().Format(time.RFC3339Nano)}
	latencies := make([]int64, 0, *requests)
	for _, w := range workers {
		r.Success += w.success
		latencies = append(latencies, w.latencies...)
		for cause, count := range w.errors {
			r.Errors[cause] += count
		}
	}
	r.Failed = r.Requests - r.Success
	r.QPS = float64(r.Success) / elapsed.Seconds()
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	r.P50US, r.P99US, r.P999US = percentile(latencies, .5), percentile(latencies, .99), percentile(latencies, .999)
	var output io.Writer = os.Stdout
	if *out != "-" {
		file, err := os.OpenFile(*out, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer file.Close()
		output = file
	}
	if err := json.NewEncoder(output).Encode(r); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "%s c=%d bytes=%d qps=%.2f success=%d failed=%d p99=%.2fus\n", r.Framework, r.Concurrency, r.PayloadBytes, r.QPS, r.Success, r.Failed, r.P99US)
	if r.Failed != 0 {
		return fmt.Errorf("invalid benchmark point: %d failed or skipped", r.Failed)
	}
	return nil
}

func share(total, workers, index int) int {
	n := total / workers
	if index < total%workers {
		n++
	}
	return n
}
func percentile(v []int64, p float64) float64 {
	if len(v) == 0 {
		return 0
	}
	return float64(v[int(math.Ceil(p*float64(len(v))))-1]) / 1000
}
