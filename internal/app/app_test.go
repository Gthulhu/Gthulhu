package app

import (
	"errors"
	"testing"

	"github.com/Gthulhu/Gthulhu/internal/schedext"
)

func TestResolveModeAndArgs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantMode string
		wantArgs []string
	}{
		{name: "default scheduler when no args", args: []string{}, wantMode: modeScheduler, wantArgs: []string{}},
		{name: "explicit scheduler mode", args: []string{modeScheduler, "-config", "a.yaml"}, wantMode: modeScheduler, wantArgs: []string{"-config", "a.yaml"}},
		{name: "explicit daemon mode", args: []string{modeDaemon, "-config", "a.yaml"}, wantMode: modeDaemon, wantArgs: []string{"-config", "a.yaml"}},
		{name: "legacy scheduler flags", args: []string{"-config", "a.yaml"}, wantMode: modeScheduler, wantArgs: []string{"-config", "a.yaml"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, modeArgs := resolveModeAndArgs(tt.args)
			if mode != tt.wantMode {
				t.Fatalf("mode=%q, want %q", mode, tt.wantMode)
			}
			if len(modeArgs) != len(tt.wantArgs) {
				t.Fatalf("args len=%d, want %d", len(modeArgs), len(tt.wantArgs))
			}
			for i := range modeArgs {
				if modeArgs[i] != tt.wantArgs[i] {
					t.Fatalf("args[%d]=%q, want %q", i, modeArgs[i], tt.wantArgs[i])
				}
			}
		})
	}
}

func TestIsGlobalHelpRequest(t *testing.T) {
	tests := []struct {
		args []string
		want bool
	}{
		{args: []string{}, want: false},
		{args: []string{"help"}, want: true},
		{args: []string{"-h"}, want: true},
		{args: []string{"--help"}, want: true},
		{args: []string{"scheduler", "--help"}, want: false},
	}

	for _, tt := range tests {
		if got := isGlobalHelpRequest(tt.args); got != tt.want {
			t.Fatalf("isGlobalHelpRequest(%v)=%v, want %v", tt.args, got, tt.want)
		}
	}
}

func TestExitCode(t *testing.T) {
	if code := ExitCode(nil); code != 0 {
		t.Fatalf("ExitCode(nil)=%d, want 0", code)
	}
	if code := ExitCode(errors.New("x")); code != 1 {
		t.Fatalf("ExitCode(generic)=%d, want 1", code)
	}
	if code := ExitCode(schedext.ErrUnsupported); code != schedext.UnsupportedExitCode {
		t.Fatalf("ExitCode(ErrUnsupported)=%d, want %d", code, schedext.UnsupportedExitCode)
	}
}
