package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
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
	// Parse command line flags
	configFile := flag.String("config", "", "Path to YAML configuration file")
	flag.Parse()

	// Load configuration
	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Panicf("Failed to load configuration: %v", err)
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
		log.Fatalf("Failed to create plugin: %v", err)
	}

	bpfModule := core.LoadSched("main.bpf.o")
	defer bpfModule.Close()

	bpfModule.SetPlugin(p)

	if cfg.IsDebugEnabled() {
		log.Println("Debug mode enabled")
		bpfModule.SetDebug(true)
	}

	if cfg.IsBuiltinIdleEnabled() {
		log.Println("Built-in idle CPU selection enabled")
		bpfModule.SetBuiltinIdle(true)
	}

	if cfg.EarlyProcessing {
		log.Println("Early processing enabled")
		bpfModule.SetEarlyProcessing(true)
	} else {
		log.Println("Early processing disabled")
	}

	bpfModule.SetDefaultSlice(schedConfig.SliceNsDefault)

	pid := os.Getpid()
	err = bpfModule.AssignUserSchedPid(pid)
	if err != nil {
		log.Printf("AssignUserSchedPid failed: %v", err)
	}
	bpfModule.Start()

	topo, err := cache.GetTopology()
	if err != nil {
		log.Panicf("GetTopology failed: %v", err)
	}
	log.Printf("Topology: %v", topo)

	err = cache.InitCacheDomains(bpfModule)
	if err != nil {
		log.Panicf("InitCacheDomains failed: %v", err)
	}

	if err := bpfModule.Attach(); err != nil {
		log.Panicf("bpfModule attach failed: %v", err)
	}

	log.Printf("UserSched's Pid: %v", core.GetUserSchedPid())

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	timer := time.NewTicker(1 * time.Second)
	notifyCount := 0
	cont := true
	go func() {
		for cont {
			select {
			case <-signalChan:
				log.Println("receive os signal")
				cont = false
			case <-timer.C:
				notifyCount++
				if notifyCount%10 == 0 {
					bss, err := bpfModule.GetBssData()
					if err != nil {
						log.Printf("GetBssData failed: %v", err)
					} else {
						b, err := json.Marshal(bss)
						if err != nil {
							log.Printf("json.Marshal failed: %v", err)
						} else {
							log.Printf("bss data: %s", string(b))

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
					log.Println("bpfModule stopped")
					cont = false
				}
			}
		}
		cancel()
		timer.Stop()
		uei, err := bpfModule.GetUeiData()
		if err == nil {
			log.Printf("uei: kind=%d, exitCode=%d, reason=%s, message=%s",
				uei.Kind, uei.ExitCode, uei.GetReason(), uei.GetMessage())
		} else {
			log.Printf("GetUeiData failed: %v", err)
		}
	}()

	var t *models.QueuedTask
	var task *core.DispatchedTask
	var cpu int32

	log.Println("scheduler started")

	for true {
		select {
		case <-ctx.Done():
			log.Println("context done, exiting scheduler loop")
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
				log.Printf("SelectCPU failed: %v", err)
			}
			task.Cpu = cpu

			err = bpfModule.DispatchTask(task)
			if err != nil {
				log.Printf("DispatchTask failed: %v", err)
				continue
			}

			err = core.NotifyComplete(bpfModule.GetPoolCount())
			if err != nil {
				log.Printf("NotifyComplete failed: %v", err)
			}
		}
	}

	log.Println("scheduler exit")
}
