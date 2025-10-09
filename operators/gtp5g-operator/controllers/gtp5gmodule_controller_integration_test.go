package controllers

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	operatorv1alpha1 "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/api/v1alpha1"
)

var _ = Describe("GTP5GModule Controller Integration Tests", func() {
	const (
		timeout  = time.Second * 30
		interval = time.Millisecond * 250
	)

	Context("When creating a GTP5GModule", func() {
		It("should create a DaemonSet for installer", func() {
			ctx := context.Background()

			moduleName := "test-gtp5g-module"
			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
					NodeSelector: map[string]string{
						"test-label": "test-value",
					},
				},
			}

			By("Creating the GTP5GModule resource")
			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			By("Checking if the DaemonSet was created")
			dsName := "gtp5g-installer-" + moduleName
			ds := &appsv1.DaemonSet{}

			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      dsName,
					Namespace: "default",
				}, ds)
			}, timeout, interval).Should(Succeed())

			By("Verifying DaemonSet properties")
			Expect(ds.Spec.Template.Spec.Containers).To(HaveLen(1))
			Expect(ds.Spec.Template.Spec.Containers[0].Name).To(Equal("installer"))
			Expect(ds.Spec.Template.Spec.HostPID).To(BeTrue())

			By("Verifying node selector is applied")
			Expect(ds.Spec.Template.Spec.NodeSelector).To(HaveKeyWithValue("test-label", "test-value"))

			By("Verifying environment variables")
			envVars := ds.Spec.Template.Spec.Containers[0].Env
			var versionEnv *corev1.EnvVar
			for _, env := range envVars {
				if env.Name == "GTP5G_VERSION" {
					versionEnv = &env
					break
				}
			}
			Expect(versionEnv).NotTo(BeNil())
			Expect(versionEnv.Value).To(Equal("v0.8.3"))

			By("Verifying OwnerReference is set")
			Expect(ds.OwnerReferences).To(HaveLen(1))
			Expect(ds.OwnerReferences[0].Name).To(Equal(moduleName))
			Expect(ds.OwnerReferences[0].Kind).To(Equal("GTP5GModule"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, module)).Should(Succeed())
		})

		It("should update status to Installing when DaemonSet is created", func() {
			ctx := context.Background()

			moduleName := "test-status-module"
			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}

			By("Creating the GTP5GModule resource")
			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			By("Waiting for status to be updated")
			Eventually(func() operatorv1alpha1.ModulePhase {
				updated := &operatorv1alpha1.GTP5GModule{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: moduleName,
				}, updated)
				if err != nil {
					return ""
				}
				return updated.Status.Phase
			}, timeout, interval).Should(Or(
				Equal(operatorv1alpha1.ModulePhasePending),
				Equal(operatorv1alpha1.ModulePhaseInstalling),
			))

			By("Verifying status message is set")
			updated := &operatorv1alpha1.GTP5GModule{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: moduleName,
			}, updated)).Should(Succeed())
			Expect(updated.Status.Message).NotTo(BeEmpty())

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, module)).Should(Succeed())
		})

		It("should add finalizer to GTP5GModule", func() {
			ctx := context.Background()

			moduleName := "test-finalizer-module"
			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}

			By("Creating the GTP5GModule resource")
			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			By("Checking if finalizer is added")
			Eventually(func() []string {
				updated := &operatorv1alpha1.GTP5GModule{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name: moduleName,
				}, updated)
				if err != nil {
					return nil
				}
				return updated.Finalizers
			}, timeout, interval).Should(ContainElement("operator.gthulhu.io/finalizer"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, module)).Should(Succeed())
		})
	})

	Context("When updating a GTP5GModule", func() {
		It("should update the DaemonSet", func() {
			ctx := context.Background()

			moduleName := "test-update-module"
			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}

			By("Creating the GTP5GModule resource")
			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			By("Waiting for DaemonSet to be created")
			dsName := "gtp5g-installer-" + moduleName
			ds := &appsv1.DaemonSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      dsName,
					Namespace: "default",
				}, ds)
			}, timeout, interval).Should(Succeed())

			By("Updating the GTP5GModule version")
			updated := &operatorv1alpha1.GTP5GModule{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{
				Name: moduleName,
			}, updated)).Should(Succeed())

			updated.Spec.Version = "v0.9.0"
			Expect(k8sClient.Update(ctx, updated)).Should(Succeed())

			By("Verifying DaemonSet is updated")
			Eventually(func() string {
				ds := &appsv1.DaemonSet{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      dsName,
					Namespace: "default",
				}, ds)
				if err != nil {
					return ""
				}
				for _, env := range ds.Spec.Template.Spec.Containers[0].Env {
					if env.Name == "GTP5G_VERSION" {
						return env.Value
					}
				}
				return ""
			}, timeout, interval).Should(Equal("v0.9.0"))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, updated)).Should(Succeed())
		})
	})

	Context("When deleting a GTP5GModule", func() {
		It("should delete the associated DaemonSet", func() {
			ctx := context.Background()

			moduleName := "test-delete-module"
			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}

			By("Creating the GTP5GModule resource")
			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			By("Waiting for DaemonSet to be created")
			dsName := "gtp5g-installer-" + moduleName
			ds := &appsv1.DaemonSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      dsName,
					Namespace: "default",
				}, ds)
			}, timeout, interval).Should(Succeed())

			By("Deleting the GTP5GModule")
			Expect(k8sClient.Delete(ctx, module)).Should(Succeed())

			By("Verifying DaemonSet is deleted via garbage collection")
			Eventually(func() bool {
				ds := &appsv1.DaemonSet{}
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      dsName,
					Namespace: "default",
				}, ds)
				return apierrors.IsNotFound(err)
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("With custom installer image", func() {
		It("should use custom image in DaemonSet", func() {
			ctx := context.Background()

			moduleName := "test-custom-image-module"
			customImage := "custom-registry/gtp5g-installer:v1.0"
			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
					Image:   customImage,
				},
			}

			By("Creating the GTP5GModule resource")
			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			By("Verifying DaemonSet uses custom image")
			dsName := "gtp5g-installer-" + moduleName
			ds := &appsv1.DaemonSet{}
			Eventually(func() string {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      dsName,
					Namespace: "default",
				}, ds)
				if err != nil {
					return ""
				}
				return ds.Spec.Template.Spec.Containers[0].Image
			}, timeout, interval).Should(Equal(customImage))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, module)).Should(Succeed())
		})
	})

	Context("Security context validation", func() {
		It("should create DaemonSet with privileged containers", func() {
			ctx := context.Background()

			moduleName := "test-security-module"
			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}

			By("Creating the GTP5GModule resource")
			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			By("Verifying security context")
			dsName := "gtp5g-installer-" + moduleName
			ds := &appsv1.DaemonSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      dsName,
					Namespace: "default",
				}, ds)
			}, timeout, interval).Should(Succeed())

			container := ds.Spec.Template.Spec.Containers[0]
			Expect(container.SecurityContext).NotTo(BeNil())
			Expect(container.SecurityContext.Privileged).NotTo(BeNil())
			Expect(*container.SecurityContext.Privileged).To(BeTrue())

			Expect(container.SecurityContext.Capabilities).NotTo(BeNil())
			Expect(container.SecurityContext.Capabilities.Add).To(ContainElements(
				corev1.Capability("SYS_ADMIN"),
				corev1.Capability("SYS_MODULE"),
			))

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, module)).Should(Succeed())
		})
	})

	Context("Volume mount validation", func() {
		It("should mount required host paths", func() {
			ctx := context.Background()

			moduleName := "test-volume-module"
			module := &operatorv1alpha1.GTP5GModule{
				ObjectMeta: metav1.ObjectMeta{
					Name: moduleName,
				},
				Spec: operatorv1alpha1.GTP5GModuleSpec{
					Version: "v0.8.3",
				},
			}

			By("Creating the GTP5GModule resource")
			Expect(k8sClient.Create(ctx, module)).Should(Succeed())

			By("Verifying volumes")
			dsName := "gtp5g-installer-" + moduleName
			ds := &appsv1.DaemonSet{}
			Eventually(func() error {
				return k8sClient.Get(ctx, types.NamespacedName{
					Name:      dsName,
					Namespace: "default",
				}, ds)
			}, timeout, interval).Should(Succeed())

			volumes := ds.Spec.Template.Spec.Volumes
			Expect(volumes).To(HaveLen(2))

			volumeNames := make([]string, len(volumes))
			for i, v := range volumes {
				volumeNames[i] = v.Name
			}
			Expect(volumeNames).To(ContainElements("lib-modules", "usr-src"))

			By("Verifying volume mounts")
			volumeMounts := ds.Spec.Template.Spec.Containers[0].VolumeMounts
			Expect(volumeMounts).To(HaveLen(2))

			mountPaths := make(map[string]bool)
			for _, vm := range volumeMounts {
				mountPaths[vm.MountPath] = vm.ReadOnly
			}
			Expect(mountPaths).To(HaveKey("/lib/modules"))
			Expect(mountPaths["/lib/modules"]).To(BeTrue()) // Should be read-only
			Expect(mountPaths).To(HaveKey("/usr/src"))
			Expect(mountPaths["/usr/src"]).To(BeFalse()) // Should be writable

			By("Cleaning up")
			Expect(k8sClient.Delete(ctx, module)).Should(Succeed())
		})
	})
})
