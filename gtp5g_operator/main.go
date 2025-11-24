package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/Gthulhu/Gthulhu/gtp5g_operator/pkg/api"
	"github.com/Gthulhu/Gthulhu/gtp5g_operator/pkg/parser"
)

const (
	// API endpoint (can be overridden by env var API_ENDPOINT)
	// For K8s deployment: "http://gthulhu-api:80"
	// For local testing with port-forward: "http://localhost:8081"
	defaultAPIEndpoint = "http://localhost:8081"
	
	// Public key path for JWT authentication
	defaultPublicKeyPath = "/home/ubuntu/Gthulhu/api/config/jwt_public_key.pem"
	
	// Send interval
	defaultSendInterval = 10 * time.Second
	
	// Priority enabled
	defaultPriorityEnabled = true
	
	// Time slice boost (20ms in nanoseconds)
	defaultTimeSliceNs = uint64(20000000) // 20ms
)

// PIDSet manages a thread-safe set of PIDs
type PIDSet struct {
	sync.RWMutex
	pids map[int]bool
}

// NewPIDSet creates a new PIDSet
func NewPIDSet() *PIDSet {
	return &PIDSet{
		pids: make(map[int]bool),
	}
}

// Add adds a PID to the set
func (ps *PIDSet) Add(pid int) {
	ps.Lock()
	defer ps.Unlock()
	ps.pids[pid] = true
}

// GetAll returns a copy of all PIDs
func (ps *PIDSet) GetAll() map[int]bool {
	ps.RLock()
	defer ps.RUnlock()
	
	copy := make(map[int]bool, len(ps.pids))
	for k, v := range ps.pids {
		copy[k] = v
	}
	return copy
}

// Size returns the number of PIDs
func (ps *PIDSet) Size() int {
	ps.RLock()
	defer ps.RUnlock()
	return len(ps.pids)
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func main() {
	log.Println("Starting GTP5G Operator...")

	// Read configuration from environment
	apiEndpoint := getEnv("API_ENDPOINT", defaultAPIEndpoint)
	publicKeyPath := getEnv("PUBLIC_KEY_PATH", defaultPublicKeyPath)
	
	log.Printf("API Endpoint: %s", apiEndpoint)
	log.Printf("Public Key Path: %s", publicKeyPath)

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialize components
	pidSet := NewPIDSet()
	traceParser := parser.NewTraceParser()
	apiClient := api.NewClient(apiEndpoint, publicKeyPath)

	// Channel for PIDs from trace parser
	pidChan := make(chan int, 100)

	// Goroutine 1: Parse trace_pipe
	go func() {
		for {
			log.Println("Starting trace_pipe parser...")
			if err := traceParser.StartTailing(ctx, pidChan); err != nil {
				if ctx.Err() != nil {
					// Context cancelled, exit gracefully
					return
				}
				log.Printf("Error tailing trace_pipe: %v, retrying in 5s...", err)
				time.Sleep(5 * time.Second)
			}
		}
	}()

	// Goroutine 2: Collect PIDs from channel
	go func() {
		for {
			select {
			case pid := <-pidChan:
				pidSet.Add(pid)
				log.Printf("Detected 5GC process PID: %d (total: %d)", pid, pidSet.Size())
			case <-ctx.Done():
				return
			}
		}
	}()

	// Goroutine 3: Periodic sender
	go func() {
		ticker := time.NewTicker(defaultSendInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				pids := pidSet.GetAll()
				if err := apiClient.SendStrategies(ctx, pids, defaultPriorityEnabled, defaultTimeSliceNs); err != nil {
					log.Printf("Error sending strategies: %v", err)
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	
	log.Println("GTP5G Operator is running. Press Ctrl+C to stop.")
	<-sigChan

	log.Println("Shutting down gracefully...")
	cancel()
	time.Sleep(1 * time.Second)
	log.Println("Shutdown complete.")
}
