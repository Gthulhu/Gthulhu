package sched

import (
	"sync"
	"time"
)

const (
	// Priority CPU tracking window - 1ms
	PRIORITY_CPU_TRACK_WINDOW = 1 * time.Millisecond
	// Maximum number of tracked entries to prevent memory bloat
	MAX_TRACKED_ENTRIES = 1000
)

// PriorityCPUEntry represents a single CPU usage entry by a priority task
type PriorityCPUEntry struct {
	CPU       int32
	Timestamp time.Time
	TaskPID   int32
}

// PriorityCPUTracker tracks CPU usage by priority tasks
type PriorityCPUTracker struct {
	entries []PriorityCPUEntry
	mutex   sync.RWMutex
}

// Global tracker instance
var priorityCPUTracker = &PriorityCPUTracker{
	entries: make([]PriorityCPUEntry, 0, MAX_TRACKED_ENTRIES),
}

// RecordPriorityCPUUsage records when a priority task uses a CPU
func RecordPriorityCPUUsage(cpu int32, taskPID int32) {
	priorityCPUTracker.mutex.Lock()
	defer priorityCPUTracker.mutex.Unlock()

	now := time.Now()
	entry := PriorityCPUEntry{
		CPU:       cpu,
		Timestamp: now,
		TaskPID:   taskPID,
	}

	// Add new entry
	priorityCPUTracker.entries = append(priorityCPUTracker.entries, entry)

	// Clean up old entries that are outside the tracking window
	cutoffTime := now.Add(-PRIORITY_CPU_TRACK_WINDOW)
	validEntries := make([]PriorityCPUEntry, 0, len(priorityCPUTracker.entries))

	for _, e := range priorityCPUTracker.entries {
		if e.Timestamp.After(cutoffTime) {
			validEntries = append(validEntries, e)
		}
	}

	priorityCPUTracker.entries = validEntries

	// Prevent memory bloat by limiting the number of entries
	if len(priorityCPUTracker.entries) > MAX_TRACKED_ENTRIES {
		// Keep only the most recent entries
		start := len(priorityCPUTracker.entries) - MAX_TRACKED_ENTRIES
		priorityCPUTracker.entries = priorityCPUTracker.entries[start:]
	}
}

// GetRecentPriorityCPUs returns a set of CPUs that have been used by priority tasks
// within the last 10ms
func GetRecentPriorityCPUs() map[int32]bool {
	priorityCPUTracker.mutex.RLock()
	defer priorityCPUTracker.mutex.RUnlock()

	now := time.Now()
	cutoffTime := now.Add(-PRIORITY_CPU_TRACK_WINDOW)
	recentCPUs := make(map[int32]bool)

	for _, entry := range priorityCPUTracker.entries {
		if entry.Timestamp.After(cutoffTime) {
			recentCPUs[entry.CPU] = true
		}
	}

	return recentCPUs
}

// IsTaskPriority checks if a task should be considered as priority based on strategy
func IsTaskPriority(pid int32) bool {
	if strategy, exists := strategyMap[pid]; exists && strategy.Priority {
		return true
	}
	return false
}

// ShouldAvoidCPU checks if a CPU should be avoided for non-priority tasks
func ShouldAvoidCPU(cpu int32, taskPID int32) bool {
	// If this is a priority task, it can use any CPU
	if IsTaskPriority(taskPID) {
		return false
	}

	// For non-priority tasks, check if the CPU was recently used by priority tasks
	recentPriorityCPUs := GetRecentPriorityCPUs()
	return recentPriorityCPUs[cpu]
}

// GetAvailableCPUsForTask returns a list of CPUs that are suitable for the task
// considering priority CPU avoidance
func GetAvailableCPUsForTask(taskPID int32, totalCPUs int32) []int32 {
	availableCPUs := make([]int32, 0, totalCPUs)

	for cpu := int32(0); cpu < totalCPUs; cpu++ {
		if !ShouldAvoidCPU(cpu, taskPID) {
			availableCPUs = append(availableCPUs, cpu)
		}
	}

	return availableCPUs
}

// GetTrackerStats returns statistics about the tracker
func GetTrackerStats() (int, int) {
	priorityCPUTracker.mutex.RLock()
	defer priorityCPUTracker.mutex.RUnlock()

	now := time.Now()
	cutoffTime := now.Add(-PRIORITY_CPU_TRACK_WINDOW)
	recentCount := 0

	for _, entry := range priorityCPUTracker.entries {
		if entry.Timestamp.After(cutoffTime) {
			recentCount++
		}
	}

	return len(priorityCPUTracker.entries), recentCount
}
