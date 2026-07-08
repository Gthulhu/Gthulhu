package scheduler

import (
	"context"
	"log/slog"

	"github.com/Gthulhu/Gthulhu/internal/schedext"
	"github.com/Gthulhu/Gthulhu/monitor"
	"github.com/Gthulhu/plugin/plugin"
)

type defaultSchedExtChecker struct{}

func (defaultSchedExtChecker) CheckSupport() error {
	return schedext.CheckSupport()
}

type defaultMonitorStarter struct{}

func (defaultMonitorStarter) StartMonitor(ctx context.Context, cfg monitor.Config, logger *slog.Logger) error {
	return monitor.StartMonitor(ctx, cfg, logger)
}

type defaultSchedulerPluginFactory struct{}

func (defaultSchedulerPluginFactory) New(ctx context.Context, cfg *plugin.SchedConfig) (plugin.CustomScheduler, error) {
	return plugin.NewSchedulerPlugin(ctx, cfg)
}
