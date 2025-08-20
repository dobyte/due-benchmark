package log_test

import (
	"bufio"
	"os"
	"sync"
	"testing"

	"github.com/arthurkiller/rollingwriter"
	"github.com/dobyte/due/v2/log"
	gologger "github.com/donnie4w/go-logger/logger"
)

const (
	outDir    = "./temp"
	debugText = ">>>>>>this is debug message>>>>>>this is debug message"
)

func init() {
	if _, err := os.Stat(outDir); err != nil {
		_ = os.MkdirAll(outDir, 0755)
	}
}

func Benchmark_Std_SerialIO(b *testing.B) {
	out, err := os.OpenFile("./temp/s_std.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		b.Fatal(err)
	}
	defer out.Close()

	w := bufio.NewWriter(out)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if _, err = w.WriteString(debugText + "\n"); err != nil {
			b.Fatal(err)
		}

		if err = w.Flush(); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_Std_ParallelIO(b *testing.B) {
	out, err := os.OpenFile("./temp/p_std.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		b.Fatal(err)
	}
	defer out.Close()

	w := bufio.NewWriter(out)

	var i int64 = 0

	mu := &sync.Mutex{}

	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		if i == 30000 {
			return
		}
		i++
		for pb.Next() {
			func() {
				mu.Lock()
				defer mu.Unlock()

				if _, err = w.Write([]byte(debugText + "\n")); err != nil {
					b.Fatal(err)
				}

				if err = w.Flush(); err != nil {
					b.Fatal(err)
				}
			}()
		}
	})
}

//var (
//	logger = log.NewLogger(
//		log.WithLevel(log.LevelDebug),
//		log.WithSyncers(file.NewSyncer(
//			file.WithPath("./temp/s_due.log"),
//			file.WithMaxSize(500*1024*1024),
//		)),
//	)
//)

func Benchmark_Due_SerialIO(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		log.Info(debugText)
	}
}

func Benchmark_Due_ParallelIO(b *testing.B) {
	var i int64 = 0

	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		if i == 30000 {
			return
		}
		i++
		for pb.Next() {
			log.Info(debugText)
		}
	})
}

func Benchmark_RollingWriter_SerialIO(b *testing.B) {
	w, err := rollingwriter.NewWriterFromConfig(&rollingwriter.Config{
		LogPath:       "./temp",
		FileName:      "s_rollingwriter",
		RollingPolicy: rollingwriter.WithoutRolling,
		WriterMode:    "lock",
	})
	if err != nil {
		b.Fatal(err)
	}
	defer w.Close()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err = w.Write([]byte(debugText)); err != nil {
			b.Fatal(err)
		}
	}
}

func Benchmark_RollingWriter_ParallelIO(b *testing.B) {
	w, err := rollingwriter.NewWriterFromConfig(&rollingwriter.Config{
		LogPath:       "./temp",
		FileName:      "p_rollingwriter",
		RollingPolicy: rollingwriter.WithoutRolling,
		WriterMode:    "buffer",
	})
	if err != nil {
		b.Fatal(err)
	}
	defer w.Close()

	var i int64 = 0

	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		if i == 30000 {
			return
		}
		i++
		for pb.Next() {
			if _, err = w.Write([]byte(debugText)); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func Benchmark_Gologger_SerialIO(b *testing.B) {
	logger := gologger.NewLogger()
	logger.SetRollingFile("./temp", "s_gologger.log", 500, gologger.MB)
	logger.SetConsole(false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Debug(debugText)
	}
}

func Benchmark_Gologger_ParallelIO(b *testing.B) {
	logger := gologger.NewLogger()
	logger.SetRollingFile("./temp", "p_gologger.log", 500, gologger.MB)
	logger.SetConsole(false)

	var i int64 = 0

	b.SetParallelism(20)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		if i == 30000 {
			return
		}
		i++
		for pb.Next() {
			logger.Debug(debugText)
		}
	})
}
