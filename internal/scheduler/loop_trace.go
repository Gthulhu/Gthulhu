package scheduler

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/bits"
	"sync/atomic"
	"time"

	"golang.org/x/sys/unix"
)

type loopStage uint64

const (
	stageDrain loopStage = iota + 1
	stageDecQueued
	stageSelectTask
	stageWait
	stageNewTask
	stageTimeSlice
	stageSelectCPU
	stageDispatch
	stagePoolCount
	stageNotify
	stageCount
)

var loopStageNames = [...]string{
	"between_calls", "DrainQueuedTask", "DecNrQueued", "SelectQueuedTask",
	"BlockTilReadyForDequeue", "NewDispatchedTask", "DetermineTimeSlice",
	"SelectCPU", "DispatchTask", "GetPoolCount", "NotifyComplete",
}

type loopStageStats struct {
	calls   atomic.Uint64
	total   atomic.Int64
	max     atomic.Int64
	slow    atomic.Uint64
	buckets [64]atomic.Uint64
}

type cpuSample struct {
	wall time.Duration
	cpu  time.Duration
}

type loopTrace struct {
	origin time.Time
	// One atomic snapshot pairs the stage (low four bits) with its start time.
	active       atomic.Uint64
	stats        [stageCount]loopStageStats
	cpuEnabled   bool
	cpuStart     int64
	cpuWallStart time.Time
	cpuCalls     atomic.Uint64
	cpuTotal     atomic.Int64
	cpuWorst     atomic.Pointer[cpuSample]
}

func (s *loopStageStats) quantiles() (time.Duration, time.Duration, time.Duration) {
	var counts [64]uint64
	var total uint64
	for i := range counts {
		counts[i] = s.buckets[i].Load()
		total += counts[i]
	}
	quantile := func(permille uint64) time.Duration {
		if total == 0 {
			return 0
		}
		rank := total - total*(1000-permille)/1000
		var count uint64
		for i, value := range counts {
			count += value
			if count >= rank {
				if i == 63 {
					return time.Duration(math.MaxInt64)
				}
				return time.Duration(uint64(1) << i)
			}
		}
		panic("invalid scheduler timing histogram")
	}
	return quantile(500), quantile(990), quantile(999)
}

func threadCPUTime() (int64, error) {
	var ts unix.Timespec
	if err := unix.ClockGettime(unix.CLOCK_THREAD_CPUTIME_ID, &ts); err != nil {
		return 0, fmt.Errorf("scheduler diagnostic thread CPU clock: %w", err)
	}
	return ts.Nano(), nil
}

func (t *loopTrace) beginCPU() error {
	if t == nil || !t.cpuEnabled {
		return nil
	}
	t.cpuWallStart = time.Now()
	start, err := threadCPUTime()
	if err != nil {
		return err
	}
	t.cpuStart = start
	return nil
}

func (t *loopTrace) endCPU() error {
	if t == nil || !t.cpuEnabled {
		return nil
	}
	end, err := threadCPUTime()
	if err != nil {
		return err
	}
	t.recordCPU(time.Since(t.cpuWallStart), time.Duration(end-t.cpuStart))
	return nil
}

func (t *loopTrace) recordCPU(wall, cpu time.Duration) {
	if previous := t.cpuWorst.Load(); previous == nil || wall > previous.wall {
		t.cpuWorst.Store(&cpuSample{wall: wall, cpu: cpu})
	}
	t.cpuTotal.Add(int64(cpu))
	t.cpuCalls.Add(1)
}

func (t *loopTrace) begin(stage loopStage) {
	if t != nil {
		t.beginAt(stage, time.Since(t.origin))
	}
}

func (t *loopTrace) beginAt(stage loopStage, elapsed time.Duration) {
	t.active.Store(uint64(elapsed)<<4 | uint64(stage))
}

func (t *loopTrace) end() {
	if t != nil {
		t.endAt(time.Since(t.origin))
	}
}

func (t *loopTrace) endAt(elapsed time.Duration) {
	active := t.active.Swap(0)
	stage := loopStage(active & 15)
	if stage == 0 {
		return
	}
	duration := int64(elapsed - time.Duration(active>>4))
	stats := &t.stats[stage]
	bucket := 0
	if duration > 1 {
		bucket = bits.Len64(uint64(duration - 1))
	}
	stats.buckets[bucket].Add(1)
	stats.calls.Add(1)
	stats.total.Add(duration)
	for previous := stats.max.Load(); duration > previous; previous = stats.max.Load() {
		if stats.max.CompareAndSwap(previous, duration) {
			break
		}
	}
	if duration >= int64(time.Millisecond) {
		stats.slow.Add(1)
	}
}

func (t *loopTrace) report(logger *slog.Logger) {
	active := t.active.Load()
	stage := loopStage(active & 15)
	attrs := []slog.Attr{slog.String("active", loopStageNames[stage])}
	if stage != 0 {
		attrs = append(attrs, slog.Duration("active_for", time.Since(t.origin)-time.Duration(active>>4)))
	}
	for stage := stageDrain; stage < stageCount; stage++ {
		stats := &t.stats[stage]
		if calls := stats.calls.Load(); calls != 0 {
			p50, p99, p999 := stats.quantiles()
			attrs = append(attrs, slog.Group(loopStageNames[stage],
				slog.Uint64("calls", calls),
				slog.Duration("avg", time.Duration(stats.total.Load()/int64(calls))),
				slog.Duration("max", time.Duration(stats.max.Load())),
				slog.Uint64("ge_1ms", stats.slow.Load()),
				slog.Duration("p50_le", p50),
				slog.Duration("p99_le", p99),
				slog.Duration("p999_le", p999),
			))
		}
	}
	if calls := t.cpuCalls.Load(); calls != 0 {
		sample := t.cpuWorst.Load()
		attrs = append(attrs, slog.Group("SelectCPU_thread",
			slog.Uint64("calls", calls),
			slog.Duration("cpu_avg", time.Duration(t.cpuTotal.Load()/int64(calls))),
			slog.Duration("worst_wall", sample.wall),
			slog.Duration("worst_cpu", sample.cpu),
			slog.Duration("worst_non_cpu_estimate", max(0, sample.wall-sample.cpu)),
		))
	}
	logger.LogAttrs(context.Background(), slog.LevelInfo, "scheduler loop timing", attrs...)
}

func startLoopTrace(logger *slog.Logger) (*loopTrace, func()) {
	trace := &loopTrace{origin: time.Now()}
	stop, done := make(chan struct{}), make(chan struct{})
	go func() {
		defer close(done)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				trace.report(logger)
			case <-stop:
				trace.report(logger)
				return
			}
		}
	}()
	return trace, func() {
		close(stop)
		<-done
	}
}
