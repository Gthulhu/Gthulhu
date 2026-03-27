package domain

type RuntimeSchedulerConfig struct {
	ConfigVersion     string `json:"configVersion,omitempty"`
	Mode              string `json:"mode,omitempty"`
	SliceNsDefault    uint64 `json:"sliceNsDefault,omitempty"`
	SliceNsMin        uint64 `json:"sliceNsMin,omitempty"`
	KernelMode        bool   `json:"kernelMode,omitempty"`
	MaxTimeWatchdog   bool   `json:"maxTimeWatchdog,omitempty"`
	EarlyProcessing   bool   `json:"earlyProcessing,omitempty"`
	BuiltinIdle       bool   `json:"builtinIdle,omitempty"`
	SchedulerEnabled  bool   `json:"schedulerEnabled"`
	MonitoringEnabled bool   `json:"monitoringEnabled"`
}

type RuntimeConfigApplyOptions struct {
	NodeIDs []string               `json:"nodeIds,omitempty"`
	Config  RuntimeSchedulerConfig `json:"config"`
}

type RuntimeConfigApplyResult struct {
	NodeID        string `json:"nodeId"`
	Host          string `json:"host,omitempty"`
	Success       bool   `json:"success"`
	Error         string `json:"error,omitempty"`
	ConfigVersion string `json:"configVersion,omitempty"`
	AppliedAt     string `json:"appliedAt,omitempty"`
	RestartCount  int64  `json:"restartCount,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}
