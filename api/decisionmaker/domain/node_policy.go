package domain

// NodePolicy represents a node-level scheduling policy pushed from the
// Manager. Unlike Intent (which targets a specific Pod/container), a
// NodePolicy targets arbitrary processes running on the node - including
// kthreads, host daemons, and any other non-Pod process - identified purely
// by their executable/comm name via CommandRegex.
type NodePolicy struct {
	PolicyID      string `json:"policyID,omitempty"`
	NodeID        string `json:"nodeID,omitempty"`
	Node          string `json:"node,omitempty"`
	Generation    int64  `json:"generation,omitempty"`
	CommandRegex  string `json:"commandRegex,omitempty"`
	Priority      int    `json:"priority,omitempty"`
	ExecutionTime int64  `json:"executionTime,omitempty"`
}

// TaskInfo identifies one kernel scheduling entity - a thread, not a process.
// The Linux scheduler acts on threads (TID); TGID ties a thread back to its
// group, and Comm is the thread's own /proc/<tgid>/task/<tid>/comm.
type TaskInfo struct {
	Node         string `json:"node"`
	TGID         int    `json:"tgid"`
	TID          int    `json:"tid"`
	Comm         string `json:"comm"`
	StartTime    uint64 `json:"startTime"`
	CgroupPath   string `json:"cgroupPath"`
	PodUID       string `json:"podUID,omitempty"`
	KernelThread bool   `json:"kernelThread,omitempty"`
}

type NodePolicyMatch struct {
	Policy      NodePolicy `json:"policy"`
	Task        TaskInfo   `json:"task"`
	IntendedKey int        `json:"intendedKey"`
}

type NodePolicyPreviewRequest struct {
	Policy   *NodePolicy `json:"policy,omitempty"`
	PolicyID string      `json:"policyID,omitempty"`
}

type NodePolicyPreview struct {
	Policy       NodePolicy           `json:"policy"`
	MatchedTasks []TaskInfo           `json:"matchedTasks"`
	Conflicts    []NodePolicyConflict `json:"conflicts"`
	Warnings     []NodePolicyWarning  `json:"warnings"`
}

type NodePolicyConflict struct {
	Task     TaskInfo     `json:"task"`
	Policies []NodePolicy `json:"policies"`
	Winner   NodePolicy   `json:"winner"`
}

type NodePolicyWarning struct {
	Node    string `json:"node,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}
