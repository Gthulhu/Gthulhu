package daemon

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/Gthulhu/Gthulhu/internal/config"
	"github.com/Gthulhu/Gthulhu/internal/schedext"
)

func TestValidateScheduler(t *testing.T) {
	tests := []struct {
		name    string
		mode    string
		nameArg string
		wantErr bool
	}{
		{name: "default mode empty is allowed", mode: "", nameArg: "", wantErr: false},
		{name: "none mode", mode: "none", nameArg: "", wantErr: false},
		{name: "gthulhu mode", mode: "gthulhu", nameArg: "", wantErr: false},
		{name: "simple mode", mode: "simple", nameArg: "", wantErr: false},
		{name: "scx missing name", mode: "scx", nameArg: "", wantErr: true},
		{name: "scx invalid path", mode: "scx", nameArg: "../scx_bpfland", wantErr: true},
		{name: "scx disallowed name", mode: "scx", nameArg: "scx_unknown", wantErr: true},
		{name: "scx allowed name", mode: "scx", nameArg: "scx_bpfland", wantErr: false},
		{name: "unsupported mode", mode: "foo", nameArg: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScheduler(tt.mode, tt.nameArg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateScheduler(%q,%q) err=%v, wantErr=%v", tt.mode, tt.nameArg, err, tt.wantErr)
			}
		})
	}
}

func TestSchedulerCommandFromConfig_NoneModeMonitorEnabled(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	cfg := config.DefaultConfig()
	cfg.Scheduler.Mode = "none"
	cfg.Monitor.Enabled = true
	if err := writeConfigFile(cfgPath, cfg); err != nil {
		t.Fatalf("writeConfigFile failed: %v", err)
	}

	bin, args, enabled, err := schedulerCommandFromConfig(cfgPath, "/tmp/gthulhu")
	if err != nil {
		t.Fatalf("schedulerCommandFromConfig error: %v", err)
	}
	if !enabled {
		t.Fatalf("enabled=false, want true")
	}
	if bin != "/tmp/gthulhu" {
		t.Fatalf("bin=%q, want /tmp/gthulhu", bin)
	}
	if len(args) != 3 || args[0] != modeScheduler || args[1] != "-config" || args[2] != cfgPath {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestSchedulerCommandFromConfig_SchedExtUnsupportedFallsBackToMonitorOnly(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	cfg := config.DefaultConfig()
	cfg.Scheduler.Mode = "gthulhu"
	cfg.Monitor.Enabled = true
	if err := writeConfigFile(cfgPath, cfg); err != nil {
		t.Fatalf("writeConfigFile failed: %v", err)
	}

	orig := schedExtSupportChecker
	t.Cleanup(func() { schedExtSupportChecker = orig })
	schedExtSupportChecker = func() error { return schedext.ErrUnsupported }

	bin, args, enabled, err := schedulerCommandFromConfig(cfgPath, "/tmp/gthulhu")
	if err != nil {
		t.Fatalf("schedulerCommandFromConfig error: %v", err)
	}
	if !enabled {
		t.Fatalf("enabled=false, want true")
	}
	if bin != "/tmp/gthulhu" {
		t.Fatalf("bin=%q, want /tmp/gthulhu", bin)
	}
	if len(args) != 3 || args[0] != modeScheduler || args[1] != "-config" || args[2] != cfgPath {
		t.Fatalf("unexpected args: %v", args)
	}
}

func TestSchedulerCommandFromConfig_SchedExtUnsupportedWithoutMonitorReturnsError(t *testing.T) {
	tmp := t.TempDir()
	cfgPath := filepath.Join(tmp, "cfg.yaml")
	cfg := config.DefaultConfig()
	cfg.Scheduler.Mode = "gthulhu"
	cfg.Monitor.Enabled = false
	if err := writeConfigFile(cfgPath, cfg); err != nil {
		t.Fatalf("writeConfigFile failed: %v", err)
	}

	orig := schedExtSupportChecker
	t.Cleanup(func() { schedExtSupportChecker = orig })
	schedExtSupportChecker = func() error { return schedext.ErrUnsupported }

	_, _, _, err := schedulerCommandFromConfig(cfgPath, "/tmp/gthulhu")
	if !errors.Is(err, schedext.ErrUnsupported) {
		t.Fatalf("error=%v, want ErrUnsupported", err)
	}
}
