package policy

import (
	"errors"
	"testing"

	"github.com/Gthulhu/Gthulhu/internal/config"
)

func TestShouldRunMonitorOnly(t *testing.T) {
	errSchedExt := errors.New("sched_ext unsupported")

	tests := []struct {
		name            string
		cfg             *config.Config
		schedExtErr     error
		wantMonitorOnly bool
		wantErr         bool
	}{
		{
			name:            "scheduler disabled always monitor-only",
			cfg:             schedulerAndMonitor(false, true),
			schedExtErr:     nil,
			wantMonitorOnly: true,
			wantErr:         false,
		},
		{
			name:            "scheduler enabled and sched_ext ok",
			cfg:             schedulerAndMonitor(true, true),
			schedExtErr:     nil,
			wantMonitorOnly: false,
			wantErr:         false,
		},
		{
			name:            "scheduler enabled sched_ext fails monitor enabled fallback",
			cfg:             schedulerAndMonitor(true, true),
			schedExtErr:     errSchedExt,
			wantMonitorOnly: true,
			wantErr:         false,
		},
		{
			name:            "scheduler enabled sched_ext fails monitor disabled returns error",
			cfg:             schedulerAndMonitor(true, false),
			schedExtErr:     errSchedExt,
			wantMonitorOnly: false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotMonitorOnly, err := ShouldRunMonitorOnly(tt.cfg, tt.schedExtErr)
			if gotMonitorOnly != tt.wantMonitorOnly {
				t.Fatalf("monitorOnly=%v, want %v", gotMonitorOnly, tt.wantMonitorOnly)
			}
			if (err != nil) != tt.wantErr {
				t.Fatalf("err=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func schedulerAndMonitor(schedulerEnabled, monitorEnabled bool) *config.Config {
	cfg := config.DefaultConfig()
	if schedulerEnabled {
		cfg.Scheduler.Mode = "gthulhu"
	} else {
		cfg.Scheduler.Mode = "none"
	}
	cfg.Monitor.Enabled = monitorEnabled
	return cfg
}
