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
