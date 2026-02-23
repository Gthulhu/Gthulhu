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

// SchedulingStrategy represents a scheduling strategy entry.
type SchedulingStrategy struct {
	Priority      int    `json:"priority"`
	ExecutionTime uint64 `json:"execution_time"`
	PID           int    `json:"pid"`
}

// SchedulingStrategiesResponse is the API response for strategy queries.
type SchedulingStrategiesResponse struct {
	Success    bool                 `json:"success"`
	Message    string               `json:"message,omitempty"`
	Timestamp  string               `json:"timestamp,omitempty"`
	Scheduling []SchedulingStrategy `json:"scheduling"`
}

// SchedulingStrategiesRequest is the request body for setting strategies.
type SchedulingStrategiesRequest struct {
	Strategies []StrategyInput `json:"strategies"`
}

// StrategyInput represents a single strategy in a set-strategies request.
type StrategyInput struct {
	Priority      bool              `json:"priority"`
	ExecutionTime uint64            `json:"execution_time"`
	Selectors     []SelectorEntry   `json:"selectors,omitempty"`
	CommandRegex  string            `json:"command_regex,omitempty"`
}

// SelectorEntry is a key-value label selector used in strategy requests.
type SelectorEntry struct {
	Key   string `json:"key"`
	Value string `json:"value"`
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

// PodPIDEntry represents a single pod-to-PID mapping.
type PodPIDEntry struct {
	PodName   string `json:"pod_name"`
	Namespace string `json:"namespace"`
	PID       int    `json:"pid"`
}

// PodPIDsResponse is the API response for pod PID queries.
type PodPIDsResponse struct {
	Success   bool          `json:"success"`
	Message   string        `json:"message,omitempty"`
	Timestamp string        `json:"timestamp,omitempty"`
	Data      []PodPIDEntry `json:"data,omitempty"`
}

// ErrorResponse represents a generic API error response.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}
