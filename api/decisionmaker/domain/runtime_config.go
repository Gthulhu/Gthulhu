package domain

import (
	"fmt"
	"strings"
)

const (
	SchedulerModeNone    = "none"
	SchedulerModeGthulhu = "gthulhu"
	SchedulerModeSimple  = "simple"
	SchedulerModeSCX     = "scx"
)

type RuntimeSchedulerConfig struct {
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

func (c *RuntimeSchedulerConfig) Normalize() {
	c.Mode = strings.TrimSpace(c.Mode)
	c.SchedulerName = strings.TrimSpace(c.SchedulerName)
	if c.Mode == "" {
		if c.SchedulerEnabled {
			c.Mode = SchedulerModeGthulhu
		} else {
			c.Mode = SchedulerModeNone
		}
	}
	switch c.Mode {
	case SchedulerModeNone:
		c.SchedulerEnabled = false
		c.SchedulerName = ""
	case SchedulerModeGthulhu, SchedulerModeSimple:
		c.SchedulerEnabled = true
		c.SchedulerName = ""
	case SchedulerModeSCX:
		c.SchedulerEnabled = true
	}
}

func (c RuntimeSchedulerConfig) Validate() error {
	switch c.Mode {
	case SchedulerModeNone, SchedulerModeGthulhu, SchedulerModeSimple:
		return nil
	case SchedulerModeSCX:
		if c.SchedulerName == "" {
			return fmt.Errorf("schedulerName is required when mode is scx")
		}
		if strings.Contains(c.SchedulerName, "/") || strings.Contains(c.SchedulerName, "\\") {
			return fmt.Errorf("schedulerName must be a binary name")
		}
		if !strings.HasPrefix(c.SchedulerName, "scx_") {
			return fmt.Errorf("schedulerName must be an scx scheduler binary")
		}
		return nil
	default:
		return fmt.Errorf("mode must be one of none, gthulhu, simple, scx")
	}
}

type RuntimeConfigStatus struct {
	ConfigVersion string                  `json:"configVersion,omitempty"`
	Applied       bool                    `json:"applied"`
	AppliedAt     string                  `json:"appliedAt,omitempty"`
	RestartCount  int64                   `json:"restartCount,omitempty"`
	LastError     string                  `json:"lastError,omitempty"`
	Config        *RuntimeSchedulerConfig `json:"config,omitempty"`
}
