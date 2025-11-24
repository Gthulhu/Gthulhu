package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Gthulhu/Gthulhu/gtp5g_operator/pkg/auth"
)

// SchedulingStrategy represents a scheduling strategy for Gthulhu
type SchedulingStrategy struct {
	PID           int    `json:"pid,omitempty"`
	Priority      bool   `json:"priority"`
	ExecutionTime uint64 `json:"execution_time"`
}

// SchedulingStrategies represents the request body for strategy updates
type SchedulingStrategies struct {
	Strategies []SchedulingStrategy `json:"strategies"`
}

// Client handles communication with Gthulhu API
type Client struct {
	apiEndpoint string
	authClient  *auth.Client
	httpClient  *http.Client
	mu          sync.Mutex
}

// NewClient creates a new API client
func NewClient(apiEndpoint, publicKeyPath string) *Client {
	return &Client{
		apiEndpoint: apiEndpoint,
		authClient:  auth.NewClient(apiEndpoint, publicKeyPath),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendStrategies sends scheduling strategies to Gthulhu API (overwrite mode)
func (c *Client) SendStrategies(ctx context.Context, pids map[int]bool, enablePriority bool, executionTimeNs uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(pids) == 0 {
		log.Println("No nr-gnb PIDs to send")
		return nil
	}

	// Build strategies list
	strategies := SchedulingStrategies{
		Strategies: make([]SchedulingStrategy, 0, len(pids)),
	}

	for pid := range pids {
		strategies.Strategies = append(strategies.Strategies, SchedulingStrategy{
			PID:           pid,
			Priority:      enablePriority,
			ExecutionTime: executionTimeNs,
		})
	}

	// Marshal request
	jsonData, err := json.Marshal(strategies)
	if err != nil {
		return fmt.Errorf("failed to marshal strategies: %w", err)
	}

	// Get JWT token
	token, err := c.authClient.GetToken()
	if err != nil {
		return fmt.Errorf("failed to get JWT token: %w", err)
	}

	// Send POST request
	strategiesURL := c.apiEndpoint + "/api/v1/scheduling/strategies"
	req, err := http.NewRequestWithContext(ctx, "POST", strategiesURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Printf("Successfully sent %d strategies to Gthulhu API", len(strategies.Strategies))
	return nil
}
