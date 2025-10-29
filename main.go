package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gthulhu/Gthulhu/internal/config"
	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin"
	"github.com/Gthulhu/plugin/plugin/gthulhu"
	core "github.com/Gthulhu/qumun/goland_core"
	cache "github.com/Gthulhu/qumun/util"
)

func main() {
	// Initialize structured logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Parse command line flags
	configFile := flag.String("config", "", "Path to YAML configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		panic(err)
	}

	// Apply scheduler configuration before loading eBPF program
	schedConfig := cfg.GetSchedulerConfig()

	var p plugin.CustomScheduler
	var SLICE_NS_DEFAULT, SLICE_NS_MIN uint64

	ctx, cancel := context.WithCancel(context.Background())
	config := &plugin.SchedConfig{
		Mode: schedConfig.Mode,
		Scheduler: plugin.Scheduler{
			SliceNsDefault: cfg.Scheduler.SliceNsDefault,
			SliceNsMin:     cfg.Scheduler.SliceNsMin,
		},
		APIConfig: plugin.APIConfig{
			BaseURL:       cfg.Api.Url,
			Interval:      cfg.Api.Interval,
			PublicKeyPath: cfg.Api.PublicKeyPath,
		},
	}
	if config.Mode == "" {
		config.Mode = "gthulhu"
	}
	p, err = plugin.NewSchedulerPlugin(ctx, config)
	if err != nil {
		slog.Error("Failed to create plugin", "error", err)
		os.Exit(1)
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

	if cfg.EarlyProcessing {
		slog.Info("Early processing enabled")
		bpfModule.SetEarlyProcessing(true)
	} else {
		slog.Info("Early processing disabled")
	}

	bpfModule.SetDefaultSlice(schedConfig.SliceNsDefault)

	pid := os.Getpid()
	err = bpfModule.AssignUserSchedPid(pid)
	if err != nil {
		slog.Warn("AssignUserSchedPid failed", "error", err)
	}
	bpfModule.Start()

	topo, err := cache.GetTopology()
	if err != nil {
		slog.Error("GetTopology failed", "error", err)
		panic(err)
	}
	slog.Info("Topology", "topology", topo)

	err = cache.InitCacheDomains(bpfModule)
	if err != nil {
		slog.Error("InitCacheDomains failed", "error", err)
		panic(err)
	}

	if err := bpfModule.Attach(); err != nil {
		slog.Error("bpfModule attach failed", "error", err)
		panic(err)
	}

	slog.Info("UserSched's Pid", "pid", core.GetUserSchedPid())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	timer := time.NewTicker(1 * time.Second)
	notifyCount := 0
	cont := true
	go func() {
		for cont {
			select {
			case <-signalChan:
				slog.Info("receive os signal")
				cont = false
			case <-timer.C:
				notifyCount++
				if notifyCount%10 == 0 {
					bss, err := bpfModule.GetBssData()
					if err != nil {
						slog.Warn("GetBssData failed", "error", err)
					} else {
						b, err := json.Marshal(bss)
						if err != nil {
							slog.Warn("json.Marshal failed", "error", err)
						} else {
							slog.Info("bss data", "data", string(b))

							// Send metrics to API server if metrics client is available
							// Convert BSS data to metrics format
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
				if bpfModule.Stopped() {
					slog.Info("bpfModule stopped")
					cont = false
				}
			}
		}
		cancel()
		timer.Stop()
		uei, err := bpfModule.GetUeiData()
		if err == nil {
			slog.Info("uei", "kind", uei.Kind, "exitCode", uei.ExitCode, "reason", uei.GetReason(), "message", uei.GetMessage())
		} else {
			slog.Warn("GetUeiData failed", "error", err)
		}
	}()

	var t *models.QueuedTask
	var task *core.DispatchedTask
	var cpu int32

	slog.Info("scheduler started")

	for true {
		select {
		case <-ctx.Done():
			slog.Info("context done, exiting scheduler loop")
			return
		default:
		}
		bpfModule.DrainQueuedTask()
		t = bpfModule.SelectQueuedTask()
		if t == nil {
			bpfModule.BlockTilReadyForDequeue(ctx)
		} else if t.Pid != -1 {
			task = core.NewDispatchedTask(t)

			// Evaluate used task time slice.
			nrWaiting := core.GetNrQueued() + core.GetNrScheduled() + 1
			task.Vtime = t.Vtime

			// Check if a custom execution time was set by a scheduling strategy
			customTime := bpfModule.DetermineTimeSlice(t)
			if customTime > 0 {
				// Use the custom execution time from the scheduling strategy
				task.SliceNs = min(customTime, (t.StopTs-t.StartTs)*11/10)
			} else {
				// No custom execution time, use default algorithm
				task.SliceNs = max(SLICE_NS_DEFAULT/nrWaiting, SLICE_NS_MIN)
			}

			err, cpu = bpfModule.SelectCPU(t)
			if err != nil {
				slog.Warn("SelectCPU failed", "error", err)
			}
			task.Cpu = cpu

			err = bpfModule.DispatchTask(task)
			if err != nil {
				slog.Warn("DispatchTask failed", "error", err)
				continue
			}

			err = core.NotifyComplete(bpfModule.GetPoolCount())
			if err != nil {
				slog.Warn("NotifyComplete failed", "error", err)
			}
		}
	}

	slog.Info("scheduler exit")
}
