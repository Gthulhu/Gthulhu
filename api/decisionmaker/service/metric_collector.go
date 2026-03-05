package service

import (
	"sync/atomic"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	_ prometheus.Collector = (*MetricCollector)(nil)
)

// MetricCollector
type MetricCollector struct {
	machineID string

	UserSchedLastRunAtMetric *prometheus.Desc
	NrQueuedMetric           *prometheus.Desc
	NrScheduledMetric        *prometheus.Desc
	NrRunningMetric          *prometheus.Desc
	NrOnlineCPUsMetric       *prometheus.Desc
	NrUserDispatchesMetric   *prometheus.Desc
	NrKernelDispatchesMetric *prometheus.Desc
	NrCancelDispatchesMetric *prometheus.Desc
	NrBounceDispatchesMetric *prometheus.Desc
	NrFailedDispatchesMetric *prometheus.Desc
	NrSchedCongestedMetric   *prometheus.Desc

	metricSet atomic.Pointer[domain.MetricSet]
}

// // NewMetricCollector creates a new MetricCollector for a specific game server.
func NewMetricCollector(machineID string) *MetricCollector {
	constantLabels := prometheus.Labels{"machine_id": machineID}
	return &MetricCollector{
		machineID: machineID,
		UserSchedLastRunAtMetric: prometheus.NewDesc(
			"user_sched_last_run_at",
			"the timestamp of the last user scheduling run",
			nil,
			constantLabels,
		),
		NrQueuedMetric: prometheus.NewDesc(
			"nr_queued",
			"number of tasks queued in the userspace scheduler",
			nil,
			constantLabels,
		),
		NrScheduledMetric: prometheus.NewDesc(
			"nr_scheduled",
			"number of tasks scheduled by the userspace scheduler",
			nil,
			constantLabels,
		),
		NrRunningMetric: prometheus.NewDesc(
			"nr_running",
			"number of tasks currently running in the userspace scheduler",
			nil,
			constantLabels,
		),
		NrOnlineCPUsMetric: prometheus.NewDesc(
			"nr_online_cpus",
			"number of online CPUs in the system",
			nil,
			constantLabels,
		),
		NrUserDispatchesMetric: prometheus.NewDesc(
			"nr_user_dispatches",
			"number of user-space dispatches",
			nil,
			constantLabels,
		),
		NrKernelDispatchesMetric: prometheus.NewDesc(
			"nr_kernel_dispatches",
			"number of kernel-space dispatches",
			nil,
			constantLabels,
		),
		NrCancelDispatchesMetric: prometheus.NewDesc(
			"nr_cancel_dispatches",
			"number of canceled dispatches",
			nil,
			constantLabels,
		),
		NrBounceDispatchesMetric: prometheus.NewDesc(
			"nr_bounce_dispatches",
			"number of bounced dispatches",
			nil,
			constantLabels,
		),
		NrFailedDispatchesMetric: prometheus.NewDesc(
			"nr_failed_dispatches",
			"number of failed dispatches",
			nil,
			constantLabels,
		),
		NrSchedCongestedMetric: prometheus.NewDesc(
			"nr_sched_congested",
			"number of times the scheduler was congested",
			nil,
			constantLabels,
		),
	}
}

// Describe sends the super-set of all possible descriptors of metrics
func (collector *MetricCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.UserSchedLastRunAtMetric
	ch <- collector.NrQueuedMetric
	ch <- collector.NrScheduledMetric
	ch <- collector.NrRunningMetric
	ch <- collector.NrOnlineCPUsMetric
	ch <- collector.NrUserDispatchesMetric
	ch <- collector.NrKernelDispatchesMetric
	ch <- collector.NrCancelDispatchesMetric
	ch <- collector.NrBounceDispatchesMetric
	ch <- collector.NrFailedDispatchesMetric
	ch <- collector.NrSchedCongestedMetric
}

// Collect is called by the Prometheus registry when collecting metrics.
func (collector *MetricCollector) Collect(ch chan<- prometheus.Metric) {
	metricSet := collector.metricSet.Load()
	if metricSet == nil {
		return
	}

	ch <- prometheus.MustNewConstMetric(collector.UserSchedLastRunAtMetric, prometheus.GaugeValue, float64(metricSet.UserSchedLastRunAt))
	ch <- prometheus.MustNewConstMetric(collector.NrQueuedMetric, prometheus.GaugeValue, float64(metricSet.NrQueued))
	ch <- prometheus.MustNewConstMetric(collector.NrScheduledMetric, prometheus.GaugeValue, float64(metricSet.NrScheduled))
	ch <- prometheus.MustNewConstMetric(collector.NrRunningMetric, prometheus.GaugeValue, float64(metricSet.NrRunning))
	ch <- prometheus.MustNewConstMetric(collector.NrOnlineCPUsMetric, prometheus.GaugeValue, float64(metricSet.NrOnlineCPUs))
	ch <- prometheus.MustNewConstMetric(collector.NrUserDispatchesMetric, prometheus.GaugeValue, float64(metricSet.NrUserDispatches))
	ch <- prometheus.MustNewConstMetric(collector.NrKernelDispatchesMetric, prometheus.GaugeValue, float64(metricSet.NrKernelDispatches))
	ch <- prometheus.MustNewConstMetric(collector.NrCancelDispatchesMetric, prometheus.GaugeValue, float64(metricSet.NrCancelDispatches))
	ch <- prometheus.MustNewConstMetric(collector.NrBounceDispatchesMetric, prometheus.GaugeValue, float64(metricSet.NrBounceDispatches))
	ch <- prometheus.MustNewConstMetric(collector.NrFailedDispatchesMetric, prometheus.GaugeValue, float64(metricSet.NrFailedDispatches))
	ch <- prometheus.MustNewConstMetric(collector.NrSchedCongestedMetric, prometheus.GaugeValue, float64(metricSet.NrSchedCongested))
}

func (collector *MetricCollector) UpdateMetrics(newMetricSet *domain.MetricSet) {
	collector.metricSet.Store(newMetricSet)
}
