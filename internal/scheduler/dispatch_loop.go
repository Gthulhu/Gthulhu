package scheduler

import (
	"context"
	"log/slog"

	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin"
	core "github.com/Gthulhu/qumun/goland_core"
)

func runSchedulerLoop(
	ctx context.Context,
	bpfModule *core.Sched,
	_ plugin.CustomScheduler,
	sliceNsDefault,
	sliceNsMin uint64,
) error {
	var t *models.QueuedTask
	var task *core.DispatchedTask
	var cpu int32
	var err error

	slog.Info("scheduler loop started")

	for {
		select {
		case <-ctx.Done():
			slog.Info("context done, exiting scheduler loop")
			return nil
		default:
		}

		cnt := bpfModule.DrainQueuedTask()
		if cnt > 0 {
			err = bpfModule.DecNrQueued(cnt)
			if err != nil {
				slog.Warn("DecNrQueued failed", "error", err)
				return err
			}
		}

		t = bpfModule.SelectQueuedTask()
		if t == nil {
			bpfModule.BlockTilReadyForDequeue(ctx)
		} else {
			task = core.NewDispatchedTask(t)
			task.Vtime = t.Vtime
			if t.Vtime != 0 {
				task.Vtime += min(t.SumExecRuntime, sliceNsDefault*100)
			}

			customTime := bpfModule.DetermineTimeSlice(t)
			if customTime > 0 {
				task.SliceNs = min(customTime, (t.StopTs-t.StartTs)*11/10)
			} else {
				task.SliceNs = sliceNsMin * t.Weight / 100
			}
			err, cpu = bpfModule.SelectCPU(t)
			if err != nil {
				slog.Warn("SelectCPU failed", "error", err)
				return err
			}
			task.Cpu = cpu

			err = bpfModule.DispatchTask(task)
			if err != nil {
				slog.Warn("DispatchTask failed", "error", err)
				return err
			}

			if bpfModule.GetPoolCount() == 0 {
				err = core.NotifyComplete(0)
				if err != nil {
					slog.Warn("NotifyComplete failed", "error", err)
					return err
				}
			}
		}
	}
}
