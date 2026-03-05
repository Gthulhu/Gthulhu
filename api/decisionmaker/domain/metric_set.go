package domain

type MetricSet struct {
	UserSchedLastRunAt uint64
	NrQueued           uint64
	NrScheduled        uint64
	NrRunning          uint64
	NrOnlineCPUs       uint64
	NrUserDispatches   uint64
	NrKernelDispatches uint64
	NrCancelDispatches uint64
	NrBounceDispatches uint64
	NrFailedDispatches uint64
	NrSchedCongested   uint64
}
