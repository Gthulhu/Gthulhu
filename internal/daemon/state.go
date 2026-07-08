package daemon

import (
	"os"
	"sync"
	"time"

	"github.com/Gthulhu/Gthulhu/internal/config"
)

type controlState struct {
	mu                sync.RWMutex
	configVersion     string
	applied           bool
	appliedAt         time.Time
	restartCount      int64
	lastError         string
	runtimeConfigPath string
	cachedConfig      *currentConfig
	cachedConfigMTime time.Time
}

func boolPtr(v bool) *bool { return &v }

func (s *controlState) readCurrentConfig() (*currentConfig, bool) {
	s.mu.RLock()
	runtimeConfigPath := s.runtimeConfigPath
	cachedConfig := s.cachedConfig
	cachedConfigMTime := s.cachedConfigMTime
	s.mu.RUnlock()

	if runtimeConfigPath == "" {
		return nil, false
	}

	fi, err := os.Stat(runtimeConfigPath)
	if err != nil {
		return nil, false
	}

	if cachedConfig != nil && fi.ModTime().Equal(cachedConfigMTime) {
		return cachedConfig, true
	}

	cfg, err := config.LoadConfig(runtimeConfigPath)
	if err != nil {
		return nil, false
	}

	loadedConfig := &currentConfig{
		Mode:              cfg.Scheduler.Mode,
		SchedulerName:     cfg.Scheduler.SchedulerName,
		SliceNsDefault:    cfg.Scheduler.SliceNsDefault,
		SliceNsMin:        cfg.Scheduler.SliceNsMin,
		KernelMode:        cfg.Scheduler.KernelMode,
		MaxTimeWatchdog:   cfg.Scheduler.MaxTimeWatchdog,
		EarlyProcessing:   cfg.EarlyProcessing,
		BuiltinIdle:       cfg.BuiltinIdle,
		SchedulerEnabled:  cfg.IsSchedulerEnabled(),
		MonitoringEnabled: cfg.Monitor.Enabled,
	}

	s.mu.Lock()
	s.cachedConfig = loadedConfig
	s.cachedConfigMTime = fi.ModTime()
	s.mu.Unlock()

	return loadedConfig, true
}

func (s *controlState) set(version string, applied bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.configVersion = version
	s.applied = applied
	if applied {
		s.appliedAt = time.Now()
		s.lastError = ""
	}
}

func (s *controlState) recordError(errMsg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lastError = errMsg
}

func (s *controlState) recordRestart() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.restartCount++
}

func (s *controlState) snapshot() runtimeConfigStatus {
	s.mu.RLock()
	configVersion := s.configVersion
	applied := s.applied
	restartCount := s.restartCount
	lastError := s.lastError
	appliedAt := s.appliedAt
	s.mu.RUnlock()

	var appliedAtStr string
	if !appliedAt.IsZero() {
		appliedAtStr = appliedAt.UTC().Format(time.RFC3339)
	}

	resp := runtimeConfigStatus{
		ConfigVersion: configVersion,
		Applied:       applied,
		AppliedAt:     appliedAtStr,
		RestartCount:  restartCount,
		LastError:     lastError,
	}
	if cfg, ok := s.readCurrentConfig(); ok {
		resp.ConfigAvailable = true
		resp.Mode = cfg.Mode
		resp.SchedulerName = cfg.SchedulerName
		resp.SliceNsDefault = cfg.SliceNsDefault
		resp.SliceNsMin = cfg.SliceNsMin
		resp.KernelMode = boolPtr(cfg.KernelMode)
		resp.MaxTimeWatchdog = boolPtr(cfg.MaxTimeWatchdog)
		resp.EarlyProcessing = boolPtr(cfg.EarlyProcessing)
		resp.BuiltinIdle = boolPtr(cfg.BuiltinIdle)
		resp.SchedulerEnabled = boolPtr(cfg.SchedulerEnabled)
		resp.MonitoringEnabled = boolPtr(cfg.MonitoringEnabled)
	}

	return resp
}

func (s *controlState) detailedSnapshot() detailedStatus {
	s.mu.RLock()
	configVersion := s.configVersion
	applied := s.applied
	restartCount := s.restartCount
	lastError := s.lastError
	appliedAt := s.appliedAt
	s.mu.RUnlock()

	var appliedAtStr string
	if !appliedAt.IsZero() {
		appliedAtStr = appliedAt.UTC().Format(time.RFC3339)
	}

	resp := detailedStatus{
		ConfigVersion: configVersion,
		Applied:       applied,
		AppliedAt:     appliedAtStr,
		RestartCount:  restartCount,
		LastError:     lastError,
	}
	if cfg, ok := s.readCurrentConfig(); ok {
		resp.ConfigAvailable = true
		resp.Mode = cfg.Mode
		resp.SchedulerName = cfg.SchedulerName
		resp.SliceNsDefault = cfg.SliceNsDefault
		resp.SliceNsMin = cfg.SliceNsMin
		resp.KernelMode = boolPtr(cfg.KernelMode)
		resp.MaxTimeWatchdog = boolPtr(cfg.MaxTimeWatchdog)
		resp.EarlyProcessing = boolPtr(cfg.EarlyProcessing)
		resp.BuiltinIdle = boolPtr(cfg.BuiltinIdle)
		resp.SchedulerEnabled = boolPtr(cfg.SchedulerEnabled)
		resp.MonitoringEnabled = boolPtr(cfg.MonitoringEnabled)
	}

	return resp
}
