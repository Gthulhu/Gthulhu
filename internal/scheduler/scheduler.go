package scheduler

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/Gthulhu/Gthulhu/internal/config"
	"github.com/Gthulhu/Gthulhu/internal/scheduler/policy"
	"github.com/Gthulhu/plugin/plugin"
	"github.com/Gthulhu/plugin/plugin/gthulhu"
	core "github.com/Gthulhu/qumun/goland_core"
	cache "github.com/Gthulhu/qumun/util"
)

func Run(args []string) error {
	schedChecker := SchedExtChecker(defaultSchedExtChecker{})
	monitorStarter := MonitorStarter(defaultMonitorStarter{})
	pluginFactory := SchedulerPluginFactory(defaultSchedulerPluginFactory{})

	fs := flag.NewFlagSet("scheduler", flag.ContinueOnError)
	fs.SetOutput(os.Stdout)
	fs.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: %s [scheduler] [flags]\n", os.Args[0])
		fmt.Fprintf(os.Stdout, "Default behavior is scheduler mode when no mode is specified.\n\n")
		fs.PrintDefaults()
	}
	configFile := fs.String("config", "", "Path to YAML configuration file")
	showHelper := fs.Bool("help", false, "Show help message")
	showExplain := fs.Bool("explain", false, "Explain configuration options")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *showHelper {
		fs.Usage()
		return nil
	}

	if *showExplain {
		fmt.Println(config.ExplainConfig())
		return nil
	}

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.IsMonitorEnabled() {
		monCfg := buildMonitorConfig(cfg)
		go func() {
			slog.Info("starting scheduling monitor",
				"bpfObject", monCfg.BPFObjectPath,
				"prometheusPort", monCfg.PrometheusPort,
				"monitorAll", monCfg.MonitorAll,
			)
			if err := monitorStarter.StartMonitor(ctx, monCfg, slog.Default()); err != nil {
				slog.Error("monitor goroutine error", "error", err)
			}
		}()
	}

	if !cfg.IsSchedulerEnabled() {
		slog.Info("running in monitor-only mode (no scheduler mode configured)")
		return waitForShutdown(ctx)
	}

	schedExtErr := schedChecker.CheckSupport()
	monitorOnly, decisionErr := policy.ShouldRunMonitorOnly(cfg, schedExtErr)
	if decisionErr != nil {
		return decisionErr
	}
	if monitorOnly {
		if schedExtErr != nil {
			slog.Warn("sched_ext unavailable; continuing in monitor-only mode", "error", schedExtErr)
		}
		return waitForShutdown(ctx)
	}

	var p plugin.CustomScheduler
	var sliceNsDefault, sliceNsMin uint64
	sliceNsDefault = cfg.Scheduler.SliceNsDefault
	sliceNsMin = cfg.Scheduler.SliceNsMin
	slog.Info("Scheduler configuration", "SliceNsDefault", sliceNsDefault, "SliceNsMin", sliceNsMin)
	pluginConfig := buildPluginConfig(cfg)
	p, err = pluginFactory.New(ctx, pluginConfig)
	if err != nil {
		return fmt.Errorf("failed to create plugin: %w", err)
	}

	bpfModule := core.LoadSched("main.bpf.o")
	defer bpfModule.Close()

	bpfModule.SetPlugin(p)

	if cfg.IsDebugEnabled() {
		slog.Info("Debug mode enabled")
		bpfModule.SetDebug(true)
	}

	if cfg.IsBuiltinIdleEnabled() {
		slog.Info("Built-in idle CPU selection enabled")
		bpfModule.SetBuiltinIdle(true)
	}

	if cfg.Scheduler.KernelMode {
		bpfModule.EnableKernelMode()
	}

	if !cfg.Scheduler.MaxTimeWatchdog {
		slog.Info("Max time watchdog disabled")
		bpfModule.DisableMaxTimeWatchdog()
	}

	if cfg.EarlyProcessing {
		slog.Info("Early processing enabled")
		bpfModule.SetEarlyProcessing(true)
	} else {
		slog.Info("Early processing disabled")
	}

	pid := os.Getpid()
	err = bpfModule.AssignUserSchedPid(pid)
	if err != nil {
		slog.Warn("AssignUserSchedPid failed", "error", err)
	}

	err = cache.ImportScxEnums()
	if err != nil {
		slog.Warn("GetScxEnums failed", "error", err)
	}

	bpfModule.Start()

	topo, err := cache.GetTopology()
	if err != nil {
		return fmt.Errorf("GetTopology failed: %w", err)
	}
	slog.Info("Topology", "topology", topo)

	err = cache.InitCacheDomains(bpfModule)
	if err != nil {
		return fmt.Errorf("InitCacheDomains failed: %w", err)
	}

	if err := bpfModule.Attach(); err != nil {
		return fmt.Errorf("bpfModule attach failed: %w", err)
	}

	slog.Info("UserSched's Pid", "pid", core.GetUserSchedPid())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	cfg.Api.Interval = policy.NormalizeAPIInterval(cfg.Api.Interval, cfg.Api.Enabled)
	oldBss, err := bpfModule.GetBssData()
	if err != nil {
		slog.Warn("GetBssData failed", "error", err)
	}
	timer := time.NewTicker(time.Duration(cfg.Api.Interval) * time.Second)
	cont := true
	go func() {
		defer timer.Stop()
		for cont {
			select {
			case <-ctx.Done():
				slog.Info("context done, exiting signal handler")
				return
			case <-signalChan:
				slog.Info("receive os signal")
				cont = false
			case <-timer.C:
				bss, err := bpfModule.GetBssData()
				if oldBss.Nr_kernel_dispatches == bss.Nr_kernel_dispatches {
					if bpfModule.Stopped() {
						slog.Info("No progress detected and scheduler stopped, exiting")
						cont = false
					}
				}
				oldBss = bss
				bss.Nr_scheduled = bpfModule.GetPoolCount()
				if err != nil {
					slog.Warn("GetBssData failed", "error", err)
				} else {
					b, err := json.Marshal(bss)
					if err != nil {
						slog.Warn("json.Marshal failed", "error", err)
					} else {
						slog.Info("bss data", "data", string(b))
						if cfg.Api.Enabled {
							metricsData := gthulhu.BssData{
								UserschedLastRunAt: bss.Usersched_last_run_at,
								NrQueued:           bss.Nr_queued,
								NrScheduled:        bss.Nr_scheduled,
								NrRunning:          bss.Nr_running,
								NrOnlineCpus:       bss.Nr_online_cpus,
								NrUserDispatches:   bss.Nr_user_dispatches,
								NrKernelDispatches: bss.Nr_kernel_dispatches,
								NrCancelDispatches: bss.Nr_cancel_dispatches,
								NrBounceDispatches: bss.Nr_bounce_dispatches,
								NrFailedDispatches: bss.Nr_failed_dispatches,
								NrSchedCongested:   bss.Nr_sched_congested,
							}
							p.SendMetrics(metricsData)
						}
					}
				}
			}
		}
		cancel()
		uei, err := bpfModule.GetUeiData()
		if err == nil {
			slog.Info("uei", "kind", uei.Kind, "exitCode", uei.ExitCode, "reason", uei.GetReason(), "message", uei.GetMessage())
		} else {
			slog.Warn("GetUeiData failed", "error", err)
		}
	}()

	slog.Info("scheduler started")

	if cfg.IsDebugEnabled() {
		go func() {
			if err := http.ListenAndServe(":6060", nil); err != nil {
				slog.Warn("pprof server error", "error", err)
			}
		}()
	}

	if cfg.Scheduler.KernelMode {
		for {
			changed, removed := p.GetChangedStrategies()
			if len(changed) > 0 || len(removed) > 0 {
				for _, strategy := range changed {
					err = bpfModule.UpdatePriorityTaskWithPrio(uint32(strategy.PID), strategy.ExecutionTime, uint32(strategy.Priority))
					if err != nil {
						slog.Warn("UpdatePriorityTaskWithPrio failed", "error", err, "pid", strategy.PID)
					} else {
						slog.Info("Updated priority task", "pid", strategy.PID, "executionTime", strategy.ExecutionTime, "priority", strategy.Priority)
					}
				}
				for _, strategy := range removed {
					err = bpfModule.RemovePriorityTask(uint32(strategy.PID))
					if err != nil {
						slog.Warn("RemovePriorityTask failed", "error", err, "pid", strategy.PID)
					} else {
						slog.Info("Removed priority task", "pid", strategy.PID)
					}
				}
			}
			if bpfModule.Stopped() {
				uei, err := bpfModule.GetUeiData()
				if err == nil {
					slog.Info("uei", "kind", uei.Kind, "exitCode", uei.ExitCode, "reason", uei.GetReason(), "message", uei.GetMessage())
				} else {
					slog.Warn("GetUeiData failed", "error", err)
				}
				return nil
			}
			select {
			case <-ctx.Done():
				slog.Info("context done, exiting kernel mode scheduler loop")
				return nil
			default:
			}
			time.Sleep(1 * time.Second)
		}
	}

	if err = runSchedulerLoop(ctx, bpfModule, sliceNsDefault, sliceNsMin); err != nil {
		slog.Info("Scheduler loop exited with error", "error", err)
		uei, err := bpfModule.GetUeiData()
		if err == nil {
			slog.Info("uei", "kind", uei.Kind, "exitCode", uei.ExitCode, "reason", uei.GetReason(), "message", uei.GetMessage())
		} else {
			slog.Warn("GetUeiData failed", "error", err)
		}
		cancel()
	}

	slog.Info("scheduler exit")
	return nil
}
