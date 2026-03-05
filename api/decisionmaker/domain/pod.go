package domain

// PodProcess represents a process information within a pod
type PodProcess struct {
	PID         int    `json:"pid"`
	Command     string `json:"command"`
	PPID        int    `json:"ppid,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
}

// PodInfo represents pod information with associated processes
type PodInfo struct {
	PodUID    string       `json:"pod_uid"`
	PodID     string       `json:"pod_id,omitempty"`
	Processes []PodProcess `json:"processes"`
}

type Intent struct {
	IntentID      string            `json:"intentID,omitempty"`
	PodName       string            `json:"podName,omitempty"`
	PodID         string            `json:"podID,omitempty"`
	NodeID        string            `json:"nodeID,omitempty"`
	K8sNamespace  string            `json:"k8sNamespace,omitempty"`
	CommandRegex  string            `json:"commandRegex,omitempty"`
	Priority      int               `json:"priority,omitempty"`
	ExecutionTime int64             `json:"executionTime,omitempty"`
	PodLabels     map[string]string `json:"podLabels,omitempty"`
}

type SchedulingIntents struct {
	Priority      int             `json:"priority"`                // Priority value; higher value means higher priority
	ExecutionTime uint64          `json:"execution_time"`          // Time slice for this process in nanoseconds
	PID           int             `json:"pid,omitempty"`           // Process ID to apply this strategy to
	Selectors     []LabelSelector `json:"selectors,omitempty"`     // Label selectors to match pods
	CommandRegex  string          `json:"command_regex,omitempty"` // Regex to match process command
}

type LabelSelector struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}
