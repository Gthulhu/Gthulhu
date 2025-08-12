package sched

import (
	"testing"
	"time"
)

func TestPriorityCPUTracker(t *testing.T) {
	priorityCPUTracker.mutex.Lock()
	priorityCPUTracker.entries = priorityCPUTracker.entries[:0]
	priorityCPUTracker.mutex.Unlock()

	t.Run("RecordAndRetrieve", func(t *testing.T) {
		RecordPriorityCPUUsage(0, 1001)
		RecordPriorityCPUUsage(1, 1002)
		RecordPriorityCPUUsage(2, 1003)

		recentCPUs := GetRecentPriorityCPUs()

		if !recentCPUs[0] {
			t.Error("CPU 0 should be in recent CPUs")
		}
		if !recentCPUs[1] {
			t.Error("CPU 1 should be in recent CPUs")
		}
		if !recentCPUs[2] {
			t.Error("CPU 2 should be in recent CPUs")
		}
	})

	t.Run("TimeWindowExpiration", func(t *testing.T) {
		priorityCPUTracker.mutex.Lock()
		priorityCPUTracker.entries = priorityCPUTracker.entries[:0]
		priorityCPUTracker.mutex.Unlock()

		RecordPriorityCPUUsage(0, 1001)

		recentCPUs := GetRecentPriorityCPUs()
		if !recentCPUs[0] {
			t.Error("CPU 0 should be in recent CPUs immediately after recording")
		}

		time.Sleep(12 * time.Millisecond)

		recentCPUs = GetRecentPriorityCPUs()
		if recentCPUs[0] {
			t.Error("CPU 0 should not be in recent CPUs after expiration")
		}
	})

	t.Run("ShouldAvoidCPU", func(t *testing.T) {
		priorityCPUTracker.mutex.Lock()
		priorityCPUTracker.entries = priorityCPUTracker.entries[:0]
		priorityCPUTracker.mutex.Unlock()

		strategyMap[1001] = SchedulingStrategy{Priority: true, PID: 1001}
		defer delete(strategyMap, 1001)

		RecordPriorityCPUUsage(0, 1001)

		if ShouldAvoidCPU(0, 1001) {
			t.Error("Priority task should not avoid any CPU")
		}

		if !ShouldAvoidCPU(0, 2001) {
			t.Error("Non-priority task should avoid recently used CPU")
		}

		if ShouldAvoidCPU(1, 2001) {
			t.Error("Non-priority task should not avoid unused CPU")
		}
	})

	t.Run("GetAvailableCPUsForTask", func(t *testing.T) {
		priorityCPUTracker.mutex.Lock()
		priorityCPUTracker.entries = priorityCPUTracker.entries[:0]
		priorityCPUTracker.mutex.Unlock()

		strategyMap[1001] = SchedulingStrategy{Priority: true, PID: 1001}
		defer delete(strategyMap, 1001)

		RecordPriorityCPUUsage(0, 1001)
		RecordPriorityCPUUsage(1, 1001)

		availableCPUs := GetAvailableCPUsForTask(2001, 4)
		expectedCPUs := []int32{2, 3}

		if len(availableCPUs) != len(expectedCPUs) {
			t.Errorf("Expected %d CPUs, got %d", len(expectedCPUs), len(availableCPUs))
		}

		for i, cpu := range expectedCPUs {
			if i >= len(availableCPUs) || availableCPUs[i] != cpu {
				t.Errorf("Expected CPU %d at position %d, got %v", cpu, i, availableCPUs)
			}
		}
	})

	t.Run("AntiStarvation", func(t *testing.T) {
		priorityCPUTracker.mutex.Lock()
		priorityCPUTracker.entries = priorityCPUTracker.entries[:0]
		priorityCPUTracker.mutex.Unlock()

		strategyMap[1001] = SchedulingStrategy{Priority: true, PID: 1001}
		defer delete(strategyMap, 1001)

		for cpu := int32(0); cpu < 4; cpu++ {
			RecordPriorityCPUUsage(cpu, 1001)
		}

		availableCPUs := GetAvailableCPUsForTask(2001, 4)

		if len(availableCPUs) != 4 {
			t.Errorf("Anti-starvation: expected all 4 CPUs to be available, got %d", len(availableCPUs))
		}
	})

	t.Run("GetTrackerStats", func(t *testing.T) {
		priorityCPUTracker.mutex.Lock()
		priorityCPUTracker.entries = priorityCPUTracker.entries[:0]
		priorityCPUTracker.mutex.Unlock()

		RecordPriorityCPUUsage(0, 1001)
		RecordPriorityCPUUsage(1, 1002)

		total, recent := GetTrackerStats()

		if total != 2 {
			t.Errorf("Expected 2 total entries, got %d", total)
		}
		if recent != 2 {
			t.Errorf("Expected 2 recent entries, got %d", recent)
		}

		time.Sleep(12 * time.Millisecond)

		total, recent = GetTrackerStats()
		if recent != 0 {
			t.Errorf("Expected 0 recent entries after expiration, got %d", recent)
		}
	})
}

func TestIsTaskPriority(t *testing.T) {
	strategyMap[1001] = SchedulingStrategy{Priority: true, PID: 1001}
	strategyMap[1002] = SchedulingStrategy{Priority: false, PID: 1002}
	defer delete(strategyMap, 1001)
	defer delete(strategyMap, 1002)

	if !IsTaskPriority(1001) {
		t.Error("Task 1001 should be priority")
	}
	if IsTaskPriority(1002) {
		t.Error("Task 1002 should not be priority")
	}
	if IsTaskPriority(1003) {
		t.Error("Task 1003 (unknown) should not be priority")
	}
}
