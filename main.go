package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Gthulhu/Gthulhu/internal/sched"
	core "github.com/Gthulhu/scx_goland_core/goland_core"
	cache "github.com/Gthulhu/scx_goland_core/util"
)

func main() {
	bpfModule := core.LoadSched("main.bpf.o")
	defer bpfModule.Close()
	pid := os.Getpid()
	err := bpfModule.AssignUserSchedPid(pid)
	if err != nil {
		log.Printf("AssignUserSchedPid failed: %v", err)
	}

	err = cache.InitCacheDomains(bpfModule)
	if err != nil {
		log.Panicf("InitCacheDomains failed: %v", err)
	}

	if err := bpfModule.Attach(); err != nil {
		log.Panicf("bpfModule attach failed: %v", err)
	}

	log.Printf("UserSched's Pid: %v", core.GetUserSchedPid())
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			if pid := bpfModule.ReceiveProcExitEvt(); pid != -1 {
				sched.DeletePidFromTaskInfo(pid)
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

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
	var info *sched.TaskInfo

	log.Println("scheduler started")

	for true {
		select {
		case <-ctx.Done():
			log.Println("context done, exiting scheduler loop")
			return
		default:
		}
		t = sched.GetTaskFromPool()
		if t == nil {
			for sched.GetPoolCount() < 10 {
				if num := sched.DrainQueuedTask(bpfModule); num == 0 {
					// prevent deadlock if no tasks are available (bpfmodule unloaded)
					select {
					case <-ctx.Done():
						log.Println("context done, exiting scheduler loop")
						return
					default:
					}
					bpfModule.BlockTilReadyForDequeue(ctx)
				}
			}
		} else if t.Pid != -1 {
			task = core.NewDispatchedTask(t)
			err, cpu = bpfModule.SelectCPU(t)
			if err != nil {
				log.Printf("SelectCPU failed: %v", err)
			}

			info, _ = sched.GetTaskInfo(t.Pid)
			// Evaluate used task time slice.
			nrWaiting := core.GetNrQueued() + core.GetNrScheduled() + 1
			task.Vtime = info.Vruntime
			task.SliceNs = max(sched.SLICE_NS_DEFAULT/nrWaiting, sched.SLICE_NS_MIN)
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
