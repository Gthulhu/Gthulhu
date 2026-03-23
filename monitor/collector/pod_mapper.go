// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"bufio"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PodRef is the minimal info needed to associate a PID with a Kubernetes pod.
type PodRef struct {
	PodName   string
	PodUID    string
	Namespace string
	NodeName  string
}

// PodMapper resolves PIDs to their owning Kubernetes pod by inspecting
// /proc/<pid>/cgroup and matching the pod UID embedded in the cgroup path.
//
// It maintains a cache that is refreshed periodically.
type PodMapper struct {
	logger   *slog.Logger
	procRoot string // typically "/proc"
	nodeName string

	mu       sync.RWMutex
	pidCache map[uint32]*PodRef // pid → pod
	podIndex map[string]*PodRef // podUID → pod (source of truth, set externally)
}

// NewPodMapper creates a PodMapper.
// podIndex should be populated (and refreshed) by the caller via SetPodIndex().
func NewPodMapper(nodeName string, logger *slog.Logger) *PodMapper {
	if logger == nil {
		logger = slog.Default()
	}
	return &PodMapper{
		logger:   logger,
		procRoot: "/proc",
		nodeName: nodeName,
		pidCache: make(map[uint32]*PodRef),
		podIndex: make(map[string]*PodRef),
	}
}

// SetPodIndex replaces the known set of pods on this node.
// Called externally when the CRD watch or K8S informer updates.
func (m *PodMapper) SetPodIndex(pods map[string]*PodRef) {
	m.mu.Lock()
	m.podIndex = pods
	// Invalidate PID cache — will be rebuilt on next lookup/scan
	m.pidCache = make(map[uint32]*PodRef)
	m.mu.Unlock()
}

// GetPodForPID returns the pod that owns a given PID, or nil if unknown.
func (m *PodMapper) GetPodForPID(pid uint32) *PodRef {
	m.mu.RLock()
	ref, ok := m.pidCache[pid]
	m.mu.RUnlock()
	if ok {
		return ref
	}

	// Cache miss — try to resolve via cgroup
	ref = m.resolvePIDtoPod(pid)
	if ref != nil {
		m.mu.Lock()
		m.pidCache[pid] = ref
		m.mu.Unlock()
	}
	return ref
}

// ScanAllPIDs performs a full /proc scan and populates the cache.
// Designed to be called periodically from a goroutine.
func (m *PodMapper) ScanAllPIDs() {
	entries, err := os.ReadDir(m.procRoot)
	if err != nil {
		m.logger.Warn("failed to read /proc", "error", err)
		return
	}

	newCache := make(map[uint32]*PodRef)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid64, err := strconv.ParseUint(entry.Name(), 10, 32)
		if err != nil {
			continue // not a PID directory
		}
		pid := uint32(pid64)
		ref := m.resolvePIDtoPod(pid)
		if ref != nil {
			newCache[pid] = ref
		}
	}

	m.mu.Lock()
	m.pidCache = newCache
	m.mu.Unlock()

	m.logger.Debug("ScanAllPIDs complete", "mapped", len(newCache))
}

// StartPeriodicScan launches a goroutine that calls ScanAllPIDs at the given interval.
func (m *PodMapper) StartPeriodicScan(interval time.Duration, done <-chan struct{}) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				m.ScanAllPIDs()
			}
		}
	}()
}

// resolvePIDtoPod reads /proc/<pid>/cgroup and extracts the pod UID,
// then looks it up in the pod index.
func (m *PodMapper) resolvePIDtoPod(pid uint32) *PodRef {
	cgroupPath := filepath.Join(m.procRoot, strconv.FormatUint(uint64(pid), 10), "cgroup")
	f, err := os.Open(cgroupPath)
	if err != nil {
		return nil
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		podUID := extractPodUID(line)
		if podUID == "" {
			continue
		}

		m.mu.RLock()
		ref, ok := m.podIndex[podUID]
		m.mu.RUnlock()
		if ok {
			return ref
		}
	}
	return nil
}

// extractPodUID parses a cgroup v1/v2 line and returns the embedded pod UID.
// Example cgroup v2 line:
//
//	0::/kubepods/burstable/pod<uid>/<container-id>
//
// Example cgroup v1 line:
//
//	12:memory:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-pod<uid>.slice/...
func extractPodUID(line string) string {
	// cgroup v2 style: look for "pod" prefix followed by UID
	if idx := strings.Index(line, "/pod"); idx != -1 {
		rest := line[idx+4:] // skip "/pod"
		// UID ends at next '/' or end of line
		if slashIdx := strings.IndexByte(rest, '/'); slashIdx != -1 {
			return normalizePodUID(rest[:slashIdx])
		}
		return normalizePodUID(rest)
	}

	// cgroup v1 systemd slice style: kubepods-burstable-pod<uid>.slice
	if idx := strings.Index(line, "-pod"); idx != -1 {
		rest := line[idx+4:]
		if dotIdx := strings.Index(rest, ".slice"); dotIdx != -1 {
			return normalizePodUID(rest[:dotIdx])
		}
	}
	return ""
}

// normalizePodUID converts underscores to dashes (systemd cgroup encoding).
func normalizePodUID(raw string) string {
	return strings.ReplaceAll(raw, "_", "-")
}

// ListMappedPIDs returns all PIDs currently mapped to pods (for BPF map sync).
func (m *PodMapper) ListMappedPIDs() []uint32 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pids := make([]uint32, 0, len(m.pidCache))
	for pid := range m.pidCache {
		pids = append(pids, pid)
	}
	return pids
}

// GetAllPodRefs returns all known pod refs.
func (m *PodMapper) GetAllPodRefs() map[string]*PodRef {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make(map[string]*PodRef, len(m.podIndex))
	for k, v := range m.podIndex {
		out[k] = v
	}
	return out
}

// String implements fmt.Stringer for debugging.
func (m *PodMapper) String() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return fmt.Sprintf("PodMapper{pods=%d, pidCache=%d}", len(m.podIndex), len(m.pidCache))
}
