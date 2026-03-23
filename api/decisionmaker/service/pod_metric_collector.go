package service

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
)

var _ prometheus.Collector = (*PodSchedMetricCollector)(nil)

// PodSchedMetricCollector reads /proc scheduling stats for pods resolved from intents
// and exposes them as Prometheus metrics on the DM's /metrics endpoint.
type PodSchedMetricCollector struct {
	nodeName string

	voluntaryCtxSwitchesDesc   *prometheus.Desc
	involuntaryCtxSwitchesDesc *prometheus.Desc
	cpuTimeNsDesc              *prometheus.Desc
	waitTimeNsDesc             *prometheus.Desc
	runCountDesc               *prometheus.Desc
	cpuMigrationsDesc          *prometheus.Desc
	processCountDesc           *prometheus.Desc

	// mu guards intentPods
	mu         sync.RWMutex
	intentPods []podTarget
}

// podTarget represents a resolved pod with its PIDs for metric collection.
type podTarget struct {
	PodName   string
	PodUID    string
	Namespace string
	NodeName  string
	PIDs      []int
}

var podSchedMetricLabels = []string{"pod_name", "pod_uid", "namespace", "node_name"}

// NewPodSchedMetricCollector creates a collector that reads /proc/PID/sched
// for each pod process discovered from scheduling intents.
func NewPodSchedMetricCollector(nodeName string) *PodSchedMetricCollector {
	return &PodSchedMetricCollector{
		nodeName: nodeName,
		voluntaryCtxSwitchesDesc: prometheus.NewDesc(
			"gthulhu_pod_voluntary_ctx_switches_total",
			"Total voluntary context switches for all processes in a pod",
			podSchedMetricLabels, nil,
		),
		involuntaryCtxSwitchesDesc: prometheus.NewDesc(
			"gthulhu_pod_involuntary_ctx_switches_total",
			"Total involuntary context switches for all processes in a pod",
			podSchedMetricLabels, nil,
		),
		cpuTimeNsDesc: prometheus.NewDesc(
			"gthulhu_pod_cpu_time_nanoseconds_total",
			"Total CPU time in nanoseconds for all processes in a pod",
			podSchedMetricLabels, nil,
		),
		waitTimeNsDesc: prometheus.NewDesc(
			"gthulhu_pod_wait_time_nanoseconds_total",
			"Total wait (runqueue) time in nanoseconds for all processes in a pod",
			podSchedMetricLabels, nil,
		),
		runCountDesc: prometheus.NewDesc(
			"gthulhu_pod_run_count_total",
			"Total number of times processes in a pod were scheduled on CPU",
			podSchedMetricLabels, nil,
		),
		cpuMigrationsDesc: prometheus.NewDesc(
			"gthulhu_pod_cpu_migrations_total",
			"Total CPU migrations for all processes in a pod",
			podSchedMetricLabels, nil,
		),
		processCountDesc: prometheus.NewDesc(
			"gthulhu_pod_process_count",
			"Number of tracked processes in a pod",
			podSchedMetricLabels, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (c *PodSchedMetricCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.voluntaryCtxSwitchesDesc
	ch <- c.involuntaryCtxSwitchesDesc
	ch <- c.cpuTimeNsDesc
	ch <- c.waitTimeNsDesc
	ch <- c.runCountDesc
	ch <- c.cpuMigrationsDesc
	ch <- c.processCountDesc
}

// Collect implements prometheus.Collector. It reads /proc for each tracked pod.
func (c *PodSchedMetricCollector) Collect(ch chan<- prometheus.Metric) {
	c.mu.RLock()
	pods := c.intentPods
	c.mu.RUnlock()

	for _, pod := range pods {
		var (
			totalVoluntary   uint64
			totalInvoluntary uint64
			totalCPUTimeNs   uint64
			totalWaitTimeNs  uint64
			totalRunCount    uint64
			totalMigrations  uint64
			processCount     int
		)

		for _, pid := range pod.PIDs {
			stats, err := readProcSchedStats(pid)
			if err != nil {
				continue // process may have exited
			}
			totalVoluntary += stats.VoluntaryCtxSwitches
			totalInvoluntary += stats.InvoluntaryCtxSwitches
			totalCPUTimeNs += stats.CPUTimeNs
			totalWaitTimeNs += stats.WaitTimeNs
			totalRunCount += stats.RunCount
			totalMigrations += stats.CPUMigrations
			processCount++
		}

		if processCount == 0 {
			continue
		}

		labels := []string{pod.PodName, pod.PodUID, pod.Namespace, pod.NodeName}
		ch <- prometheus.MustNewConstMetric(c.voluntaryCtxSwitchesDesc, prometheus.CounterValue, float64(totalVoluntary), labels...)
		ch <- prometheus.MustNewConstMetric(c.involuntaryCtxSwitchesDesc, prometheus.CounterValue, float64(totalInvoluntary), labels...)
		ch <- prometheus.MustNewConstMetric(c.cpuTimeNsDesc, prometheus.CounterValue, float64(totalCPUTimeNs), labels...)
		ch <- prometheus.MustNewConstMetric(c.waitTimeNsDesc, prometheus.CounterValue, float64(totalWaitTimeNs), labels...)
		ch <- prometheus.MustNewConstMetric(c.runCountDesc, prometheus.CounterValue, float64(totalRunCount), labels...)
		ch <- prometheus.MustNewConstMetric(c.cpuMigrationsDesc, prometheus.CounterValue, float64(totalMigrations), labels...)
		ch <- prometheus.MustNewConstMetric(c.processCountDesc, prometheus.GaugeValue, float64(processCount), labels...)
	}
}

// UpdatePodTargets replaces the list of pods to collect metrics for.
// Called after intent resolution to keep the metric targets up-to-date.
func (c *PodSchedMetricCollector) UpdatePodTargets(intents []*domain.Intent, podInfos map[string]*domain.PodInfo) {
	var targets []podTarget

	for _, intent := range intents {
		if intent == nil {
			continue
		}
		podInfo, ok := podInfos[intent.PodID]
		if !ok || len(podInfo.Processes) == 0 {
			continue
		}

		var pids []int
		for _, proc := range podInfo.Processes {
			if proc.Command == "pause" {
				continue
			}
			pids = append(pids, proc.PID)
		}

		if len(pids) == 0 {
			continue
		}

		targets = append(targets, podTarget{
			PodName:   intent.PodName,
			PodUID:    intent.PodID,
			Namespace: intent.K8sNamespace,
			NodeName:  firstNonEmptyStr(intent.NodeID, c.nodeName),
			PIDs:      pids,
		})
	}

	c.mu.Lock()
	c.intentPods = targets
	c.mu.Unlock()
}

// procSchedStats holds scheduling stats read from /proc.
type procSchedStats struct {
	VoluntaryCtxSwitches   uint64
	InvoluntaryCtxSwitches uint64
	CPUTimeNs              uint64
	WaitTimeNs             uint64
	RunCount               uint64
	CPUMigrations          uint64
}

// readProcSchedStats reads scheduling metrics from /proc/<pid>/schedstat and /proc/<pid>/sched.
func readProcSchedStats(pid int) (*procSchedStats, error) {
	stats := &procSchedStats{}

	// /proc/<pid>/schedstat: "cpu_time_ns wait_time_ns run_count"
	schedstatPath := fmt.Sprintf("/proc/%d/schedstat", pid)
	if data, err := os.ReadFile(schedstatPath); err == nil {
		fields := strings.Fields(strings.TrimSpace(string(data)))
		if len(fields) >= 3 {
			stats.CPUTimeNs, _ = strconv.ParseUint(fields[0], 10, 64)
			stats.WaitTimeNs, _ = strconv.ParseUint(fields[1], 10, 64)
			stats.RunCount, _ = strconv.ParseUint(fields[2], 10, 64)
		}
	}

	// /proc/<pid>/sched: key-value pairs for context switches and migrations
	schedPath := fmt.Sprintf("/proc/%d/sched", pid)
	if file, err := os.Open(schedPath); err == nil {
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			valStr := strings.TrimSpace(parts[1])
			switch key {
			case "nr_voluntary_switches":
				stats.VoluntaryCtxSwitches, _ = strconv.ParseUint(valStr, 10, 64)
			case "nr_involuntary_switches":
				stats.InvoluntaryCtxSwitches, _ = strconv.ParseUint(valStr, 10, 64)
			case "se.nr_migrations":
				stats.CPUMigrations, _ = strconv.ParseUint(valStr, 10, 64)
			}
		}
	}

	// Fallback: if /proc/<pid>/sched is not available, try /proc/<pid>/status
	if stats.VoluntaryCtxSwitches == 0 && stats.InvoluntaryCtxSwitches == 0 {
		statusPath := fmt.Sprintf("/proc/%d/status", pid)
		if file, err := os.Open(statusPath); err == nil {
			defer file.Close()
			scanner := bufio.NewScanner(file)
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "voluntary_ctxt_switches:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(line, "voluntary_ctxt_switches:"))
					stats.VoluntaryCtxSwitches, _ = strconv.ParseUint(valStr, 10, 64)
				} else if strings.HasPrefix(line, "nonvoluntary_ctxt_switches:") {
					valStr := strings.TrimSpace(strings.TrimPrefix(line, "nonvoluntary_ctxt_switches:"))
					stats.InvoluntaryCtxSwitches, _ = strconv.ParseUint(valStr, 10, 64)
				}
			}
		}
	}

	return stats, nil
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// GetMachineID returns the machine ID for use as node name.
func GetMachineID() string {
	return util.GetMachineID()
}
