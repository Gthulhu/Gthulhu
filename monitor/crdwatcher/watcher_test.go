// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package crdwatcher

import (
	"testing"

	"github.com/Gthulhu/Gthulhu/monitor/collector"
	"github.com/Gthulhu/api/decisionmaker/domain"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ───────────────── MatchesCommandRegex ─────────────────

func TestMatchesCommandRegex(t *testing.T) {
	tests := []struct {
		name         string
		pattern, cmd string
		want         bool
	}{
		{"empty pattern matches all", "", "anything", true},
		{"wildcard match", ".*nginx.*", "nginx-proxy", true},
		{"exact match", "^envoy$", "envoy", true},
		{"exact no match", "^envoy$", "envoy-proxy", false},
		{"invalid regex returns false", "[invalid", "anything", false},
		{"dot star", ".*", "", true},
		{"partial match", "kube", "kube-proxy", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MatchesCommandRegex(tc.pattern, tc.cmd)
			if got != tc.want {
				t.Errorf("MatchesCommandRegex(%q, %q) = %v, want %v",
					tc.pattern, tc.cmd, got, tc.want)
			}
		})
	}
}

// ───────────────── parsePSM ─────────────────

func TestParsePSM_Basic(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gthulhu.io/v1alpha1",
			"kind":       "PodSchedulingMetrics",
			"metadata": map[string]interface{}{
				"name":      "test-psm",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"enabled":                   true,
				"k8sNamespaces":             []interface{}{"default", "kube-system"},
				"commandRegex":              ".*nginx.*",
				"collectionIntervalSeconds": int64(15),
				"labelSelectors": []interface{}{
					map[string]interface{}{"key": "app", "value": "nginx"},
				},
				"metrics": map[string]interface{}{
					"voluntaryCtxSwitches":   true,
					"involuntaryCtxSwitches": true,
					"cpuTimeNs":              true,
					"waitTimeNs":             false,
				},
			},
		},
	}

	psm, err := parsePSM(obj)
	if err != nil {
		t.Fatalf("parsePSM failed: %v", err)
	}

	if psm.Name != "test-psm" {
		t.Errorf("Name = %q, want %q", psm.Name, "test-psm")
	}
	if psm.Namespace != "default" {
		t.Errorf("Namespace = %q, want %q", psm.Namespace, "default")
	}
	if !psm.Spec.Enabled {
		t.Error("Spec.Enabled should be true")
	}
	if len(psm.Spec.K8sNamespaces) != 2 {
		t.Errorf("K8sNamespaces len = %d, want 2", len(psm.Spec.K8sNamespaces))
	}
	if psm.Spec.CommandRegex != ".*nginx.*" {
		t.Errorf("CommandRegex = %q, want %q", psm.Spec.CommandRegex, ".*nginx.*")
	}
	if psm.Spec.CollectionIntervalSeconds != 15 {
		t.Errorf("CollectionIntervalSeconds = %d, want 15", psm.Spec.CollectionIntervalSeconds)
	}
	if len(psm.Spec.LabelSelectors) != 1 {
		t.Errorf("LabelSelectors len = %d, want 1", len(psm.Spec.LabelSelectors))
	}
	if !psm.Spec.Metrics.VoluntaryCtxSwitches {
		t.Error("Metrics.VoluntaryCtxSwitches should be true")
	}
}

func TestParsePSM_MissingSpec(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gthulhu.io/v1alpha1",
			"kind":       "PodSchedulingMetrics",
			"metadata": map[string]interface{}{
				"name":      "empty",
				"namespace": "default",
			},
		},
	}

	_, err := parsePSM(obj)
	if err == nil {
		t.Fatal("expected error for missing spec")
	}
}

func TestParsePSM_WithScaling(t *testing.T) {
	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "gthulhu.io/v1alpha1",
			"kind":       "PodSchedulingMetrics",
			"metadata": map[string]interface{}{
				"name":      "scaling-psm",
				"namespace": "production",
			},
			"spec": map[string]interface{}{
				"enabled": true,
				"scaling": map[string]interface{}{
					"enabled":         true,
					"metricName":      "gthulhu_pod_cpu_time_nanoseconds_total",
					"targetValue":     "1000000000",
					"minReplicaCount": int64(1),
					"maxReplicaCount": int64(10),
					"cooldownPeriod":  int64(60),
					"scaleTargetRef": map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"name":       "my-app",
					},
				},
			},
		},
	}

	psm, err := parsePSM(obj)
	if err != nil {
		t.Fatalf("parsePSM failed: %v", err)
	}
	if psm.Spec.Scaling == nil {
		t.Fatal("Scaling should not be nil")
	}
	if !psm.Spec.Scaling.Enabled {
		t.Error("Scaling.Enabled should be true")
	}
	if psm.Spec.Scaling.MaxReplicaCount != 10 {
		t.Errorf("MaxReplicaCount = %d, want 10", psm.Spec.Scaling.MaxReplicaCount)
	}
	if psm.Spec.Scaling.CooldownPeriod != 60 {
		t.Errorf("CooldownPeriod = %d, want 60", psm.Spec.Scaling.CooldownPeriod)
	}
	if psm.Spec.Scaling.ScaleTargetRef == nil {
		t.Fatal("ScaleTargetRef should not be nil")
	}
	if psm.Spec.Scaling.ScaleTargetRef.Name != "my-app" {
		t.Errorf("ScaleTargetRef.Name = %q, want %q",
			psm.Spec.Scaling.ScaleTargetRef.Name, "my-app")
	}
}

// ───────────────── psmMatchesPod ─────────────────

func TestPsmMatchesPod_NamespaceFilter(t *testing.T) {
	w := &Watcher{}

	psm := &domain.PodSchedulingMetrics{
		Spec: domain.PodSchedulingMetricsSpec{
			Enabled:       true,
			K8sNamespaces: []string{"production", "staging"},
		},
	}

	// Pod in matching namespace
	ref := &collector.PodRef{Namespace: "production"}
	if !w.psmMatchesPod(psm, ref) {
		t.Error("expected match for pod in 'production' namespace")
	}

	// Pod in non-matching namespace
	ref2 := &collector.PodRef{Namespace: "default"}
	if w.psmMatchesPod(psm, ref2) {
		t.Error("expected no match for pod in 'default' namespace")
	}
}

func TestPsmMatchesPod_NoNamespaceFilter(t *testing.T) {
	w := &Watcher{}

	psm := &domain.PodSchedulingMetrics{
		Spec: domain.PodSchedulingMetricsSpec{
			Enabled:       true,
			K8sNamespaces: nil, // no filter → match all
		},
	}

	ref := &collector.PodRef{Namespace: "any-namespace"}
	if !w.psmMatchesPod(psm, ref) {
		t.Error("expected match when no namespace filter is set")
	}
}

// ───────────────── GetActiveSpecs ─────────────────

func TestWatcher_GetActiveSpecs_Empty(t *testing.T) {
	w := &Watcher{
		specs: make(map[string]*domain.PodSchedulingMetrics),
	}
	specs := w.GetActiveSpecs()
	if len(specs) != 0 {
		t.Errorf("expected 0 specs, got %d", len(specs))
	}
}

func TestWatcher_GetActiveSpecs(t *testing.T) {
	w := &Watcher{
		specs: map[string]*domain.PodSchedulingMetrics{
			"ns/a": {Name: "a", Namespace: "ns"},
			"ns/b": {Name: "b", Namespace: "ns"},
		},
	}
	specs := w.GetActiveSpecs()
	if len(specs) != 2 {
		t.Errorf("expected 2 specs, got %d", len(specs))
	}
}
