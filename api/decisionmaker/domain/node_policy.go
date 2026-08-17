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

// TaskInfo identifies one kernel scheduling entity - a thread, not a process.
// The Linux scheduler acts on threads (TID); TGID ties a thread back to its
// group, and Comm is the thread's own /proc/<tgid>/task/<tid>/comm.
type TaskInfo struct {
	TGID int
	TID  int
	Comm string
}
