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
	"github.com/Gthulhu/Gthulhu/internal/sched"
	core "github.com/Gthulhu/scx_goland_core/goland_core"
	cache "github.com/Gthulhu/scx_goland_core/util"
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
	sched.SetSchedulerConfig(
		cfg.Scheduler.SliceNsDefault,
		cfg.Scheduler.SliceNsMin,
	)

	log.Printf("Scheduler config: SLICE_NS_DEFAULT=%d, SLICE_NS_MIN=%d",
		sched.SLICE_NS_DEFAULT, sched.SLICE_NS_MIN)

	bpfModule := core.LoadSched("main.bpf.o")
	defer bpfModule.Close()

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
	ctx, cancel := context.WithCancel(context.Background())

	// Start scheduling strategy fetcher
	apiConfig := cfg.GetApiConfig()
	if apiConfig.Enabled {
		apiUrl := apiConfig.Url + "/api/v1/scheduling/strategies"
		log.Printf("API config: URL=%s, Interval=%d seconds", apiUrl, apiConfig.Interval)
		sched.StartStrategyFetcher(ctx, apiUrl, time.Duration(apiConfig.Interval)*time.Second)
		log.Printf("Started scheduling strategy fetcher with interval %d seconds", apiConfig.Interval)
	}

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

	var t *core.QueuedTask
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
		sched.DrainQueuedTask(bpfModule)
		t = sched.GetTaskFromPool()
		if t == nil {
			bpfModule.BlockTilReadyForDequeue(ctx)
		} else if t.Pid != -1 {
			task = core.NewDispatchedTask(t)

			// Evaluate used task time slice.
			nrWaiting := core.GetNrQueued() + core.GetNrScheduled() + 1
			task.Vtime = t.Vtime

			// Check if a custom execution time was set by a scheduling strategy
			customTime := sched.GetTaskExecutionTime(t.Pid)
			if customTime > 0 {
				// Use the custom execution time from the scheduling strategy
				task.SliceNs = min(customTime, (t.StopTs-t.StartTs)*11/10)
			} else {
				// No custom execution time, use default algorithm
				task.SliceNs = max(sched.SLICE_NS_DEFAULT/nrWaiting, sched.SLICE_NS_MIN)
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

			err = core.NotifyComplete(uint64(sched.GetPoolCount()))
			if err != nil {
				log.Printf("NotifyComplete failed: %v", err)
			}
		}
	}

	log.Println("scheduler exit")
}
