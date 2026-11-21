package scheduler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"sync/atomic"
	"time"
)

type strategyHTTPClient interface {
	MakeAuthenticatedRequest(method, url string, body io.Reader) (*http.Response, error)
}

func fetchKernelStrategies(client strategyHTTPClient, baseURL string) ([]kernelStrategy, error) {
	resp, err := client.MakeAuthenticatedRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v1/scheduling/strategies", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("decision maker strategies: HTTP %d", resp.StatusCode)
	}
	var result struct {
		Success    bool             `json:"success"`
		Scheduling []kernelStrategy `json:"scheduling"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode decision maker strategies: %w", err)
	}
	if !result.Success || result.Scheduling == nil {
		return nil, fmt.Errorf("decision maker did not return a successful strategies snapshot")
	}
	return result.Scheduling, nil
}

// The scheduler consumes only the latest successful complete snapshot. A
// network failure is never treated as an authoritative empty snapshot.
func pollKernelStrategies(ctx context.Context, client strategyHTTPClient, baseURL string, interval time.Duration, latest *atomic.Pointer[[]kernelStrategy]) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for ctx.Err() == nil {
		if strategies, err := fetchKernelStrategies(client, baseURL); err == nil {
			latest.Store(&strategies)
		} else {
			slog.Warn("Failed to fetch kernel scheduling strategies", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
