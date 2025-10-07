package controllers

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorv1alpha1 "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/api/v1alpha1"
)

var _ = Describe("GTP5GModule Controller", func() {
	const (
		timeout  = time.Second * 10
		interval = time.Millisecond * 250
	)

	Context("When reconciling a GTP5GModule", func() {
		It("Should create a DaemonSet", func() {
			ctx := context.Background()

			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-module",
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}

			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			moduleKey := types.NamespacedName{Name: "test-module"}
			createdModule := &operatorv1alpha1.GTP5GModule{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, moduleKey, createdModule)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			By("Checking the DaemonSet is created")
			dsKey := types.NamespacedName{
				Name:      "gtp5g-installer-test-module",
				Namespace: "default",
			}
			ds := &appsv1.DaemonSet{}

			Eventually(func() bool {
				err := k8sClient.Get(ctx, dsKey, ds)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(ds.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers[0].Name).Should(Equal("installer"))
		})
	})
})

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
