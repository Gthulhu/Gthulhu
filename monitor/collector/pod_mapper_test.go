// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package collector

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// ───────────────── extractPodUID ─────────────────

func TestExtractPodUID_CgroupV2(t *testing.T) {
	tests := []struct {
		name, line, want string
	}{
		{
			name: "standard burstable pod",
			line: "0::/kubepods/burstable/podabc-def-123/container-id-456",
			want: "abc-def-123",
		},
		{
			name: "besteffort pod without container suffix",
			line: "0::/kubepods/besteffort/pod12345",
			want: "12345",
		},
		{
			name: "guaranteed pod",
			line: "0::/kubepods/podaaa-bbb-ccc/ctr",
			want: "aaa-bbb-ccc",
		},
		{
			name: "no pod UID in path",
			line: "0::/system.slice/docker.service",
			want: "",
		},
		{
			name: "empty line",
			line: "",
			want: "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPodUID(tc.line)
			if got != tc.want {
				t.Errorf("extractPodUID(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

func TestExtractPodUID_CgroupV1(t *testing.T) {
	tests := []struct {
		name, line, want string
	}{
		{
			name: "systemd slice with underscores",
			line: "12:memory:/kubepods.slice/kubepods-burstable.slice/kubepods-burstable-podabc_def_123.slice/docker-container.scope",
			want: "abc-def-123",
		},
		{
			name: "short v1 slice",
			line: "3:cpu:/kubepods-burstable-pod11_22_33.slice/",
			want: "11-22-33",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractPodUID(tc.line)
			if got != tc.want {
				t.Errorf("extractPodUID(%q) = %q, want %q", tc.line, got, tc.want)
			}
		})
	}
}

// ───────────────── normalizePodUID ─────────────────

func TestNormalizePodUID(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"abc-def", "abc-def"},
		{"abc_def", "abc-def"},
		{"a_b_c", "a-b-c"},
		{"", ""},
		{"no_underscores_at_all", "no-underscores-at-all"},
	}
	for _, tc := range tests {
		got := normalizePodUID(tc.raw)
		if got != tc.want {
			t.Errorf("normalizePodUID(%q) = %q, want %q", tc.raw, got, tc.want)
		}
	}
}

// ───────────────── PodMapper integration ─────────────────

func TestPodMapper_GetPodForPID(t *testing.T) {
	tmp := t.TempDir()

	// PID 100 — belongs to a known pod
	os.MkdirAll(filepath.Join(tmp, "100"), 0o755)
	os.WriteFile(filepath.Join(tmp, "100", "cgroup"),
		[]byte("0::/kubepods/burstable/podtest-uid-1/containerXYZ\n"), 0o644)

	// PID 200 — system process, no pod
	os.MkdirAll(filepath.Join(tmp, "200"), 0o755)
	os.WriteFile(filepath.Join(tmp, "200", "cgroup"),
		[]byte("0::/system.slice/sshd.service\n"), 0o644)

	m := NewPodMapper("test-node", nil)
	m.procRoot = tmp
	m.SetPodIndex(map[string]*PodRef{
		"test-uid-1": {PodName: "my-pod", PodUID: "test-uid-1", Namespace: "default", NodeName: "test-node"},
	})

	ref := m.GetPodForPID(100)
	if ref == nil {
		t.Fatal("expected pod ref for pid 100, got nil")
	}
	if ref.PodName != "my-pod" {
		t.Errorf("PodName = %q, want %q", ref.PodName, "my-pod")
	}
	if ref.Namespace != "default" {
		t.Errorf("Namespace = %q, want %q", ref.Namespace, "default")
	}

	// Second call should hit cache
	ref2 := m.GetPodForPID(100)
	if ref2 == nil || ref2.PodName != "my-pod" {
		t.Error("cache miss on second call for same PID")
	}

	// PID 200 should not resolve
	if m.GetPodForPID(200) != nil {
		t.Error("expected nil for system PID 200")
	}
}

func TestPodMapper_ScanAllPIDs(t *testing.T) {
	tmp := t.TempDir()

	for _, pid := range []string{"10", "20", "30"} {
		os.MkdirAll(filepath.Join(tmp, pid), 0o755)
	}
	os.WriteFile(filepath.Join(tmp, "10", "cgroup"),
		[]byte("0::/kubepods/burstable/poduid-a/ctr1\n"), 0o644)
	os.WriteFile(filepath.Join(tmp, "20", "cgroup"),
		[]byte("0::/kubepods/besteffort/poduid-b/ctr2\n"), 0o644)
	os.WriteFile(filepath.Join(tmp, "30", "cgroup"),
		[]byte("0::/system.slice/kernel\n"), 0o644)

	m := NewPodMapper("node1", nil)
	m.procRoot = tmp
	m.SetPodIndex(map[string]*PodRef{
		"uid-a": {PodName: "pod-a", PodUID: "uid-a", Namespace: "ns-a", NodeName: "node1"},
		"uid-b": {PodName: "pod-b", PodUID: "uid-b", Namespace: "ns-b", NodeName: "node1"},
	})

	m.ScanAllPIDs()

	pids := m.ListMappedPIDs()
	sort.Slice(pids, func(i, j int) bool { return pids[i] < pids[j] })

	if len(pids) != 2 {
		t.Fatalf("expected 2 mapped PIDs, got %d: %v", len(pids), pids)
	}
	if pids[0] != 10 || pids[1] != 20 {
		t.Errorf("mapped PIDs = %v, want [10, 20]", pids)
	}
}

func TestPodMapper_GetAllPodRefs(t *testing.T) {
	m := NewPodMapper("n1", nil)
	m.SetPodIndex(map[string]*PodRef{
		"uid-1": {PodName: "p1"},
		"uid-2": {PodName: "p2"},
	})

	refs := m.GetAllPodRefs()
	if len(refs) != 2 {
		t.Errorf("expected 2 pod refs, got %d", len(refs))
	}
}

func TestPodMapper_SetPodIndex_InvalidatesCache(t *testing.T) {
	tmp := t.TempDir()
	os.MkdirAll(filepath.Join(tmp, "1"), 0o755)
	os.WriteFile(filepath.Join(tmp, "1", "cgroup"),
		[]byte("0::/kubepods/poduid-x/ctr\n"), 0o644)

	m := NewPodMapper("n", nil)
	m.procRoot = tmp
	m.SetPodIndex(map[string]*PodRef{
		"uid-x": {PodName: "old-name", PodUID: "uid-x"},
	})

	// Populate cache
	ref := m.GetPodForPID(1)
	if ref == nil || ref.PodName != "old-name" {
		t.Fatal("initial resolve failed")
	}

	// Replace pod index → cache should be cleared
	m.SetPodIndex(map[string]*PodRef{
		"uid-x": {PodName: "new-name", PodUID: "uid-x"},
	})

	ref2 := m.GetPodForPID(1)
	if ref2 == nil {
		t.Fatal("expected resolve after SetPodIndex")
	}
	if ref2.PodName != "new-name" {
		t.Errorf("PodName = %q, want %q", ref2.PodName, "new-name")
	}
}

func TestPodMapper_String(t *testing.T) {
	m := NewPodMapper("n1", nil)
	m.SetPodIndex(map[string]*PodRef{"a": {}, "b": {}})
	s := m.String()
	if s == "" {
		t.Error("String() returned empty")
	}
}
