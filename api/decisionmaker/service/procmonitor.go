package service

import (
	"context"
	"os"
	"strconv"
	"strings"
)

// ProcessSource enumerates currently running processes on the node, keyed by
// PID with their executable/comm name (as read from /proc/<pid>/comm).
//
// The default implementation (procScanSource) polls /proc directly, which
// works everywhere without special privileges beyond what the Decision Maker
// already requires. On kernels where the sched_ext scheduler eBPF stack is
// available, the same information can instead be produced by
// bpf/proc_monitor.bpf.c, which hooks tp_btf/sched_process_exec and
// tp_btf/sched_process_exit to notify user-space of process start/exit in
// real time (see docs/node-scheduling-policy.md). Since regex matching
// cannot run inside the BPF program, both implementations feed the same
// PID -> comm snapshot into resolveNodeSchedulingIntents, which performs the
// CommandRegex matching in user-space.
type ProcessSource interface {
	// Snapshot returns the set of processes currently running on the node.
	Snapshot(ctx context.Context) (map[int]string, error)
}

// procScanSource implements ProcessSource by scanning /proc.
type procScanSource struct {
	rootDir string
}

// NewProcScanSource creates a ProcessSource backed by /proc scanning. Passing
// an empty rootDir defaults to "/proc".
func NewProcScanSource(rootDir string) ProcessSource {
	if rootDir == "" {
		rootDir = "/proc"
	}
	return &procScanSource{rootDir: rootDir}
}

func (p *procScanSource) Snapshot(_ context.Context) (map[int]string, error) {
	entries, err := os.ReadDir(p.rootDir)
	if err != nil {
		return nil, err
	}

	procs := make(map[int]string)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		commPath := p.rootDir + "/" + entry.Name() + "/comm"
		data, err := os.ReadFile(commPath)
		if err != nil {
			// Process may have exited between ReadDir and ReadFile.
			continue
		}
		procs[pid] = strings.TrimSpace(string(data))
	}
	return procs, nil
}
