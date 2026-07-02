package scheduler

import (
	"context"
	"log/slog"

	"github.com/Gthulhu/Gthulhu/monitor"
	"github.com/Gthulhu/plugin/plugin"
)

// SchedExtChecker abstracts sched_ext capability probing.
type SchedExtChecker interface {
	CheckSupport() error
}

// MonitorStarter abstracts monitor startup wiring.
type MonitorStarter interface {
	StartMonitor(ctx context.Context, cfg monitor.Config, logger *slog.Logger) error
}

// SchedulerPluginFactory abstracts scheduler plugin construction.
type SchedulerPluginFactory interface {
	New(ctx context.Context, cfg *plugin.SchedConfig) (plugin.CustomScheduler, error)
}
