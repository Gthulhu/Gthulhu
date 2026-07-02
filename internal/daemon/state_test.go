package daemon

import (
	"path/filepath"
	"testing"

	"github.com/Gthulhu/Gthulhu/internal/config"
)

func TestControlStateSnapshotWithConfig(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "runtime.yaml")

	cfg := config.DefaultConfig()
	cfg.Scheduler.Mode = "gthulhu"
	cfg.Scheduler.SliceNsDefault = 10
	cfg.Scheduler.SliceNsMin = 5
	cfg.Monitor.Enabled = true
	if err := writeConfigFile(cfgPath, cfg); err != nil {
		t.Fatalf("writeConfigFile failed: %v", err)
	}

	state := &controlState{runtimeConfigPath: cfgPath}
	state.set("v1", true)
	state.recordRestart()

	snapshot := state.snapshot()
	if !snapshot.Applied {
		t.Fatalf("snapshot.Applied=false, want true")
	}
	if snapshot.ConfigVersion != "v1" {
		t.Fatalf("snapshot.ConfigVersion=%q, want v1", snapshot.ConfigVersion)
	}
	if snapshot.RestartCount != 1 {
		t.Fatalf("snapshot.RestartCount=%d, want 1", snapshot.RestartCount)
	}
	if !snapshot.ConfigAvailable {
		t.Fatalf("snapshot.ConfigAvailable=false, want true")
	}
	if snapshot.Mode != "gthulhu" {
		t.Fatalf("snapshot.Mode=%q, want gthulhu", snapshot.Mode)
	}
	if snapshot.SliceNsDefault != 10 || snapshot.SliceNsMin != 5 {
		t.Fatalf("unexpected slice values default=%d min=%d", snapshot.SliceNsDefault, snapshot.SliceNsMin)
	}
	if snapshot.SchedulerEnabled == nil || !*snapshot.SchedulerEnabled {
		t.Fatalf("snapshot.SchedulerEnabled=%v, want true", snapshot.SchedulerEnabled)
	}
	if snapshot.MonitoringEnabled == nil || !*snapshot.MonitoringEnabled {
		t.Fatalf("snapshot.MonitoringEnabled=%v, want true", snapshot.MonitoringEnabled)
	}
}

func TestControlStateRecordError(t *testing.T) {
	state := &controlState{}
	state.recordError("boom")

	detail := state.detailedSnapshot()
	if detail.LastError != "boom" {
		t.Fatalf("detail.LastError=%q, want boom", detail.LastError)
	}
}
