package v1alpha1

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGTP5GModuleValidation(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GTP5GModule Validation Suite")
}

var _ = Describe("GTP5GModule Validation", func() {
	Context("Version field validation", func() {
		It("should accept valid semantic version", func() {
			module := &GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-module",
				},
				Spec: GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}
			_, err := module.ValidateCreate()
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject empty version", func() {
			module := &GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-module",
				},
				Spec: GTP5GModuleSpec{
					Version: "",
				},
			}
			_, err := module.ValidateCreate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("version is required"))
		})

		It("should reject invalid version format", func() {
			module := &GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-module",
				},
				Spec: GTP5GModuleSpec{
					Version: "invalid",
				},
			}
			_, err := module.ValidateCreate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("must match pattern"))
		})

		It("should reject version without 'v' prefix", func() {
			module := &GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-module",
				},
				Spec: GTP5GModuleSpec{
					Version: "0.8.3",
				},
			}
			_, err := module.ValidateCreate()
			Expect(err).To(HaveOccurred())
		})
	})

	Context("Default values", func() {
		It("should set default node selector", func() {
			module := &GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-module",
				},
				Spec: GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}
			module.Default()
			Expect(module.Spec.NodeSelector).NotTo(BeNil())
			Expect(module.Spec.NodeSelector).To(HaveKeyWithValue("gtp5g.gthulhu.io/enabled", "true"))
		})

		It("should set default installer image", func() {
			module := &GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-module",
				},
				Spec: GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}
			module.Default()
			Expect(module.Spec.Image).NotTo(BeEmpty())
			Expect(module.Spec.Image).To(Equal("localhost:5000/gtp5g-installer:latest"))
		})

		It("should not override existing node selector", func() {
			customSelector := map[string]string{
				"custom-label": "custom-value",
			}
			module := &GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test-module",
				},
				Spec: GTP5GModuleSpec{
					Version:      "v0.8.3",
					NodeSelector: customSelector,
				},
			}
			module.Default()
			Expect(module.Spec.NodeSelector).To(Equal(customSelector))
		})
	})

	Context("Update validation", func() {
		It("should allow updating kernel version", func() {
			oldModule := &GTP5GModule{
				Spec: GTP5GModuleSpec{
					Version:       "v0.8.3",
					KernelVersion: "5.15.0-56-generic",
				},
			}
			newModule := &GTP5GModule{
				Spec: GTP5GModuleSpec{
					Version:       "v0.8.3",
					KernelVersion: "5.15.0-57-generic",
				},
			}
			_, err := newModule.ValidateUpdate(oldModule)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow updating node selector", func() {
			oldModule := &GTP5GModule{
				Spec: GTP5GModuleSpec{
					Version: "v0.8.3",
					NodeSelector: map[string]string{
						"label1": "value1",
					},
				},
			}
			newModule := &GTP5GModule{
				Spec: GTP5GModuleSpec{
					Version: "v0.8.3",
					NodeSelector: map[string]string{
						"label2": "value2",
					},
				},
			}
			_, err := newModule.ValidateUpdate(oldModule)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should allow updating gtp5g version", func() {
			oldModule := &GTP5GModule{
				Spec: GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}
			newModule := &GTP5GModule{
				Spec: GTP5GModuleSpec{
					Version: "v0.9.0",
				},
			}
			_, err := newModule.ValidateUpdate(oldModule)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Context("Delete validation", func() {
		It("should allow deletion", func() {
			module := &GTP5GModule{
				Spec: GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}
			_, err := module.ValidateDelete()
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
