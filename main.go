package main

import (
	"log"
	"os"
	"os/exec"
	"os/signal"
	"sync"
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
	PF_WQ_WORKER       = 0x00000020
)

const taskPoolSize = 4096

var taskPool = make([]Task, taskPoolSize)
var taskPoolCount = 0
var taskPoolHead, taskPoolTail int

func DrainQueuedTask(s *core.Sched) int {
	var count int
	for (taskPoolTail+1)%taskPoolSize != taskPoolHead {
		var newQueuedTask core.QueuedTask
		s.DequeueTask(&newQueuedTask)
		if newQueuedTask.Pid == -1 {
			return count
		}
		updatedEnqueueTask(s, &newQueuedTask)
		mapLock.RLock()
		t := Task{
			QueuedTask: &newQueuedTask,
			Deadline:   taskInfoMap[newQueuedTask.Pid].vruntime,
			Timestamp:  taskInfoMap[newQueuedTask.Pid].nvcswTs,
		}
		mapLock.RUnlock()
		InsertTaskToPool(t)
		count++
	}
	return 0
}

var timeout = uint64(3 * NSEC_PER_SEC)

func updatedEnqueueTask(s *core.Sched, t *core.QueuedTask) {
	var timeStp, deltaT, avgNvcsw, deltaNvcsw, slice,
		minVruntimeLimit, weightMultiplier, baseWeight,
		latencyWeight, vslice uint64
	var info *TaskInfo
	var exists bool
	timeStp = now()
	mapLock.Lock()
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
	mapLock.Unlock()

	deltaT = timeStp - info.nvcswTs
	if deltaT >= NSEC_PER_SEC {
		deltaNvcsw = t.Nvcsw - info.nvcsw
		avgNvcsw = uint64(0)
		avgNvcsw = min(deltaNvcsw*NSEC_PER_SEC/deltaT, 1000)
		info.nvcsw = t.Nvcsw
		info.nvcswTs = timeStp
		info.avgNvcsw = calcAvg(info.avgNvcsw, avgNvcsw)
	}

	// Evaluate used task time slice.
	slice = min(
		saturating_sub(t.SumExecRuntime, info.prevExecRuntime),
		SLICE_NS_DEFAULT,
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

	minVruntimeLimit = saturating_sub(minVruntime, SLICE_NS_DEFAULT*latencyWeight)
	if info.vruntime < minVruntimeLimit {
		info.vruntime = minVruntimeLimit
	}
	vslice = slice * 100 / t.Weight
	info.vruntime += vslice
	minVruntime += vslice
}

func GetTaskFromPool() *core.QueuedTask {
	if taskPoolHead == taskPoolTail {
		return nil
	}
	t := &taskPool[taskPoolHead]
	taskPoolHead = (taskPoolHead + 1) % taskPoolSize
	taskPoolCount--
	return t.QueuedTask
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
var mapLock sync.RWMutex
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

type Task struct {
	*core.QueuedTask
	Deadline  uint64
	Timestamp uint64
}

func LessQueuedTask(a, b *Task) bool {
	if a.Deadline != b.Deadline {
		return a.Deadline < b.Deadline
	}
	if a.Timestamp != b.Timestamp {
		return a.Timestamp < b.Timestamp
	}
	return a.Pid < b.Pid
}

func InsertTaskToPool(newTask Task) bool {
	if taskPoolCount >= taskPoolSize-1 {
		return false
	}
	insertIdx := taskPoolTail
	for i := 0; i < taskPoolCount; i++ {
		idx := (taskPoolHead + i) % taskPoolSize
		if LessQueuedTask(&newTask, &taskPool[idx]) {
			insertIdx = idx
			break
		}
	}

	cur := taskPoolTail
	for cur != insertIdx {
		next := (cur - 1 + taskPoolSize) % taskPoolSize
		taskPool[cur] = taskPool[next]
		cur = next
	}
	taskPool[insertIdx] = newTask
	taskPoolTail = (taskPoolTail + 1) % taskPoolSize
	taskPoolCount++
	return true
}

func main() {
	bpfModule := core.LoadSched("main.bpf.o")
	defer bpfModule.Close()
	pid := os.Getpid()
	err := bpfModule.AssignUserSchedPid(pid)
	if err != nil {
		log.Printf("AssignUserSchedPid failed: %v", err)
	}

	err = util.InitCacheDomains(bpfModule)
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
				mapLock.Lock()
				delete(taskInfoMap, int32(pid))
				mapLock.Unlock()
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
		var info *TaskInfo

		for true {
			t = GetTaskFromPool()
			if t == nil {
				for uint64(taskPoolCount) < 10 {
					if num := DrainQueuedTask(bpfModule); num == 0 {
						bpfModule.BlockTilReadyForDequeue()
					}
				}
			} else if t.Pid != -1 {
				task = core.NewDispatchedTask(t)
				err, cpu = bpfModule.SelectCPU(t)
				if err != nil {
					log.Printf("SelectCPU failed: %v", err)
				}

				mapLock.RLock()
				info = taskInfoMap[t.Pid]
				mapLock.RUnlock()
				// Evaluate used task time slice.
				nrWaiting := core.GetNrQueued() + core.GetNrScheduled() + 1
				task.Vtime = info.vruntime
				task.SliceNs = max(SLICE_NS_DEFAULT/nrWaiting, SLICE_NS_MIN)
				task.Cpu = cpu

				err = bpfModule.DispatchTask(task)
				if err != nil {
					log.Printf("DispatchTask failed: %v", err)
					continue
				}

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
