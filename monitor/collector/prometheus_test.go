// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"testing"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/prometheus/client_golang/prometheus"
)

func TestMetricNames(t *testing.T) {
	names := MetricNames()
	if len(names) != 7 {
		t.Errorf("expected 7 metric names, got %d", len(names))
	}

	want := map[string]bool{
		"gthulhu_pod_voluntary_ctx_switches_total":   true,
		"gthulhu_pod_involuntary_ctx_switches_total": true,
		"gthulhu_pod_cpu_time_nanoseconds_total":     true,
		"gthulhu_pod_wait_time_nanoseconds_total":    true,
		"gthulhu_pod_run_count_total":                true,
		"gthulhu_pod_cpu_migrations_total":           true,
		"gthulhu_pod_process_count":                  true,
	}
	for _, n := range names {
		if !want[n] {
			t.Errorf("unexpected metric name: %q", n)
		}
		delete(want, n)
	}
	if len(want) > 0 {
		t.Errorf("missing metric names: %v", want)
	}
}

func TestPodSchedMetricsCollector_Describe(t *testing.T) {
	col := &Collector{
		podMetrics: make(map[string]*domain.PodSchedMetrics),
	}
	pc := NewPodSchedMetricsCollector(col)

	ch := make(chan *prometheus.Desc, 20)
	pc.Describe(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count != 7 {
		t.Errorf("Describe emitted %d descriptors, want 7", count)
	}
}

func TestPodSchedMetricsCollector_Collect_SinglePod(t *testing.T) {
	col := &Collector{
		podMetrics: map[string]*domain.PodSchedMetrics{
			"uid-1": {
				PodName:                "test-pod",
				PodUID:                 "uid-1",
				Namespace:              "default",
				NodeName:               "node-1",
				VoluntaryCtxSwitches:   100,
				InvoluntaryCtxSwitches: 50,
				CpuTimeNs:             5000000000,
				WaitTimeNs:            1000000000,
				RunCount:              200,
				CpuMigrations:         5,
				ProcessCount:          3,
			},
		},
	}
	pc := NewPodSchedMetricsCollector(col)

	ch := make(chan prometheus.Metric, 100)
	pc.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	// 7 metrics per pod × 1 pod = 7
	if count != 7 {
		t.Errorf("Collect emitted %d metrics, want 7", count)
	}
}

func TestPodSchedMetricsCollector_Collect_Empty(t *testing.T) {
	col := &Collector{
		podMetrics: make(map[string]*domain.PodSchedMetrics),
	}
	pc := NewPodSchedMetricsCollector(col)

	ch := make(chan prometheus.Metric, 100)
	pc.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	if count != 0 {
		t.Errorf("Collect emitted %d metrics for empty pods, want 0", count)
	}
}

func TestPodSchedMetricsCollector_Collect_MultiplePods(t *testing.T) {
	col := &Collector{
		podMetrics: map[string]*domain.PodSchedMetrics{
			"uid-1": {PodName: "pod-1", PodUID: "uid-1", Namespace: "ns1", NodeName: "n1", ProcessCount: 2},
			"uid-2": {PodName: "pod-2", PodUID: "uid-2", Namespace: "ns2", NodeName: "n1", ProcessCount: 1},
			"uid-3": {PodName: "pod-3", PodUID: "uid-3", Namespace: "ns1", NodeName: "n2", ProcessCount: 5},
		},
	}
	pc := NewPodSchedMetricsCollector(col)

	ch := make(chan prometheus.Metric, 100)
	pc.Collect(ch)
	close(ch)

	count := 0
	for range ch {
		count++
	}
	// 7 metrics × 3 pods = 21
	if count != 21 {
		t.Errorf("Collect emitted %d metrics for 3 pods, want 21", count)
	}
}
