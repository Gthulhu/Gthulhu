package domain

// NodePolicy represents a node-level scheduling policy pushed from the
// Manager. Unlike Intent (which targets a specific Pod/container), a
// NodePolicy targets arbitrary processes running on the node - including
// kthreads, host daemons, and any other non-Pod process - identified purely
// by their executable/comm name via CommandRegex.
type NodePolicy struct {
	PolicyID      string `json:"policyID,omitempty"`
	NodeID        string `json:"nodeID,omitempty"`
	CommandRegex  string `json:"commandRegex,omitempty"`
	Priority      int    `json:"priority,omitempty"`
	ExecutionTime int64  `json:"executionTime,omitempty"`
}

// ProcessInfo describes a running process discovered on the node, keyed by
// PID with its executable/comm name. It is populated either by polling
// /proc (default, portable implementation) or by the eBPF process monitor
// (bpf/proc_monitor.bpf.c) which observes sched_process_exec/exit
// tracepoints in real time.
type ProcessInfo struct {
	PID  int
	Comm string
}
