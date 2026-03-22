package domain

// PodSchedulingMetricValue represents the latest collected scheduling metrics for a pod.
type PodSchedulingMetricValue struct {
	Namespace              string `json:"namespace"`
	PodName                string `json:"podName"`
	NodeID                 string `json:"nodeID,omitempty"`
	VoluntaryCtxSwitches   uint64 `json:"voluntaryCtxSwitches"`
	InvoluntaryCtxSwitches uint64 `json:"involuntaryCtxSwitches"`
	CPUTimeNs              uint64 `json:"cpuTimeNs"`
	WaitTimeNs             uint64 `json:"waitTimeNs"`
	RunCount               uint64 `json:"runCount"`
	CPUMigrations          uint64 `json:"cpuMigrations"`
}

// PodSchedulingMetricValuesResult is the aggregated runtime metrics view returned by manager.
type PodSchedulingMetricValuesResult struct {
	Items    []*PodSchedulingMetricValue `json:"items"`
	Warnings []string                    `json:"warnings,omitempty"`
}
