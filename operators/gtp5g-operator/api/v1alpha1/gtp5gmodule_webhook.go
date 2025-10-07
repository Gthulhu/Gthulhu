package v1alpha1

import (
	"fmt"
	"regexp"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var gtp5gmodulelog = logf.Log.WithName("gtp5gmodule-resource")

// versionPattern validates semantic versioning with 'v' prefix
var versionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)

func (r *GTP5GModule) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// +kubebuilder:webhook:path=/mutate-operator-gthulhu-io-v1alpha1-gtp5gmodule,mutating=true,failurePolicy=fail,sideEffects=None,groups=operator.gthulhu.io,resources=gtp5gmodules,verbs=create;update,versions=v1alpha1,name=mgtp5gmodule.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &GTP5GModule{}

// Default implements webhook.Defaulter
func (r *GTP5GModule) Default() {
	gtp5gmodulelog.Info("default", "name", r.Name)

	// Set default node selector if not provided
	if r.Spec.NodeSelector == nil {
		r.Spec.NodeSelector = map[string]string{
			"gtp5g.gthulhu.io/enabled": "true",
		}
	}

	// Set default installer image if not provided
	if r.Spec.Image == "" {
		r.Spec.Image = "localhost:5000/gtp5g-installer:latest"
	}
}

// +kubebuilder:webhook:path=/validate-operator-gthulhu-io-v1alpha1-gtp5gmodule,mutating=false,failurePolicy=fail,sideEffects=None,groups=operator.gthulhu.io,resources=gtp5gmodules,verbs=create;update,versions=v1alpha1,name=vgtp5gmodule.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &GTP5GModule{}

// ValidateCreate implements webhook.Validator
func (r *GTP5GModule) ValidateCreate() (admission.Warnings, error) {
	gtp5gmodulelog.Info("validate create", "name", r.Name)

	if err := r.validateGTP5GModule(); err != nil {
		return nil, err
	}

	return nil, nil
}

// ValidateUpdate implements webhook.Validator
func (r *GTP5GModule) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	gtp5gmodulelog.Info("validate update", "name", r.Name)

	if err := r.validateGTP5GModule(); err != nil {
		return nil, err
	}

	// Allow all updates for now
	// Could add immutability checks in future if needed
	return nil, nil
}

// ValidateDelete implements webhook.Validator
func (r *GTP5GModule) ValidateDelete() (admission.Warnings, error) {
	gtp5gmodulelog.Info("validate delete", "name", r.Name)

	// No validation needed for deletion
	return nil, nil
}

// validateGTP5GModule contains common validation logic
func (r *GTP5GModule) validateGTP5GModule() error {
	// Validate version is required
	if r.Spec.Version == "" {
		return fmt.Errorf("spec.version is required")
	}

	// Validate version format
	if !versionPattern.MatchString(r.Spec.Version) {
		return fmt.Errorf("spec.version must match pattern ^v[0-9]+\\.[0-9]+\\.[0-9]+$, got: %s", r.Spec.Version)
	}

	return nil
}
