package sched

import (
	"sync"

	"github.com/Gthulhu/Gthulhu/util"
	core "github.com/Gthulhu/scx_goland_core/goland_core"
)

const (
	MAX_LATENCY_WEIGHT = 1000
	SCX_ENQ_WAKEUP     = 1
	NSEC_PER_SEC       = 1000000000 // 1 second in nanoseconds
	PF_WQ_WORKER       = 0x00000020
)

// Configurable scheduler parameters
var (
	SLICE_NS_DEFAULT uint64 = 5000 * 1000 // 5ms (default)
	SLICE_NS_MIN     uint64 = 500 * 1000  // 0.5ms (default)
)

const taskPoolSize = 4096

var taskPool = make([]Task, taskPoolSize)
var taskPoolCount = 0
var taskPoolHead, taskPoolTail int

func DeletePidFromTaskInfo(pid int) {
	mapLock.Lock()
	defer mapLock.Unlock()
	if _, exists := taskInfoMap[int32(pid)]; exists {
		delete(taskInfoMap, int32(pid))
	}
}

func GetTaskInfo(pid int32) (*TaskInfo, bool) {
	mapLock.RLock()
	defer mapLock.RUnlock()
	info, exists := taskInfoMap[pid]
	if !exists {
		return nil, false
	}
	return info, true
}

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
			Deadline:   taskInfoMap[newQueuedTask.Pid].Vruntime,
			Timestamp:  taskInfoMap[newQueuedTask.Pid].nvcswTs,
		}
		mapLock.RUnlock()
		InsertTaskToPool(t)
		count++
	}
	return 0
}

func updatedEnqueueTask(s *core.Sched, t *core.QueuedTask) {
	var timeStp, deltaT, avgNvcsw, deltaNvcsw, slice,
		minVruntimeLimit, weightMultiplier, baseWeight,
		latencyWeight, vslice uint64
	var info *TaskInfo
	var exists bool
	timeStp = util.Now()
	mapLock.Lock()
	info, exists = taskInfoMap[t.Pid]
	if !exists {
		info = &TaskInfo{
			prevExecRuntime: t.SumExecRuntime,
			Vruntime:        minVruntime,
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
		info.avgNvcsw = util.CalcAvg(info.avgNvcsw, avgNvcsw)
	}

	// Evaluate used task time slice.
	slice = min(
		util.SaturatingSub(t.SumExecRuntime, info.prevExecRuntime),
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

	minVruntimeLimit = util.SaturatingSub(minVruntime, SLICE_NS_DEFAULT*latencyWeight)
	if info.Vruntime < minVruntimeLimit {
		info.Vruntime = minVruntimeLimit
	}
	vslice = slice * 100 / t.Weight
	info.Vruntime += vslice
	minVruntime += vslice
}

func GetPoolCount() int {
	return taskPoolCount
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

// SetSchedulerConfig updates the scheduler parameters from configuration
func SetSchedulerConfig(sliceNsDefault, sliceNsMin uint64) {
	if sliceNsDefault > 0 {
		SLICE_NS_DEFAULT = sliceNsDefault
	}
	if sliceNsMin > 0 {
		SLICE_NS_MIN = sliceNsMin
	}
}

// GetSchedulerConfig returns current scheduler configuration
func GetSchedulerConfig() (uint64, uint64) {
	return SLICE_NS_DEFAULT, SLICE_NS_MIN
}

// TaskInfo stores task statistics
type TaskInfo struct {
	sumExecRuntime  uint64
	prevExecRuntime uint64
	Vruntime        uint64
	avgNvcsw        uint64
	nvcsw           uint64
	nvcswTs         uint64
}

var taskInfoMap = make(map[int32]*TaskInfo)
var mapLock sync.RWMutex
var minVruntime uint64 = 0 // global vruntime

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
	return a.QueuedTask.Pid < b.QueuedTask.Pid
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
