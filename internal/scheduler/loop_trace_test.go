package scheduler

import (
	"bytes"
	"log/slog"
	"math"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestLoopTraceDisabled(t *testing.T) {
	var trace *loopTrace
	trace.begin(stageSelectCPU)
	trace.end()
	if err := trace.beginCPU(); err != nil {
		t.Fatal(err)
	}
	if err := trace.endCPU(); err != nil {
		t.Fatal(err)
	}
}

func TestLoopTraceDurations(t *testing.T) {
	trace := &loopTrace{}
	for _, duration := range []time.Duration{time.Millisecond - 1, time.Millisecond, 5 * time.Second} {
		trace.beginAt(stageSelectCPU, 10*time.Second)
		trace.endAt(10*time.Second + duration)
	}
	stats := &trace.stats[stageSelectCPU]
	if got := stats.calls.Load(); got != 3 {
		t.Fatalf("calls = %d, want 3", got)
	}
	if got := stats.total.Load(); got != int64(5*time.Second+2*time.Millisecond-1) {
		t.Fatalf("total = %d", got)
	}
	if got := stats.max.Load(); got != int64(5*time.Second) {
		t.Fatalf("max = %d, want 5 seconds", got)
	}
	if got := stats.slow.Load(); got != 2 {
		t.Fatalf("ge_1ms = %d, want 2", got)
	}
	if got := trace.active.Load(); got != 0 {
		t.Fatalf("active after return = %d", got)
	}
}

func TestLoopTraceReport(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&output, nil))
	trace := &loopTrace{origin: time.Now().Add(-6 * time.Second)}
	trace.beginAt(stageDispatch, 0)
	trace.endAt(2 * time.Millisecond)
	trace.beginAt(stageWait, time.Second)
	trace.report(logger)
	for _, field := range []string{
		`"active":"BlockTilReadyForDequeue"`, `"active_for":`,
		`"DispatchTask":{"calls":1,"avg":2000000,"max":2000000,"ge_1ms":1,`,
		`"p99_le":2097152`,
	} {
		if !strings.Contains(output.String(), field) {
			t.Errorf("missing %s in %s", field, output.String())
		}
	}
}

func TestLoopTraceConcurrentReport(t *testing.T) {
	trace := &loopTrace{origin: time.Now()}
	logger := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			trace.report(logger)
		}
	}()
	for i := 0; i < 1000; i++ {
		trace.begin(stageDrain)
		trace.end()
		trace.recordCPU(time.Duration(i+1), time.Duration(i))
	}
	wg.Wait()
}

func TestLoopTraceStopReports(t *testing.T) {
	var output bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&output, nil))
	trace, stop := startLoopTrace(logger)
	trace.begin(stageNotify)
	trace.end()
	stop()
	if !strings.Contains(output.String(), "NotifyComplete.calls=1") {
		t.Fatalf("missing final summary: %s", output.String())
	}
}

func BenchmarkLoopTrace(b *testing.B) {
	b.Run("Disabled", func(b *testing.B) {
		var trace *loopTrace
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			trace.begin(stageSelectCPU)
			trace.end()
		}
	})
	b.Run("Enabled", func(b *testing.B) {
		trace := &loopTrace{origin: time.Now()}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			trace.begin(stageSelectCPU)
			trace.end()
		}
	})
	b.Run("ThreadCPU", func(b *testing.B) {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		trace := &loopTrace{origin: time.Now(), cpuEnabled: true}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			trace.begin(stageSelectCPU)
			if err := trace.beginCPU(); err != nil {
				b.Fatal(err)
			}
			if err := trace.endCPU(); err != nil {
				b.Fatal(err)
			}
			trace.end()
		}
	})
}

func TestLoopTraceQuantileBounds(t *testing.T) {
	for _, ns := range []int64{0, 1, 2, 3, 1024, 1025, 1_000_000, math.MaxInt64} {
		trace := &loopTrace{}
		trace.beginAt(stageSelectCPU, 0)
		trace.endAt(time.Duration(ns))
		p50, p99, p999 := trace.stats[stageSelectCPU].quantiles()
		want := time.Duration(1)
		for int64(want) < ns && want < 1<<62 {
			want *= 2
		}
		if ns > 1<<62 {
			want = time.Duration(math.MaxInt64)
		}
		if p50 != want || p99 != want || p999 != want {
			t.Fatalf("duration %d: quantiles %v/%v/%v, want %v", ns, p50, p99, p999, want)
		}
	}
}

func TestLoopTraceQuantileRanks(t *testing.T) {
	trace := &loopTrace{}
	for i := 0; i < 1000; i++ {
		duration := time.Nanosecond
		if i >= 990 {
			duration = 1024
		}
		if i == 999 {
			duration = 2048
		}
		trace.beginAt(stageSelectCPU, 0)
		trace.endAt(duration)
	}
	p50, p99, p999 := trace.stats[stageSelectCPU].quantiles()
	if p50 != 1 || p99 != 1 || p999 != 1024 {
		t.Fatalf("unexpected quantiles %v/%v/%v", p50, p99, p999)
	}
	var empty loopStageStats
	if a, b, c := empty.quantiles(); a != 0 || b != 0 || c != 0 {
		t.Fatalf("nonzero empty histogram: %v/%v/%v", a, b, c)
	}
}

func TestLoopTraceCPUSample(t *testing.T) {
	trace := &loopTrace{}
	trace.recordCPU(15*time.Millisecond, time.Millisecond)
	trace.recordCPU(time.Millisecond, time.Millisecond)
	var output bytes.Buffer
	trace.report(slog.New(slog.NewJSONHandler(&output, nil)))
	for _, field := range []string{
		`"cpu_avg":1000000`, `"worst_wall":15000000`,
		`"worst_cpu":1000000`, `"worst_non_cpu_estimate":14000000`,
	} {
		if !strings.Contains(output.String(), field) {
			t.Errorf("missing %s in %s", field, output.String())
		}
	}
}

func TestLoopTraceThreadCPUClock(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	trace := &loopTrace{cpuEnabled: true}
	if err := trace.beginCPU(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	if err := trace.endCPU(); err != nil {
		t.Fatal(err)
	}
	sample := trace.cpuWorst.Load()
	if sample == nil || sample.wall < 20*time.Millisecond || sample.cpu < 0 || sample.cpu >= sample.wall {
		t.Fatalf("invalid CPU/wall sample across sleep: %+v", sample)
	}
}
