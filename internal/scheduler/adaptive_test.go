package scheduler

import (
	"strings"
	"testing"

	"github.com/Gthulhu/plugin/models"
)

func TestAdaptiveWakeupFlagRemoved(t *testing.T) {
	err := Run([]string{"-adaptive-wakeup"})
	if err == nil || !strings.Contains(err.Error(), "flag provided but not defined: -adaptive-wakeup") {
		t.Fatalf("got %v, want removed-flag error", err)
	}
}

func TestAdaptiveWakeup(t *testing.T) {
	for _, tc := range []struct {
		name string
		task models.QueuedTask
		want bool
	}{
		{"short wakeup", models.QueuedTask{Flags: 1, StartTs: 10, StopTs: 100010, SumExecRuntime: 100000}, true},
		{"threshold", models.QueuedTask{Flags: 1, StopTs: 1_000_000, SumExecRuntime: 1_000_000}, true},
		{"long segment", models.QueuedTask{Flags: 1, StopTs: 1_000_001, SumExecRuntime: 1_000_001}, false},
		{"long previous burst with short final segment", models.QueuedTask{Flags: 1, StopTs: 100000, SumExecRuntime: 10_000_000}, false},
		{"unknown previous burst", models.QueuedTask{Flags: 1, StopTs: 100000}, false},
		{"unknown", models.QueuedTask{Flags: 1}, false},
		{"invalid timestamps", models.QueuedTask{Flags: 1, StartTs: 2, StopTs: 1, SumExecRuntime: 100000}, false},
		{"reenqueue", models.QueuedTask{StopTs: 100000, SumExecRuntime: 100000}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := adaptiveWakeup(&tc.task, 1); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func TestAdaptiveDeadline(t *testing.T) {
	for _, tc := range []struct {
		name                   string
		deadline, budget, want uint64
	}{
		{"explicit priority", 0, 20_000_000, 0},
		{"floor", 1, 20_000_000, 1},
		{"below budget", 19_999_999, 20_000_000, 1},
		{"at budget", 20_000_000, 20_000_000, 1},
		{"above budget", 20_000_002, 20_000_000, 2},
		{"no budget", 42, 0, 42},
		{"maximum deadline", ^uint64(0), 20_000_000, ^uint64(0) - 20_000_000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := adaptiveDeadline(tc.deadline, tc.budget); got != tc.want {
				t.Fatalf("got %d, want %d", got, tc.want)
			}
		})
	}
}
