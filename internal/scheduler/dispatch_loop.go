package scheduler

import (
	"context"
	"log/slog"
	"runtime"

	"github.com/Gthulhu/plugin/models"
	core "github.com/Gthulhu/qumun/goland_core"
	"github.com/Gthulhu/qumun/util"
	"golang.org/x/sys/unix"
)

func runSchedulerLoop(
	ctx context.Context,
	bpfModule *core.Sched,
	sliceNsDefault,
	sliceNsMin uint64,
	traceEnabled, traceCPU, preemptLoop bool,
) error {
	var t *models.QueuedTask
	var task *core.DispatchedTask
	var cpu int32
	var err error

	slog.Info("scheduler loop started")
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	enums, err := util.GetScxEnums()
	if err != nil {
		return err
	}
	wakeupFlag, preemptFlag := enums.SCX_ENQ_WAKEUP, enums.SCX_ENQ_PREEMPT

	if preemptLoop {
		tid := uint32(unix.Gettid())
		bpfModule.SetUserSchedLoopTid(tid)
		defer bpfModule.SetUserSchedLoopTid(0)
		slog.Info("experimental scheduler loop wakeup preemption enabled", "tid", tid, "min_interval", "1ms")
	}

	var trace *loopTrace
	if traceEnabled {
		var stop func()
		trace, stop = startLoopTrace(slog.Default())
		defer stop()
		trace.cpuEnabled = traceCPU
		slog.Info("scheduler loop tracing enabled; timings include off-CPU time and add diagnostic overhead",
			"gomaxprocs", runtime.GOMAXPROCS(0), "tid", unix.Gettid(), "thread_cpu_timing", traceCPU)
	}

	for {
		select {
		case <-ctx.Done():
			slog.Info("context done, exiting scheduler loop")
			return nil
		default:
		}

		trace.begin(stageDrain)
		cnt := bpfModule.DrainQueuedTask()
		trace.end()
		if err := bpfModule.QueuedError(); err != nil {
			return err
		}
		if cnt > 0 {
			trace.begin(stageDecQueued)
			err = bpfModule.DecNrQueued(cnt)
			trace.end()
			if err != nil {
				slog.Warn("DecNrQueued failed", "error", err)
				return err
			}
		}

		trace.begin(stageSelectTask)
		t = bpfModule.SelectQueuedTask()
		trace.end()
		if t == nil {
			trace.begin(stageWait)
			bpfModule.BlockTilReadyForDequeue(ctx)
			trace.end()
			if err := bpfModule.QueuedError(); err != nil {
				return err
			}
		} else {
			trace.begin(stageNewTask)
			task = core.NewDispatchedTask(t)
			trace.end()
			task.Vtime = t.Vtime
			if t.Vtime != 0 {
				task.Vtime += min(t.SumExecRuntime, sliceNsDefault*100)
			}

			trace.begin(stageTimeSlice)
			customTime := bpfModule.DetermineTimeSlice(t)
			trace.end()
			if customTime > 0 {
				task.SliceNs = min(customTime, (t.StopTs-t.StartTs)*11/10)
			} else {
				task.SliceNs = sliceNsMin * t.Weight / 100
			}
			trace.begin(stageSelectCPU)
			if err := trace.beginCPU(); err != nil {
				trace.end()
				return err
			}
			err, cpu = bpfModule.SelectCPU(t)
			cpuTimingErr := trace.endCPU()
			trace.end()
			if cpuTimingErr != nil {
				return cpuTimingErr
			}
			if err != nil {
				slog.Warn("SelectCPU failed", "error", err)
				return err
			}
			task.Cpu = cpu
			if adaptiveWakeup(t, wakeupFlag) {
				task.Flags |= preemptFlag
				task.Vtime = adaptiveDeadline(task.Vtime, sliceNsDefault)
				if cpu == core.RL_CPU_ANY && t.NrCpusAllowed == 1 {
					task.Cpu = t.Cpu
				}
			}

			trace.begin(stageDispatch)
			err = bpfModule.DispatchTask(task)
			trace.end()
			if err != nil {
				slog.Warn("DispatchTask failed", "error", err)
				return err
			}

			trace.begin(stagePoolCount)
			poolCount := bpfModule.GetPoolCount()
			trace.end()
			if poolCount == 0 {
				trace.begin(stageNotify)
				err = core.NotifyComplete(0)
				trace.end()
				if err != nil {
					slog.Warn("NotifyComplete failed", "error", err)
					return err
				}
				if err := bpfModule.YieldScheduler(); err != nil {
					return err
				}
			}
		}
	}
}
