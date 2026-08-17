package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

// TestLiveEngineCoreOnRealProc runs the actual #135 scanner and node-policy
// resolver against the real /proc on this machine. It is gated behind
// GTHULHU_LIVE=1 so it never runs in CI. Start a workload with named worker
// threads first, e.g. `uv run scratchpad/thread_demo_alive.py`.
func TestLiveEngineCoreOnRealProc(t *testing.T) {
	if os.Getenv("GTHULHU_LIVE") == "" {
		t.Skip("set GTHULHU_LIVE=1 and start an EngineCore workload to run the live /proc check")
	}
	ctx := context.Background()
	src := NewProcTaskSource("/proc")

	// 1) Real scanner over the real /proc: find the EngineCore worker threads.
	var engine []domain.TaskInfo
	for i := 0; i < 50; i++ {
		tasks, err := src.Snapshot(ctx)
		if err != nil {
			t.Fatalf("Snapshot(real /proc) failed: %v", err)
		}
		engine = engine[:0]
		for _, task := range tasks {
			if strings.HasPrefix(task.Comm, "EngineCore") {
				engine = append(engine, task)
			}
		}
		if len(engine) >= 2 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(engine) < 2 {
		t.Fatalf("no EngineCore worker threads found on real /proc (is the workload running?)")
	}

	sort.Slice(engine, func(i, j int) bool { return engine[i].TID < engine[j].TID })
	t.Log("real /proc scan found the worker threads the old top-level scan could not:")
	for _, task := range engine {
		leader := "?"
		if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", task.TGID)); err == nil {
			leader = strings.TrimSpace(string(b))
		}
		t.Logf("  TGID=%d leader_comm=%q -> TID=%d comm=%q  (non-leader thread: %v)",
			task.TGID, leader, task.TID, task.Comm, task.TID != task.TGID)
	}

	// 2) Real node-policy resolver (the exact #135 code path) against real /proc.
	svc := &Service{taskSource: src}
	if err := svc.ProcessNodePolicies(ctx, []*domain.NodePolicy{{
		PolicyID:      "live-enginecore",
		CommandRegex:  "^EngineCore(_DP[0-9]+)?$",
		Priority:      5,
		ExecutionTime: 20000000,
	}}); err != nil {
		t.Fatalf("ProcessNodePolicies: %v", err)
	}
	intents, err := svc.resolveNodeSchedulingIntents(ctx)
	if err != nil {
		t.Fatalf("resolveNodeSchedulingIntents: %v", err)
	}
	if len(intents) < 2 {
		t.Fatalf("resolver emitted %d intents; expected >=2 EngineCore worker TIDs", len(intents))
	}
	t.Logf("resolveNodeSchedulingIntents emitted %d intents keyed by the worker TID:", len(intents))
	for _, in := range intents {
		comm := "?"
		if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", in.PID)); err == nil {
			comm = strings.TrimSpace(string(b))
		}
		t.Logf("  intent PID(=TID)=%d comm=%q priority=%d execNs=%d", in.PID, comm, in.Priority, in.ExecutionTime)
	}
}
