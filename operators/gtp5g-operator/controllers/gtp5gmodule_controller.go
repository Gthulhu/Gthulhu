package controllers

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/metrics"

	"github.com/prometheus/client_golang/prometheus"
	operatorv1alpha1 "github.com/Gthulhu/Gthulhu/operators/gtp5g-operator/api/v1alpha1"
)

const (
	finalizerName = "operator.gthulhu.io/finalizer"
	// Requeue delays
	requeueDelayOnError   = time.Second * 30
	requeueDelayOnSuccess = time.Minute * 5
)

var (
	// Metrics
	reconcileCounter = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "gtp5g_operator_reconcile_total",
			Help: "Total number of reconciliations per GTP5GModule",
		},
		[]string{"name", "result"},
	)

	reconcileDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "gtp5g_operator_reconcile_duration_seconds",
			Help:    "Duration of reconciliations in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"name"},
	)

	modulePhaseGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "gtp5g_operator_module_phase",
			Help: "Current phase of GTP5GModule (0=Pending, 1=Installing, 2=Installed, 3=Failed)",
		},
		[]string{"name"},
	)
)

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(
		reconcileCounter,
		reconcileDuration,
		modulePhaseGauge,
	)
}

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
	startTime := time.Now()
	logger := log.FromContext(ctx)

	logger.Info("Starting reconciliation", "module", req.NamespacedName)

	// Track reconciliation duration
	defer func() {
		duration := time.Since(startTime).Seconds()
		reconcileDuration.WithLabelValues(req.Name).Observe(duration)
		logger.Info("Reconciliation completed", "module", req.NamespacedName, "duration_seconds", duration)
	}()

	// Fetch the GTP5GModule instance
	module := &operatorv1alpha1.GTP5GModule{}
	if err := r.Get(ctx, req.NamespacedName, module); err != nil {
		if apierrors.IsNotFound(err) {
			logger.Info("GTP5GModule not found, likely deleted", "module", req.NamespacedName)
			reconcileCounter.WithLabelValues(req.Name, "deleted").Inc()
			return ctrl.Result{}, nil
		}
		logger.Error(err, "Failed to get GTP5GModule")
		reconcileCounter.WithLabelValues(req.Name, "error").Inc()
		return ctrl.Result{RequeueAfter: requeueDelayOnError}, err
	}

	// Handle deletion
	if !module.DeletionTimestamp.IsZero() {
		logger.Info("Handling deletion", "module", module.Name)
		result, err := r.handleDeletion(ctx, module)
		if err != nil {
			logger.Error(err, "Failed to handle deletion", "module", module.Name)
			reconcileCounter.WithLabelValues(module.Name, "deletion_error").Inc()
			return ctrl.Result{RequeueAfter: requeueDelayOnError}, err
		}
		reconcileCounter.WithLabelValues(module.Name, "deleted").Inc()
		return result, nil
	}

	// Add finalizer if not present
	if !containsString(module.Finalizers, finalizerName) {
		logger.Info("Adding finalizer", "module", module.Name, "finalizer", finalizerName)
		module.Finalizers = append(module.Finalizers, finalizerName)
		if err := r.Update(ctx, module); err != nil {
			logger.Error(err, "Failed to add finalizer", "module", module.Name)
			reconcileCounter.WithLabelValues(module.Name, "error").Inc()
			return ctrl.Result{RequeueAfter: requeueDelayOnError}, err
		}
		// Requeue to ensure finalizer is persisted
		return ctrl.Result{Requeue: true}, nil
	}

	// Reconcile DaemonSet
	logger.Info("Reconciling DaemonSet", "module", module.Name)
	if err := r.reconcileDaemonSet(ctx, module); err != nil {
		logger.Error(err, "Failed to reconcile DaemonSet", "module", module.Name)
		reconcileCounter.WithLabelValues(module.Name, "daemonset_error").Inc()

		// Update status to Failed
		module.Status.Phase = operatorv1alpha1.ModulePhaseFailed
		module.Status.Message = fmt.Sprintf("Failed to reconcile DaemonSet: %v", err)
		module.Status.LastUpdateTime = metav1.Now()
		if statusErr := r.Status().Update(ctx, module); statusErr != nil {
			logger.Error(statusErr, "Failed to update failed status", "module", module.Name)
		}

		return ctrl.Result{RequeueAfter: requeueDelayOnError}, err
	}

	// Update status
	logger.Info("Updating status", "module", module.Name)
	if err := r.updateStatus(ctx, module); err != nil {
		logger.Error(err, "Failed to update status", "module", module.Name)
		reconcileCounter.WithLabelValues(module.Name, "status_error").Inc()
		return ctrl.Result{RequeueAfter: requeueDelayOnError}, err
	}

	// Update metrics
	r.updateMetrics(module)

	logger.Info("Reconciliation successful", "module", module.Name, "phase", module.Status.Phase)
	reconcileCounter.WithLabelValues(module.Name, "success").Inc()

	// Requeue after some time to check for updates
	return ctrl.Result{RequeueAfter: requeueDelayOnSuccess}, nil
}

// updateMetrics updates Prometheus metrics based on module status
func (r *GTP5GModuleReconciler) updateMetrics(module *operatorv1alpha1.GTP5GModule) {
	phase := 0.0
	switch module.Status.Phase {
	case operatorv1alpha1.ModulePhasePending:
		phase = 0.0
	case operatorv1alpha1.ModulePhaseInstalling:
		phase = 1.0
	case operatorv1alpha1.ModulePhaseInstalled:
		phase = 2.0
	case operatorv1alpha1.ModulePhaseFailed:
		phase = 3.0
	}
	modulePhaseGauge.WithLabelValues(module.Name).Set(phase)
}

func (r *GTP5GModuleReconciler) reconcileDaemonSet(ctx context.Context, module *operatorv1alpha1.GTP5GModule) error {
	logger := log.FromContext(ctx)
	desired := r.constructDaemonSet(module)

	existing := &appsv1.DaemonSet{}
	err := r.Get(ctx, client.ObjectKey{
		Name:      desired.Name,
		Namespace: desired.Namespace,
	}, existing)

	if err != nil && apierrors.IsNotFound(err) {
		logger.Info("Creating DaemonSet", "name", desired.Name, "namespace", desired.Namespace)
		if err := r.Create(ctx, desired); err != nil {
			return fmt.Errorf("failed to create DaemonSet: %w", err)
		}
		logger.Info("DaemonSet created successfully", "name", desired.Name)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to get DaemonSet: %w", err)
	}

	// Update if needed
	logger.Info("Updating DaemonSet", "name", existing.Name, "namespace", existing.Namespace)
	existing.Spec = desired.Spec
	if err := r.Update(ctx, existing); err != nil {
		return fmt.Errorf("failed to update DaemonSet: %w", err)
	}
	logger.Info("DaemonSet updated successfully", "name", existing.Name)

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
