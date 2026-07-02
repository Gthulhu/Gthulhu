package daemon

// runtimeConfigRequest is the daemon control API payload for runtime updates.
type runtimeConfigRequest struct {
	ConfigVersion     string `json:"configVersion,omitempty"`
	Mode              string `json:"mode,omitempty"`
	SchedulerName     string `json:"schedulerName,omitempty"`
	SliceNsDefault    uint64 `json:"sliceNsDefault,omitempty"`
	SliceNsMin        uint64 `json:"sliceNsMin,omitempty"`
	KernelMode        bool   `json:"kernelMode,omitempty"`
	MaxTimeWatchdog   bool   `json:"maxTimeWatchdog,omitempty"`
	EarlyProcessing   bool   `json:"earlyProcessing,omitempty"`
	BuiltinIdle       bool   `json:"builtinIdle,omitempty"`
	SchedulerEnabled  bool   `json:"schedulerEnabled"`
	MonitoringEnabled bool   `json:"monitoringEnabled"`
}

type runtimeConfigStatus struct {
	ConfigVersion     string `json:"configVersion,omitempty"`
	Applied           bool   `json:"applied"`
	AppliedAt         string `json:"appliedAt,omitempty"`
	RestartCount      int64  `json:"restartCount,omitempty"`
	LastError         string `json:"lastError,omitempty"`
	ConfigAvailable   bool   `json:"configAvailable"`
	Mode              string `json:"mode,omitempty"`
	SchedulerName     string `json:"schedulerName,omitempty"`
	SliceNsDefault    uint64 `json:"sliceNsDefault,omitempty"`
	SliceNsMin        uint64 `json:"sliceNsMin,omitempty"`
	KernelMode        *bool  `json:"kernelMode,omitempty"`
	MaxTimeWatchdog   *bool  `json:"maxTimeWatchdog,omitempty"`
	EarlyProcessing   *bool  `json:"earlyProcessing,omitempty"`
	BuiltinIdle       *bool  `json:"builtinIdle,omitempty"`
	SchedulerEnabled  *bool  `json:"schedulerEnabled,omitempty"`
	MonitoringEnabled *bool  `json:"monitoringEnabled,omitempty"`
}

type detailedStatus struct {
	ConfigVersion     string `json:"configVersion,omitempty"`
	Applied           bool   `json:"applied"`
	AppliedAt         string `json:"appliedAt,omitempty"`
	RestartCount      int64  `json:"restartCount"`
	LastError         string `json:"lastError,omitempty"`
	ConfigAvailable   bool   `json:"configAvailable"`
	Mode              string `json:"mode,omitempty"`
	SchedulerName     string `json:"schedulerName,omitempty"`
	SliceNsDefault    uint64 `json:"sliceNsDefault,omitempty"`
	SliceNsMin        uint64 `json:"sliceNsMin,omitempty"`
	KernelMode        *bool  `json:"kernelMode,omitempty"`
	MaxTimeWatchdog   *bool  `json:"maxTimeWatchdog,omitempty"`
	EarlyProcessing   *bool  `json:"earlyProcessing,omitempty"`
	BuiltinIdle       *bool  `json:"builtinIdle,omitempty"`
	SchedulerEnabled  *bool  `json:"schedulerEnabled,omitempty"`
	MonitoringEnabled *bool  `json:"monitoringEnabled,omitempty"`
}

type currentConfig struct {
	Mode              string
	SchedulerName     string
	SliceNsDefault    uint64
	SliceNsMin        uint64
	KernelMode        bool
	MaxTimeWatchdog   bool
	EarlyProcessing   bool
	BuiltinIdle       bool
	SchedulerEnabled  bool
	MonitoringEnabled bool
}

var allowedSCXSchedulers = map[string]struct{}{
	"scx_beerland":    {},
	"scx_bpfland":     {},
	"scx_cake":        {},
	"scx_chaos":       {},
	"scx_cosmos":      {},
	"scx_flash":       {},
	"scx_lavd":        {},
	"scx_layered":     {},
	"scx_mitosis":     {},
	"scx_p2dq":        {},
	"scx_pandemonium": {},
	"scx_rlfifo":      {},
	"scx_rustland":    {},
	"scx_rusty":       {},
	"scx_tickless":    {},
	"scx_timely":      {},
	"scx_wd40":        {},
}
