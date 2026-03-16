// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/prometheus/client_golang/prometheus"
)

const metricsNamespace = "gthulhu"
const metricsSubsystem = "pod"

// PodSchedMetricsCollector implements prometheus.Collector and exposes
// pod-level scheduling metrics gathered by the eBPF Collector.
type PodSchedMetricsCollector struct {
	collector *Collector

	voluntaryCtxSwitches   *prometheus.Desc
	involuntaryCtxSwitches *prometheus.Desc
	cpuTimeNs              *prometheus.Desc
	waitTimeNs             *prometheus.Desc
	runCount               *prometheus.Desc
	cpuMigrations          *prometheus.Desc
	processCount           *prometheus.Desc
}

var _ prometheus.Collector = (*PodSchedMetricsCollector)(nil)

// NewPodSchedMetricsCollector creates a Prometheus collector backed by
// the eBPF Collector's aggregated pod metrics.
func NewPodSchedMetricsCollector(c *Collector) *PodSchedMetricsCollector {
	labels := []string{"pod_name", "pod_uid", "namespace", "node_name"}

	return &PodSchedMetricsCollector{
		collector: c,

		voluntaryCtxSwitches: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "voluntary_ctx_switches_total"),
			"Total voluntary context switches for all processes in a pod",
			labels, nil,
		),
		involuntaryCtxSwitches: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "involuntary_ctx_switches_total"),
			"Total involuntary context switches for all processes in a pod",
			labels, nil,
		),
		cpuTimeNs: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "cpu_time_nanoseconds_total"),
			"Total CPU time consumed by all processes in a pod (nanoseconds)",
			labels, nil,
		),
		waitTimeNs: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "wait_time_nanoseconds_total"),
			"Total run-queue wait time for all processes in a pod (nanoseconds)",
			labels, nil,
		),
		runCount: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "run_count_total"),
			"Total number of times processes in a pod were scheduled on a CPU",
			labels, nil,
		),
		cpuMigrations: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "cpu_migrations_total"),
			"Total CPU migration count for processes in a pod",
			labels, nil,
		),
		processCount: prometheus.NewDesc(
			prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "process_count"),
			"Number of processes currently tracked for this pod",
			labels, nil,
		),
	}
}

// Describe implements prometheus.Collector.
func (p *PodSchedMetricsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- p.voluntaryCtxSwitches
	ch <- p.involuntaryCtxSwitches
	ch <- p.cpuTimeNs
	ch <- p.waitTimeNs
	ch <- p.runCount
	ch <- p.cpuMigrations
	ch <- p.processCount
}

// Collect implements prometheus.Collector.
func (p *PodSchedMetricsCollector) Collect(ch chan<- prometheus.Metric) {
	podMetrics := p.collector.GetPodMetrics()
	for _, pm := range podMetrics {
		labels := []string{pm.PodName, pm.PodUID, pm.Namespace, pm.NodeName}
		p.emitGauge(ch, p.voluntaryCtxSwitches, pm.VoluntaryCtxSwitches, labels)
		p.emitGauge(ch, p.involuntaryCtxSwitches, pm.InvoluntaryCtxSwitches, labels)
		p.emitGauge(ch, p.cpuTimeNs, pm.CpuTimeNs, labels)
		p.emitGauge(ch, p.waitTimeNs, pm.WaitTimeNs, labels)
		p.emitGauge(ch, p.runCount, pm.RunCount, labels)
		p.emitGauge(ch, p.cpuMigrations, uint64(pm.CpuMigrations), labels)
		p.emitGauge(ch, p.processCount, uint64(pm.ProcessCount), labels)
	}
}

func (p *PodSchedMetricsCollector) emitGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, val uint64, labels []string) {
	m, err := prometheus.NewConstMetric(desc, prometheus.GaugeValue, float64(val), labels...)
	if err == nil {
		ch <- m
	}
}

// MetricNames returns all metric FQ names for documentation / adapter config.
func MetricNames() []string {
	return []string{
		prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "voluntary_ctx_switches_total"),
		prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "involuntary_ctx_switches_total"),
		prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "cpu_time_nanoseconds_total"),
		prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "wait_time_nanoseconds_total"),
		prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "run_count_total"),
		prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "cpu_migrations_total"),
		prometheus.BuildFQName(metricsNamespace, metricsSubsystem, "process_count"),
	}
}

// PodMetricsSnapshot is a serialisable snapshot suitable for REST responses.
type PodMetricsSnapshot struct {
	Pods      []*domain.PodSchedMetrics `json:"pods"`
	Timestamp string                    `json:"timestamp"`
}
