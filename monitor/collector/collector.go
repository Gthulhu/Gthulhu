// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

// Package collector loads the scheduling-monitor eBPF program, reads per-PID
// metrics from BPF maps, and exposes aggregated pod-level Prometheus metrics.
package collector

/*
#include "../bpf/sched_monitor.h"
*/
import "C"

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/Gthulhu/api/decisionmaker/domain"
	bpf "github.com/aquasecurity/libbpfgo"
)

// Config controls the collector behaviour.
type Config struct {
	BPFObjectPath           string        // path to compiled sched_monitor.bpf.o
	PollInterval            time.Duration // how often to read BPF maps (default 10s)
	MonitorAll              bool          // mirror of the BPF global monitor_all flag
	StreamEvents            bool          // mirror of the BPF global stream_events flag
	TopologyRefreshInterval time.Duration // how often to refresh CPU topology map (default 5m)
}

// Collector owns the eBPF lifecycle, reads maps, and provides aggregated data.
type Collector struct {
	cfg    Config
	module *bpf.Module
	logger *slog.Logger

	// BPF maps
	taskMetricsMap *bpf.BPFMap
	monitoredPIDs  *bpf.BPFMap
	monitoredTGIDs *bpf.BPFMap
	cpuTopologyMap *bpf.BPFMap

	// Pod mapper
	podMapper *PodMapper

	// Latest aggregated pod metrics (protected by mu)
	mu         sync.RWMutex
	podMetrics map[string]*domain.PodSchedMetrics // key = podUID
}

// New creates a Collector; call Start() to begin.
func New(cfg Config, podMapper *PodMapper, logger *slog.Logger) *Collector {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 10 * time.Second
	}
	if cfg.TopologyRefreshInterval == 0 {
		cfg.TopologyRefreshInterval = 5 * time.Minute
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Collector{
		cfg:        cfg,
		podMapper:  podMapper,
		logger:     logger,
		podMetrics: make(map[string]*domain.PodSchedMetrics),
	}
}

// Start loads the BPF program, attaches probes, and starts the poll loop.
// It blocks until ctx is cancelled.
func (c *Collector) Start(ctx context.Context) error {
	if err := c.loadBPF(); err != nil {
		return fmt.Errorf("loadBPF: %w", err)
	}
	defer c.module.Close()
	c.logger.Info("sched_monitor BPF program loaded", "object", c.cfg.BPFObjectPath)

	ticker := time.NewTicker(c.cfg.PollInterval)
	defer ticker.Stop()
	topologyTicker := time.NewTicker(c.cfg.TopologyRefreshInterval)
	defer topologyTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			c.logger.Info("collector shutting down")
			return nil
		case <-ticker.C:
			c.poll()
		case <-topologyTicker.C:
			if err := c.injectCPUTopology(); err != nil {
				c.logger.Warn("failed to refresh cpu topology map", "error", err)
			}
		}
	}
}

// GetPodMetrics returns a snapshot of the latest pod-level metrics.
func (c *Collector) GetPodMetrics() map[string]*domain.PodSchedMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]*domain.PodSchedMetrics, len(c.podMetrics))
	for k, v := range c.podMetrics {
		clone := *v
		out[k] = &clone
	}
	return out
}

// AddMonitoredPID inserts a PID into the BPF monitored_pids map.
func (c *Collector) AddMonitoredPID(pid uint32) error {
	if c.monitoredPIDs == nil {
		return fmt.Errorf("BPF not loaded")
	}
	val := uint8(1)
	return c.monitoredPIDs.Update(unsafe.Pointer(&pid), unsafe.Pointer(&val))
}

// RemoveMonitoredPID removes a PID from the BPF monitored_pids map.
func (c *Collector) RemoveMonitoredPID(pid uint32) error {
	if c.monitoredPIDs == nil {
		return fmt.Errorf("BPF not loaded")
	}
	return c.monitoredPIDs.DeleteKey(unsafe.Pointer(&pid))
}

// AddMonitoredTGID inserts a TGID into the BPF monitored_tgids map.
func (c *Collector) AddMonitoredTGID(tgid uint32) error {
	if c.monitoredTGIDs == nil {
		return fmt.Errorf("BPF not loaded")
	}
	val := uint8(1)
	return c.monitoredTGIDs.Update(unsafe.Pointer(&tgid), unsafe.Pointer(&val))
}

// RemoveMonitoredTGID removes a TGID from the BPF monitored_tgids map.
func (c *Collector) RemoveMonitoredTGID(tgid uint32) error {
	if c.monitoredTGIDs == nil {
		return fmt.Errorf("BPF not loaded")
	}
	return c.monitoredTGIDs.DeleteKey(unsafe.Pointer(&tgid))
}

// ========================== internal ==========================

func (c *Collector) loadBPF() error {
	mod, err := bpf.NewModuleFromFile(c.cfg.BPFObjectPath)
	if err != nil {
		return fmt.Errorf("NewModuleFromFile: %w", err)
	}
	c.module = mod

	// Set global variables before loading
	if err := mod.InitGlobalVariable("monitor_all", c.cfg.MonitorAll); err != nil {
		c.logger.Warn("failed to set monitor_all global", "error", err)
	}
	if err := mod.InitGlobalVariable("stream_events", c.cfg.StreamEvents); err != nil {
		c.logger.Warn("failed to set stream_events global", "error", err)
	}

	if err := mod.BPFLoadObject(); err != nil {
		return fmt.Errorf("BPFLoadObject: %w", err)
	}

	// Attach tracepoints
	if _, err := mod.GetProgram("handle_sched_switch"); err != nil {
		return fmt.Errorf("get handle_sched_switch: %w", err)
	}
	if _, err := mod.GetProgram("handle_sched_process_exit"); err != nil {
		return fmt.Errorf("get handle_sched_process_exit: %w", err)
	}

	// Auto-attach all programs (tp_btf programs auto-attach by name)
	iter := mod.Iterator()
	for {
		prog := iter.NextProgram()
		if prog == nil {
			break
		}
		if _, err := prog.AttachGeneric(); err != nil {
			return fmt.Errorf("attach %s: %w", prog.Name(), err)
		}
		c.logger.Info("attached BPF program", "name", prog.Name())
	}

	// Grab map references
	c.taskMetricsMap, err = mod.GetMap("task_metrics")
	if err != nil {
		return fmt.Errorf("get task_metrics map: %w", err)
	}
	c.monitoredPIDs, err = mod.GetMap("monitored_pids")
	if err != nil {
		return fmt.Errorf("get monitored_pids map: %w", err)
	}
	c.monitoredTGIDs, err = mod.GetMap("monitored_tgids")
	if err != nil {
		return fmt.Errorf("get monitored_tgids map: %w", err)
	}
	c.cpuTopologyMap, err = mod.GetMap("cpu_topology_map")
	if err != nil {
		return fmt.Errorf("get cpu_topology_map map: %w", err)
	}
	if err := c.injectCPUTopology(); err != nil {
		c.logger.Warn("failed to inject cpu topology map", "error", err)
	}

	return nil
}

// poll reads all entries from the BPF task_metrics map and aggregates by pod.
func (c *Collector) poll() {
	if c.taskMetricsMap == nil {
		return
	}

	// Collect all per-PID metrics from BPF
	pidMetrics := make(map[uint32]*domain.TaskSchedMetrics)
	iter := c.taskMetricsMap.Iterator()
	for iter.Next() {
		keyBytes := iter.Key()
		if len(keyBytes) < 4 {
			continue
		}
		pid := *(*uint32)(unsafe.Pointer(&keyBytes[0]))

		valBytes, err := c.taskMetricsMap.GetValue(unsafe.Pointer(&pid))
		if err != nil {
			continue
		}
		bpfMetrics := (*C.struct_task_sched_metrics)(unsafe.Pointer(&valBytes[0]))

		pidMetrics[pid] = &domain.TaskSchedMetrics{
			PID:                    uint32(bpfMetrics.pid),
			TGID:                   uint32(bpfMetrics.tgid),
			VoluntaryCtxSwitches:   uint64(bpfMetrics.voluntary_ctx_switches),
			InvoluntaryCtxSwitches: uint64(bpfMetrics.involuntary_ctx_switches),
			CpuTimeNs:              uint64(bpfMetrics.cpu_time_ns),
			WaitTimeNs:             uint64(bpfMetrics.wait_time_ns),
			RunCount:               uint64(bpfMetrics.run_count),
			CpuMigrations:          uint32(bpfMetrics.cpu_migrations),
			SMTMigrations:          uint32(bpfMetrics.smt_migrations),
			L3Migrations:           uint32(bpfMetrics.l3_migrations),
			NUMAMigrations:         uint32(bpfMetrics.numa_migrations),
			LastCPU:                uint32(bpfMetrics.last_cpu),
		}
	}

	// Aggregate by pod
	podAgg := make(map[string]*domain.PodSchedMetrics)
	for pid, tm := range pidMetrics {
		podInfo := c.podMapper.GetPodForPID(pid)
		if podInfo == nil {
			continue // PID not associated with any known pod
		}
		agg, exists := podAgg[podInfo.PodUID]
		if !exists {
			agg = &domain.PodSchedMetrics{
				PodName:   podInfo.PodName,
				PodUID:    podInfo.PodUID,
				Namespace: podInfo.Namespace,
				NodeName:  podInfo.NodeName,
			}
			podAgg[podInfo.PodUID] = agg
		}
		agg.VoluntaryCtxSwitches += tm.VoluntaryCtxSwitches
		agg.InvoluntaryCtxSwitches += tm.InvoluntaryCtxSwitches
		agg.CpuTimeNs += tm.CpuTimeNs
		agg.WaitTimeNs += tm.WaitTimeNs
		agg.RunCount += tm.RunCount
		agg.CpuMigrations += tm.CpuMigrations
		agg.SMTMigrations += tm.SMTMigrations
		agg.L3Migrations += tm.L3Migrations
		agg.NUMAMigrations += tm.NUMAMigrations
		agg.ProcessCount++
	}

	// Publish
	c.mu.Lock()
	c.podMetrics = podAgg
	c.mu.Unlock()
	c.logger.Debug("poll complete", "pids", len(pidMetrics), "pods", len(podAgg))
}

type cpuTopologyInfo struct {
	CoreID    uint32
	PackageID uint32
	NUMAID    uint32
	LLCID     uint32
	Valid     uint8
	Pad       [3]uint8
}

func (c *Collector) injectCPUTopology() error {
	if c.cpuTopologyMap == nil {
		return nil
	}

	cpuDirs, err := filepath.Glob("/sys/devices/system/cpu/cpu[0-9]*")
	if err != nil {
		return fmt.Errorf("glob cpu topology dirs: %w", err)
	}

	for _, cpuDir := range cpuDirs {
		cpuID, err := parseCPUID(cpuDir)
		if err != nil {
			continue
		}

		coreID, err := readUint32File(filepath.Join(cpuDir, "topology/core_id"))
		if err != nil {
			continue
		}
		packageID, err := readUint32File(filepath.Join(cpuDir, "topology/physical_package_id"))
		if err != nil {
			continue
		}

		numaID := packageID
		if parsedNUMAID, err := readNUMAID(cpuDir); err == nil {
			numaID = parsedNUMAID
		}

		llcID := uint32(0)
		if parsedLLCID, err := readUint32File(filepath.Join(cpuDir, "cache/index3/id")); err == nil {
			llcID = parsedLLCID
		}

		key := cpuID
		value := cpuTopologyInfo{
			CoreID:    coreID,
			PackageID: packageID,
			NUMAID:    numaID,
			LLCID:     llcID,
			Valid:     1,
		}
		if err := c.cpuTopologyMap.Update(unsafe.Pointer(&key), unsafe.Pointer(&value)); err != nil {
			c.logger.Debug("failed to update cpu topology entry", "cpu", cpuID, "error", err)
		}
	}

	return nil
}

func parseCPUID(cpuDir string) (uint32, error) {
	base := filepath.Base(cpuDir)
	if !strings.HasPrefix(base, "cpu") {
		return 0, fmt.Errorf("invalid cpu dir: %s", cpuDir)
	}
	id, err := strconv.ParseUint(strings.TrimPrefix(base, "cpu"), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse cpu id %s: %w", cpuDir, err)
	}
	return uint32(id), nil
}

func readNUMAID(cpuDir string) (uint32, error) {
	nodeDirs, err := filepath.Glob(filepath.Join(cpuDir, "node*"))
	if err != nil || len(nodeDirs) == 0 {
		return 0, fmt.Errorf("no numa node for %s", cpuDir)
	}
	base := filepath.Base(nodeDirs[0])
	id, err := strconv.ParseUint(strings.TrimPrefix(base, "node"), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("parse numa id for %s: %w", cpuDir, err)
	}
	return uint32(id), nil
}

func readUint32File(path string) (uint32, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	value, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 32)
	if err != nil {
		return 0, err
	}
	return uint32(value), nil
}
