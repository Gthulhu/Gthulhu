package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	core "github.com/Gthulhu/scx_goland_core/goland_core"
	"github.com/Gthulhu/scx_goland_core/util"
)

const (
	MAX_LATENCY_WEIGHT = 1000
	SLICE_NS_DEFAULT   = 5000 * 1000 // 5ms
	SLICE_NS_MIN       = 500 * 1000
	SCX_ENQ_WAKEUP     = 1
	NSEC_PER_SEC       = 1000000000 // 1 second in nanoseconds
)

const taskPoolSize = 4096

var taskPool = make([]core.QueuedTask, taskPoolSize)
var taskPoolCount = 0
var taskPoolHead, taskPoolTail int

func DrainQueuedTask(s *core.Sched) int {
	var count int
	for (taskPoolTail+1)%taskPoolSize != taskPoolHead {
		s.DequeueTask(&taskPool[taskPoolTail])
		if taskPool[taskPoolTail].Pid == -1 {
			return count
		}
		taskPoolTail = (taskPoolTail + 1) % taskPoolSize
		taskPoolCount++
		count++
	}
	return 0
}

func GetTaskFromPool() *core.QueuedTask {
	if taskPoolHead == taskPoolTail {
		return nil
	}
	t := &taskPool[taskPoolHead]
	taskPoolHead = (taskPoolHead + 1) % taskPoolSize
	taskPoolCount--
	return t
}

// TaskInfo stores task statistics
type TaskInfo struct {
	sumExecRuntime  uint64
	prevExecRuntime uint64
	vruntime        uint64
	avgNvcsw        uint64
	nvcsw           uint64
	nvcswTs         uint64
}

var taskInfoMap = make(map[int32]*TaskInfo)
var minVruntime uint64 = 0 // global vruntime

func now() uint64 {
	return uint64(time.Now().UnixNano())
}

func calcAvg(oldVal uint64, newVal uint64) uint64 {
	return (oldVal - (oldVal >> 2)) + (newVal >> 2)
}

func saturating_sub(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}

func main() {
	bpfModule := core.LoadSched("main.bpf.o")
	defer bpfModule.Close()
	pid := os.Getpid()
	err := bpfModule.AssignUserSchedPid(pid)
	if err != nil {
		log.Printf("AssignUserSchedPid failed: %v", err)
	}
	log.Printf("pid: %v", pid)

	err = util.InitCacheDomains(bpfModule)
	if err != nil {
		log.Panicf("InitCacheDomains failed: %v", err)
	}

	if err := bpfModule.Attach(); err != nil {
		log.Panicf("bpfModule attach failed: %v", err)
	}

	log.Printf("GetUserSchedPid: %v", core.GetUserSchedPid())

	go func() {
		var t *core.QueuedTask
		var task *core.DispatchedTask
		var err error
		var cpu int32
		var timeStp, deltaT, avgNvcsw, deltaNvcsw, slice,
			minVruntimeLimit, weightMultiplier, baseWeight,
			latencyWeight, nrWaiting, sliceNs, vslice uint64
		var info *TaskInfo
		var exists bool

		sleepCnt := time.Duration(1)

		for true {
			t = GetTaskFromPool()
			if t == nil {
				if num := DrainQueuedTask(bpfModule); num == 0 {
					// No tasks in the pool, wait for new tasks.
					time.Sleep(100 * time.Microsecond)
					sleepCnt++
					continue
				} else {
					sleepCnt = 1
				}
			} else {
				task = core.NewDispatchedTask(t)
				err, cpu = bpfModule.SelectCPU(t)
				if err != nil {
					log.Printf("SelectCPU failed: %v", err)
				}

				timeStp = now()
				info, exists = taskInfoMap[t.Pid]
				if !exists {
					info = &TaskInfo{
						prevExecRuntime: t.SumExecRuntime,
						vruntime:        minVruntime,
						nvcsw:           t.Nvcsw,
						nvcswTs:         timeStp,
					}
					taskInfoMap[t.Pid] = info
				}

				deltaT = timeStp - info.nvcswTs
				if deltaT >= NSEC_PER_SEC {
					deltaNvcsw = t.Nvcsw - info.nvcsw
					avgNvcsw = uint64(0)
					if deltaT > 0 {
						avgNvcsw = min(deltaNvcsw*NSEC_PER_SEC/deltaT, 1000)
					}
					info.nvcsw = t.Nvcsw
					info.nvcswTs = timeStp
					info.avgNvcsw = calcAvg(info.avgNvcsw, avgNvcsw)
				}

				// Evaluate used task time slice.
				nrWaiting = core.GetNrQueued() + core.GetNrScheduled() + 1
				sliceNs = max(SLICE_NS_DEFAULT/nrWaiting, SLICE_NS_MIN)
				task.SliceNs = sliceNs

				// Evaluate used task time slice.
				slice = min(
					saturating_sub(t.SumExecRuntime, info.prevExecRuntime),
					sliceNs,
				)
				// Update total task cputime.
				info.prevExecRuntime = t.SumExecRuntime

				// Update task's vruntime re-aligning it to min_vruntime.
				//
				// The amount of vruntime budget an idle task can accumulate is adjusted in function of its
				// latency weight, which is derived from the average number of voluntary context switches.
				// This ensures that latency-sensitive tasks receive a priority boost.
				baseWeight = min(info.avgNvcsw, MAX_LATENCY_WEIGHT)
				weightMultiplier = uint64(1)
				if t.Flags&SCX_ENQ_WAKEUP != 0 {
					weightMultiplier = 2
				}
				latencyWeight = (baseWeight * weightMultiplier) + 1

				minVruntimeLimit = saturating_sub(minVruntime, sliceNs*latencyWeight)

				if info.vruntime < minVruntimeLimit {
					info.vruntime = minVruntimeLimit
				}
				vslice = slice * 100 / t.Weight
				info.vruntime += vslice
				minVruntime += vslice
				task.Vtime = info.vruntime
				task.Cpu = cpu

				bpfModule.DispatchTask(task)

				err = core.NotifyCompleteSkel(uint64(taskPoolCount))
				if err != nil {
					log.Printf("NotifyComplete failed: %v", err)
				}
			}
		}
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	cont := true
	for cont {
		select {
		case <-signalChan:
			log.Println("receive os signal")
			cont = false
		default:
			if bpfModule.Stopped() {
				log.Println("bpfModule stopped")
				cont = false
			} else {
				time.Sleep(1 * time.Second)
			}
		}
	}

	log.Println("scheduler exit")
}
