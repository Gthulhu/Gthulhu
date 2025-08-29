package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/Gthulhu/Gthulhu/internal/auth"
)

// BssData represents the metrics data structure that matches the API server
type BssData struct {
	UserschedLastRunAt uint64 `json:"usersched_last_run_at"` // The PID of the userspace scheduler
	NrQueued           uint64 `json:"nr_queued"`             // Number of tasks queued in the userspace scheduler
	NrScheduled        uint64 `json:"nr_scheduled"`          // Number of tasks scheduled by the userspace scheduler
	NrRunning          uint64 `json:"nr_running"`            // Number of tasks currently running in the userspace scheduler
	NrOnlineCpus       uint64 `json:"nr_online_cpus"`        // Number of online CPUs in the system
	NrUserDispatches   uint64 `json:"nr_user_dispatches"`    // Number of user-space dispatches
	NrKernelDispatches uint64 `json:"nr_kernel_dispatches"`  // Number of kernel-space dispatches
	NrCancelDispatches uint64 `json:"nr_cancel_dispatches"`  // Number of cancelled dispatches
	NrBounceDispatches uint64 `json:"nr_bounce_dispatches"`  // Number of bounce dispatches
	NrFailedDispatches uint64 `json:"nr_failed_dispatches"`  // Number of failed dispatches
	NrSchedCongested   uint64 `json:"nr_sched_congested"`    // Number of times the scheduler was congested
}

// MetricsResponse represents the response structure from the API server
type MetricsResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

// ErrorResponse represents error response structure
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// MetricsClient handles sending metrics to the API server
type MetricsClient struct {
	jwtClient    *auth.JWTClient
	metricsURL   string
	lastSentTime time.Time
	minInterval  time.Duration
}

// NewMetricsClient creates a new metrics client
func NewMetricsClient(jwtClient *auth.JWTClient, apiBaseURL string) *MetricsClient {
	return &MetricsClient{
		jwtClient:   jwtClient,
		metricsURL:  apiBaseURL + "/api/v1/metrics",
		minInterval: 5 * time.Second, // Minimum interval between sends to avoid spam
	}
}

// SendMetrics sends BSS metrics data to the API server
func (c *MetricsClient) SendMetrics(data BssData) error {
	// Rate limiting: don't send too frequently
	if time.Since(c.lastSentTime) < c.minInterval {
		return nil // Skip sending
	}

	// Convert to JSON
	jsonData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal metrics data: %v", err)
	}

	// Send authenticated request
	resp, err := c.jwtClient.MakeAuthenticatedRequest("POST", c.metricsURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send metrics request: %v", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != 200 {
		return fmt.Errorf("metrics request failed with status code: %d", resp.StatusCode)
	}

	c.lastSentTime = time.Now()
	log.Printf("Successfully sent metrics to API server")
	return nil
}

// SendMetricsAsync sends metrics asynchronously (non-blocking)
func (c *MetricsClient) SendMetricsAsync(data BssData) {
	if err := c.SendMetrics(data); err != nil {
		log.Printf("Failed to send metrics: %v", err)
	}
}
