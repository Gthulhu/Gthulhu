package sched

import (
	"context"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"time"

	core "github.com/Gthulhu/scx_goland_core/goland_core"
)

// SchedulingStrategy represents a strategy for process scheduling
type SchedulingStrategy struct {
	Priority      bool   `json:"priority"`       // If true, set vtime to minimum vtime
	ExecutionTime uint64 `json:"execution_time"` // Time slice for this process in nanoseconds
	PID           int    `json:"pid"`            // Process ID to apply this strategy to
}

// SchedulingStrategiesResponse represents the response structure from the API
type SchedulingStrategiesResponse struct {
	Success    bool                 `json:"success"`
	Message    string               `json:"message"`
	Timestamp  string               `json:"timestamp"`
	Scheduling []SchedulingStrategy `json:"scheduling"`
}

// strategyMap maps PIDs to their scheduling strategies
var strategyMap = make(map[int32]SchedulingStrategy)

// FetchSchedulingStrategies fetches scheduling strategies from the API server
func FetchSchedulingStrategies(apiUrl string) ([]SchedulingStrategy, error) {
	resp, err := http.Get(apiUrl)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response SchedulingStrategiesResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	// Only update if successful
	if response.Success {
		return response.Scheduling, nil
	}

	return nil, nil
}

// UpdateStrategyMap updates the strategy map from a slice of strategies
func UpdateStrategyMap(strategies []SchedulingStrategy) {
	// Create a new map to avoid concurrent access issues
	newMap := make(map[int32]SchedulingStrategy)

	for _, strategy := range strategies {
		newMap[int32(strategy.PID)] = strategy
	}

	// Replace the old map with the new one
	strategyMap = newMap

	log.Printf("Updated strategy map with %d strategies", len(newMap))
}

// StartStrategyFetcher starts a background goroutine to periodically fetch scheduling strategies
func StartStrategyFetcher(ctx context.Context, apiUrl string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		// Fetch immediately on start
		if strategies, err := FetchSchedulingStrategies(apiUrl); err == nil && strategies != nil {
			log.Printf("Initial scheduling strategies fetched: %d strategies", len(strategies))
			UpdateStrategyMap(strategies)
		} else if err != nil {
			log.Printf("Failed to fetch initial scheduling strategies: %v", err)
		}

		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-ticker.C:
				if strategies, err := FetchSchedulingStrategies(apiUrl); err == nil && strategies != nil {
					log.Printf("Scheduling strategies updated: %d strategies", len(strategies))
					UpdateStrategyMap(strategies)
				} else if err != nil {
					log.Printf("Failed to fetch scheduling strategies: %v", err)
				}
			}
		}
	}()
}

const SCX_ENQ_PREEMPT = 1 << 32

// ApplySchedulingStrategy applies scheduling strategies to a task
func ApplySchedulingStrategy(task *core.QueuedTask) bool {
	if strategy, exists := strategyMap[task.Pid]; exists {
		// Apply strategy
		if strategy.Priority {
			// Priority tasks get minimum vtime
			task.Vtime = minVruntime
			task.Flags |= SCX_ENQ_PREEMPT
		}

		return true
	}
	return false
}

// GetTaskExecutionTime returns the custom execution time for a task if defined
func GetTaskExecutionTime(pid int32) uint64 {
	if strategy, exists := strategyMap[pid]; exists && strategy.ExecutionTime > 0 {
		return strategy.ExecutionTime
	}
	return 0
}
