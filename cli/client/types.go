// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package client

// TokenRequest represents the request body for JWT token generation.
type TokenRequest struct {
	PublicKey string `json:"public_key"`
}

// TokenData holds the token string and its expiration timestamp.
type TokenData struct {
	Token     string `json:"token,omitempty"`
	ExpiredAt int64  `json:"expired_at,omitempty"`
}

// TokenResponse is the API response for token requests.
type TokenResponse struct {
	Success   bool      `json:"success"`
	Data      TokenData `json:"data"`
	Timestamp string    `json:"timestamp"`
}

// LabelSelector represents a Kubernetes label selector.
type LabelSelector struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// ScheduleStrategy represents a scheduling strategy in the response.
type ScheduleStrategy struct {
	ID                string          `json:"id"`
	Priority          int             `json:"priority"`
	ExecutionTime     int             `json:"executionTime"`
	CommandRegex      string          `json:"commandRegex"`
	K8sNamespace      []string        `json:"k8sNamespace"`
	LabelSelectors    []LabelSelector `json:"labelSelectors"`
	StrategyNamespace string          `json:"strategyNamespace"`
}

// ListSchedulerStrategiesData holds the list of strategies.
type ListSchedulerStrategiesData struct {
	Strategies []ScheduleStrategy `json:"strategies"`
}

// ListSchedulerStrategiesResponse is the API response for listing strategies.
type ListSchedulerStrategiesResponse struct {
	Success   bool                        `json:"success"`
	Data      ListSchedulerStrategiesData `json:"data"`
	Timestamp string                      `json:"timestamp"`
}

// CreateScheduleStrategyRequest is the request body for creating a strategy.
type CreateScheduleStrategyRequest struct {
	Priority          int             `json:"priority"`
	ExecutionTime     int             `json:"executionTime"`
	CommandRegex      string          `json:"commandRegex"`
	K8sNamespace      []string        `json:"k8sNamespace"`
	LabelSelectors    []LabelSelector `json:"labelSelectors"`
	StrategyNamespace string          `json:"strategyNamespace"`
}

// DeleteScheduleStrategyRequest is the request body for deleting a strategy.
type DeleteScheduleStrategyRequest struct {
	StrategyID string `json:"strategyId"`
}

// EmptyDataResponse represents a successful response with empty data.
type EmptyDataResponse struct {
	Success   bool     `json:"success"`
	Data      struct{} `json:"data"`
	Timestamp string   `json:"timestamp"`
}

// BssData mirrors the scheduler BSS metrics structure.
type BssData struct {
	UserschedLastRunAt uint64 `json:"usersched_last_run_at"`
	NrQueued           uint64 `json:"nr_queued"`
	NrScheduled        uint64 `json:"nr_scheduled"`
	NrRunning          uint64 `json:"nr_running"`
	NrOnlineCpus       uint64 `json:"nr_online_cpus"`
	NrUserDispatches   uint64 `json:"nr_user_dispatches"`
	NrKernelDispatches uint64 `json:"nr_kernel_dispatches"`
	NrCancelDispatches uint64 `json:"nr_cancel_dispatches"`
	NrBounceDispatches uint64 `json:"nr_bounce_dispatches"`
	NrFailedDispatches uint64 `json:"nr_failed_dispatches"`
	NrSchedCongested   uint64 `json:"nr_sched_congested"`
}

// MetricsResponse is the API response for metrics endpoints.
type MetricsResponse struct {
	Success   bool     `json:"success"`
	Message   string   `json:"message,omitempty"`
	Timestamp string   `json:"timestamp,omitempty"`
	Data      *BssData `json:"data,omitempty"`
}

// ---------------------------------------------------------------------------
// Nodes
// ---------------------------------------------------------------------------

// NodeInfo represents a Kubernetes node.
type NodeInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ListNodesData holds the list of nodes.
type ListNodesData struct {
	Nodes []NodeInfo `json:"nodes"`
}

// ListNodesResponse is the API response for listing nodes.
type ListNodesResponse struct {
	Success   bool          `json:"success"`
	Data      ListNodesData `json:"data"`
	Timestamp string        `json:"timestamp"`
}

// PodPIDProcess represents a process in a pod.
type PodPIDProcess struct {
	PID         int    `json:"pid"`
	PPID        int    `json:"ppid"`
	Command     string `json:"command"`
	ContainerID string `json:"container_id"`
}

// PodPIDInfo holds pod and process information.
type PodPIDInfo struct {
	PodID     string          `json:"pod_id"`
	PodUID    string          `json:"pod_uid"`
	Processes []PodPIDProcess `json:"processes"`
}

// GetNodePodPIDMappingData holds the node's pod-PID mapping data.
type GetNodePodPIDMappingData struct {
	NodeID    string       `json:"node_id"`
	NodeName  string       `json:"node_name"`
	Pods      []PodPIDInfo `json:"pods"`
	Timestamp string       `json:"timestamp"`
}

// GetNodePodPIDMappingResponse is the API response for node pod-PID mappings.
type GetNodePodPIDMappingResponse struct {
	Success   bool                     `json:"success"`
	Data      GetNodePodPIDMappingData `json:"data"`
	Timestamp string                   `json:"timestamp"`
}

// ---------------------------------------------------------------------------
// Pods (/api/v1/pods/pids - decisionmaker endpoint)
// ---------------------------------------------------------------------------

// PodProcess represents a process in a pod (decisionmaker format).
type PodProcess struct {
	PID         int    `json:"pid"`
	PPID        int    `json:"ppid"`
	Command     string `json:"command"`
	ContainerID string `json:"container_id"`
}

// PodInfo holds pod and process information (decisionmaker format).
type PodInfo struct {
	PodID     string       `json:"pod_id"`
	PodUID    string       `json:"pod_uid"`
	Processes []PodProcess `json:"processes"`
}

// GetPodsPIDsData holds the pod-PID mapping data.
type GetPodsPIDsData struct {
	NodeID    string    `json:"node_id"`
	NodeName  string    `json:"node_name"`
	Pods      []PodInfo `json:"pods"`
	Timestamp string    `json:"timestamp"`
}

// GetPodsPIDsResponse is the API response for /api/v1/pods/pids (decisionmaker endpoint).
type GetPodsPIDsResponse struct {
	Success   bool            `json:"success"`
	Data      GetPodsPIDsData `json:"data"`
	Timestamp string          `json:"timestamp"`
}

// LoginRequest represents the request body for user login.
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// LoginData holds the JWT token returned from login.
type LoginData struct {
	Token string `json:"token"`
}

// LoginResponse is the API response for login requests.
type LoginResponse struct {
	Success   bool      `json:"success"`
	Data      LoginData `json:"data"`
	Timestamp string    `json:"timestamp"`
}

// ErrorResponse represents a generic API error response.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}
