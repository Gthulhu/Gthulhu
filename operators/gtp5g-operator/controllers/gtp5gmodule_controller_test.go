package controllers

import (
	"testing"
)

// NOTE: Ginkgo/Gomega integration tests are disabled due to envtest requirements.
// These tests require kubebuilder's envtest setup with etcd and kube-apiserver.
//
// To enable integration tests:
// 1. Install kubebuilder: https://book.kubebuilder.io/quick-start.html
// 2. Install setup-envtest: go install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest
// 3. Uncomment suite_test.go setup
// 4. Run: make test
//
// For now, we rely on pure unit tests below.

// var _ = Describe("GTP5GModule Controller", func() {
// 	// Integration tests commented out - require envtest
// })

func TestContainsString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		target   string
		expected bool
	}{
		{"Found", []string{"a", "b", "c"}, "b", true},
		{"Not found", []string{"a", "b", "c"}, "d", false},
		{"Empty slice", []string{}, "a", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsString(tt.slice, tt.target)
			if result != tt.expected {
				t.Errorf("containsString(%v, %s) = %v; want %v", tt.slice, tt.target, result, tt.expected)
			}
		})
	}
}

func TestRemoveString(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		target   string
		expected []string
	}{
		{"Remove middle", []string{"a", "b", "c"}, "b", []string{"a", "c"}},
		{"Remove first", []string{"a", "b", "c"}, "a", []string{"b", "c"}},
		{"Remove last", []string{"a", "b", "c"}, "c", []string{"a", "b"}},
		{"Not found", []string{"a", "b", "c"}, "d", []string{"a", "b", "c"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeString(tt.slice, tt.target)
			if len(result) != len(tt.expected) {
				t.Errorf("removeString(%v, %s) length = %d; want %d", tt.slice, tt.target, len(result), len(tt.expected))
				return
			}
			for i := range result {
				if result[i] != tt.expected[i] {
					t.Errorf("removeString(%v, %s) = %v; want %v", tt.slice, tt.target, result, tt.expected)
					return
				}
			}
		})
	}
}
