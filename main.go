package main

import (
	"log"
	"os"
	"os/exec"
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

	go func() {
		for {
			if pid := bpfModule.ReceiveProcExitEvt(); pid != -1 {
				sched.DeletePidFromTaskInfo(pid)
			} else {
				time.Sleep(100 * time.Millisecond)
			}
		}
	}()

	go func() {
		var t *core.QueuedTask
		var task *core.DispatchedTask
		var err error
		var cpu int32
		var info *sched.TaskInfo

		for true {
			t = sched.GetTaskFromPool()
			if t == nil {
				for sched.GetPoolCount() < 10 {
					if num := sched.DrainQueuedTask(bpfModule); num == 0 {
						bpfModule.BlockTilReadyForDequeue()
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

				err = core.NotifyCompleteSkel(uint64(sched.GetPoolCount()))
				if err != nil {
					log.Printf("NotifyComplete failed: %v", err)
				}
			}
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	cont := true
	timer := time.NewTicker(1 * time.Second)
	for cont {
		select {
		case <-signalChan:
			log.Println("receive os signal")
			cont = false
		case <-timer.C:
			if bpfModule.Stopped() {
				log.Println("bpfModule stopped")
				cmd := exec.Command("bpftool", []string{"map", "dump", "name", "main_bpf.data"}...)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					log.Printf("bpftool map dump failed: %v", err)
				}
				cont = false
			}
		}
	}
	timer.Stop()
	log.Println("scheduler exit")
}
