package scheduler

import "github.com/Gthulhu/plugin/models"

// Only known short bursts qualify; an unobserved task is not assumed interactive.
func adaptiveWakeup(task *models.QueuedTask, wakeupFlag uint64) bool {
	return task.Flags&wakeupFlag != 0 &&
		task.StopTs > task.StartTs &&
		task.StopTs-task.StartTs <= 1_000_000 &&
		task.SumExecRuntime > 0 &&
		task.SumExecRuntime <= 1_000_000
}

func adaptiveDeadline(deadline, budget uint64) uint64 {
	if deadline == 0 {
		return 0
	}
	if deadline <= budget {
		return 1
	}
	return deadline - budget
}
