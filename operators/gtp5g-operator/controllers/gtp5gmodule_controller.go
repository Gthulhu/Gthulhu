package controllers

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	operatorv1alpha1 "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/api/v1alpha1"
)

const finalizerName = "operator.gthulhu.io/finalizer"

// GTP5GModuleReconciler reconciles a GTP5GModule object
type GTP5GModuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=operator.gthulhu.io,resources=gtp5gmodules,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=operator.gthulhu.io,resources=gtp5gmodules/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=operator.gthulhu.io,resources=gtp5gmodules/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=daemonsets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups="",resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups="",resources=pods,verbs=get;list;watch

func (r *GTP5GModuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	// Fetch the GTP5GModule instance
	module := &operatorv1alpha1.GTP5GModule{}
	if err := r.Get(ctx, req.NamespacedName, module); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Handle deletion
	if !module.DeletionTimestamp.IsZero() {
		return r.handleDeletion(ctx, module)
	}

	// Add finalizer if not present
	if !containsString(module.Finalizers, finalizerName) {
		module.Finalizers = append(module.Finalizers, finalizerName)
		if err := r.Update(ctx, module); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Reconcile DaemonSet
	if err := r.reconcileDaemonSet(ctx, module); err != nil {
		logger.Error(err, "Failed to reconcile DaemonSet")
		return ctrl.Result{}, err
	}

	// Update status
	if err := r.updateStatus(ctx, module); err != nil {
		logger.Error(err, "Failed to update status")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *GTP5GModuleReconciler) reconcileDaemonSet(ctx context.Context, module *operatorv1alpha1.GTP5GModule) error {
	desired := r.constructDaemonSet(module)

	existing := &appsv1.DaemonSet{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if err != nil && apierrors.IsNotFound(err) {
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("failed to create DaemonSet: %w", err)
		}
		return nil
	} else if err != nil {
		return err
	}

	// Update if needed (simplified - just update)
	existing.Spec = desired.Spec
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update DaemonSet: %w", err)
	}

	return nil
}

func (r *GTP5GModuleReconciler) constructDaemonSet(module *operatorv1alpha1.GTP5GModule) *appsv1.DaemonSet {
	labels := map[string]string{
		"app":                          "gtp5g-installer",
		"gtp5g.gthulhu.io/module-name": module.Name,
	}

	image := module.Spec.Image
	if image == "" {
		image = "localhost:5000/gtp5g-installer:latest"
	}

	privileged := true

	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("gtp5g-installer-%s", module.Name),
			Namespace: "default",
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(module, operatorv1alpha1.GroupVersion.WithKind("GTP5GModule")),
			},
		},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					HostPID:            true,
					ServiceAccountName: "default",
					Containers: []corev1.Container{
						{
							Name:  "installer",
							Image: image,
							SecurityContext: &corev1.SecurityContext{
								Privileged: &privileged,
								Capabilities: &corev1.Capabilities{
									Add: []corev1.Capability{
										"SYS_ADMIN",
										"SYS_MODULE",
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "GTP5G_VERSION",
									Value: module.Spec.Version,
								},
								{
									Name:  "KERNEL_VERSION",
									Value: module.Spec.KernelVersion,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "lib-modules",
									MountPath: "/lib/modules",
									ReadOnly:  true,
								},
								{
									Name:      "usr-src",
									MountPath: "/usr/src",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "lib-modules",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/lib/modules",
								},
							},
						},
						{
							Name: "usr-src",
							VolumeSource: corev1.VolumeSource{
								HostPath: &corev1.HostPathVolumeSource{
									Path: "/usr/src",
								},
							},
						},
					},
					NodeSelector: module.Spec.NodeSelector,
				},
			},
		},
	}
}

func (r *GTP5GModuleReconciler) updateStatus(ctx context.Context, module *operatorv1alpha1.GTP5GModule) error {
	ds := &appsv1.DaemonSet{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      fmt.Sprintf("gtp5g-installer-%s", module.Name),
		Namespace: "default",
	}, ds)
	if err != nil {
		return err
	}

	if ds.Status.NumberReady == ds.Status.DesiredNumberScheduled && ds.Status.DesiredNumberScheduled > 0 {
		module.Status.Phase = operatorv1alpha1.ModulePhaseInstalled
		module.Status.Message = "All nodes have gtp5g installed"
	} else if ds.Status.NumberReady > 0 {
		module.Status.Phase = operatorv1alpha1.ModulePhaseInstalling
		module.Status.Message = fmt.Sprintf("%d/%d nodes ready", ds.Status.NumberReady, ds.Status.DesiredNumberScheduled)
	} else {
		module.Status.Phase = operatorv1alpha1.ModulePhasePending
		module.Status.Message = "Waiting for installer pods"
	}

	module.Status.LastUpdateTime = metav1.Now()

	if err := r.Status().Update(ctx, module); err != nil {
		return err
	}

	return nil
}

func (r *GTP5GModuleReconciler) handleDeletion(ctx context.Context, module *operatorv1alpha1.GTP5GModule) (ctrl.Result, error) {
	if containsString(module.Finalizers, finalizerName) {
		// Remove finalizer
		module.Finalizers = removeString(module.Finalizers, finalizerName)
		if err := r.Update(ctx, module); err != nil {
			return ctrl.Result{}, err
		}
	}
	return ctrl.Result{}, nil
}

func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	result := []string{}
	for _, item := range slice {
		if item != s {
			result = append(result, item)
		}
	}
	return result
}

func (r *GTP5GModuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&operatorv1alpha1.GTP5GModule{}).
		Owns(&appsv1.DaemonSet{}).
		Complete(r)
}
