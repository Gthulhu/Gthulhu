package rest

import (
	"testing"
	"time"
)

func TestComputeFeaturesZeroGuard(t *testing.T) {
	f := computeFeatures(metricsPayload{
		VolCtxSW:   10,
		InvolCtxSW: 2,
		CPUTime:    0,
		WaitTime:   0,
		RunCount:   0,
		SMTMigr:    1,
		L3Migr:     0,
		NUMAMigr:   0,
	})

	if f[0] != 10 {
		t.Fatalf("vol_ctx_ratio mismatch, got=%v", f[0])
	}
	if f[1] != 2 {
		t.Fatalf("invol_ctx_ratio mismatch, got=%v", f[1])
	}
	if f[3] != 0 {
		t.Fatalf("wait_ratio should be 0, got=%v", f[3])
	}
	if f[5] != 0 {
		t.Fatalf("numa_migr_ratio should be 0, got=%v", f[5])
	}
}

func TestAdaptiveClassifierPhaseProgression(t *testing.T) {
	c := NewAdaptiveClassifier(3)
	now := time.Now().Unix()
	input := classificationInput{
		Namespace: "default",
		Pod:       "pod-a",
		Node:      "node-1",
		Metrics: metricsPayload{
			VolCtxSW:   100,
			InvolCtxSW: 10,
			CPUTime:    10000,
			WaitTime:   500,
			RunCount:   100,
			SMTMigr:    1,
			L3Migr:     2,
			NUMAMigr:   1,
		},
	}

	for i := 0; i < 9; i++ {
		input.Timestamp = now + int64(i)
		item := c.Ingest(input)
		if item.Phase != PodPhaseColdStart {
			t.Fatalf("expected cold_start at i=%d, got=%s", i, item.Phase)
		}
	}

	for i := 9; i < 29; i++ {
		input.Timestamp = now + int64(i)
		item := c.Ingest(input)
		if item.Phase != PodPhaseWarmingUp {
			t.Fatalf("expected warming_up at i=%d, got=%s", i, item.Phase)
		}
	}

	item := c.Ingest(input)
	if item.Phase != PodPhaseStable {
		t.Fatalf("expected stable after 30 samples, got=%s", item.Phase)
	}
}

func TestAdaptiveClassifierListFilters(t *testing.T) {
	c := NewAdaptiveClassifier(2)
	for i := 0; i < 12; i++ {
		c.Ingest(classificationInput{
			Timestamp: time.Now().Unix() + int64(i),
			Namespace: "ns-a",
			Pod:       "pod-a",
			Metrics: metricsPayload{
				VolCtxSW:   20,
				InvolCtxSW: 1,
				CPUTime:    1000,
				WaitTime:   50,
				RunCount:   20,
			},
		})
		c.Ingest(classificationInput{
			Timestamp: time.Now().Unix() + int64(i),
			Namespace: "ns-b",
			Pod:       "pod-b",
			Metrics: metricsPayload{
				VolCtxSW:   2,
				InvolCtxSW: 20,
				CPUTime:    5000,
				WaitTime:   100,
				RunCount:   20,
			},
		})
	}

	items := c.List("ns-a", "", "")
	if len(items) != 1 {
		t.Fatalf("expected 1 item for namespace filter, got=%d", len(items))
	}
	items = c.List("", PodPhaseWarmingUp, "")
	if len(items) == 0 {
		t.Fatal("expected at least one warming_up item")
	}
}

func TestAdaptiveClassifierTagsHighCPUPerRunAsCPUHeavy(t *testing.T) {
	c := NewAdaptiveClassifier(3)
	now := time.Now().Unix()
	input := classificationInput{
		Namespace: "default",
		Pod:       "test-pod1",
		Node:      "myvm",
		Metrics: metricsPayload{
			VolCtxSW:   19,
			InvolCtxSW: 9627,
			CPUTime:    183346997199,
			WaitTime:   1490167876,
			RunCount:   9647,
		},
	}

	var item *classifyResponseItem
	for i := 0; i < stableMinSamples; i++ {
		input.Timestamp = now + int64(i)
		item = c.Ingest(input)
	}

	if item.Phase != PodPhaseStable {
		t.Fatalf("expected stable after stable samples, got=%s", item.Phase)
	}
	if !containsTag(item.Classification.CurrentType, "cpu_heavy") {
		t.Fatalf("expected cpu_heavy type, got=%v", item.Classification.CurrentType)
	}
	if containsTag(item.Classification.CurrentType, "balanced") {
		t.Fatalf("expected cpu_heavy to replace balanced, got=%v", item.Classification.CurrentType)
	}
	if item.Recommendation.Action != "increase_cpu_limit" {
		t.Fatalf("expected increase_cpu_limit recommendation, got=%s", item.Recommendation.Action)
	}
}

func TestInferSemanticsAssignsDocumentedTypeTags(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		cluster int
		centers []featureVector
	}{
		{
			name:    "balanced",
			tag:     "balanced",
			cluster: 0,
			centers: []featureVector{
				{0, 0, 0, 0, 0, 0},
				{1, 1, 1, 1, 1, 1},
			},
		},
		{
			name:    "cpu_heavy",
			tag:     "cpu_heavy",
			cluster: 0,
			centers: []featureVector{
				{0, 0, cpuHeavyPerRunNS * 2, 0, 0, 0},
				{1, 1, 1, 1, 1, 1},
			},
		},
		{
			name:    "needs_higher_priority",
			tag:     "needs_higher_priority",
			cluster: 0,
			centers: []featureVector{
				{0, 2, 0, 0, 0, 0},
				{1, 1, 1, 1, 1, 1},
			},
		},
		{
			name:    "interactive",
			tag:     "interactive",
			cluster: 0,
			centers: []featureVector{
				{2, 0, 0, 0, 0, 0},
				{1, 1, 1, 1, 1, 1},
			},
		},
		{
			name:    "cache_unfriendly",
			tag:     "cache_unfriendly",
			cluster: 0,
			centers: []featureVector{
				{0, 0, 0, 0, 2, 0},
				{1, 1, 1, 1, 1, 1},
			},
		},
		{
			name:    "numa_unfriendly",
			tag:     "numa_unfriendly",
			cluster: 0,
			centers: []featureVector{
				{0, 0, 0, 0, 0, 2},
				{1, 1, 1, 1, 1, 1},
			},
		},
		{
			name:    "scheduling_latency",
			tag:     "scheduling_latency",
			cluster: 0,
			centers: []featureVector{
				{0, 0, 0, 2, 0, 0},
				{1, 1, 1, 1, 1, 1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			semantics := inferSemantics(tt.centers)
			if !containsTag(semantics[tt.cluster], tt.tag) {
				t.Fatalf("expected cluster %d to include %s, got=%v", tt.cluster, tt.tag, semantics[tt.cluster])
			}
		})
	}
}

func TestBuildClassifyItemReturnsDocumentedTypeTags(t *testing.T) {
	for _, tag := range []string{
		"cpu_heavy",
		"balanced",
		"needs_higher_priority",
		"interactive",
		"cache_unfriendly",
		"numa_unfriendly",
		"scheduling_latency",
	} {
		t.Run(tag, func(t *testing.T) {
			item := buildClassifyItem(&podState{
				namespace:     "default",
				pod:           "pod-a",
				phase:         PodPhaseStable,
				currentTypes:  []string{tag},
				lastTimestamp: time.Now().Unix(),
			})

			if !containsTag(item.Classification.CurrentType, tag) {
				t.Fatalf("expected API response to include %s, got=%v", tag, item.Classification.CurrentType)
			}
		})
	}
}
