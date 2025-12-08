package main

import (
	"context"
	"encoding/json"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/Gthulhu/Gthulhu/internal/config"
	"github.com/Gthulhu/plugin/models"
	"github.com/Gthulhu/plugin/plugin"
	"github.com/Gthulhu/plugin/plugin/gthulhu"
	core "github.com/Gthulhu/qumun/goland_core"
	cache "github.com/Gthulhu/qumun/util"
)

func main() {
	runtime.GOMAXPROCS(1)
	// Initialize structured logger
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
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
	SLICE_NS_DEFAULT = cfg.Scheduler.SliceNsDefault
	SLICE_NS_MIN = cfg.Scheduler.SliceNsMin
	slog.Info("Scheduler configuration", "SliceNsDefault", SLICE_NS_DEFAULT, "SliceNsMin", SLICE_NS_MIN)

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
			Enabled:       cfg.Api.Enabled,
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
	if (cfg.Api.Interval <= 0) || (!cfg.Api.Enabled) {
		cfg.Api.Interval = 5
	}
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
		// Start pprof server for debugging
		go func() {
			http.ListenAndServe(":6060", nil)
		}()
	}

	if err = runSchedulerLoop(ctx, bpfModule, p, SLICE_NS_DEFAULT, SLICE_NS_MIN); err != nil {
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
}

func runSchedulerLoop(
	ctx context.Context,
	bpfModule *core.Sched,
	p plugin.CustomScheduler,
	SLICE_NS_DEFAULT,
	SLICE_NS_MIN uint64,
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

		// Drain all pending tasks from ringbuf (like scx_rustland)
		cnt := bpfModule.DrainQueuedTask()
		if cnt > 0 {
			err = bpfModule.DecNrQueued(cnt)
			if err != nil {
				slog.Warn("DecNrQueued failed", "error", err)
				return err
			}
		}

		// Dispatch ONE task per iteration (like scx_rustland)
		// This ensures low-latency response for newly enqueued tasks
		t = bpfModule.SelectQueuedTask()
		if t == nil {
			bpfModule.BlockTilReadyForDequeue(ctx)
		} else {
			task = core.NewDispatchedTask(t)
			// Deadline calculation:
			// deadline = vtime + min(exec_runtime, 100 * slice_ns)
			task.Vtime = t.Vtime
			if t.Vtime != 0 {
				task.Vtime += min(t.SumExecRuntime, SLICE_NS_DEFAULT*100)
			}

			// Check if a custom execution time was set by a scheduling strategy
			customTime := bpfModule.DetermineTimeSlice(t)
			if customTime > 0 {
				// Use the custom execution time from the scheduling strategy
				task.SliceNs = min(customTime, (t.StopTs-t.StartTs)*11/10)
			} else {
				// Assign minimum time slice scaled by task weight
				task.SliceNs = SLICE_NS_MIN * t.Weight / 100
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

			// Notify completion with pending task count
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
