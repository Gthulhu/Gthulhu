package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// GTP5GModuleSpec defines the desired state of GTP5GModule
type GTP5GModuleSpec struct {
	// Version is the gtp5g module version (git tag)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=^v[0-9]+\.[0-9]+\.[0-9]+$
	Version string `json:"version"`

	// KernelVersion specifies target kernel version (optional, auto-detect if empty)
	// +optional
	KernelVersion string `json:"kernelVersion,omitempty"`

	// NodeSelector selects nodes to install the module
	// +optional
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`

	// Image is the installer container image (optional)
	// +optional
	Image string `json:"image,omitempty"`
}

// ModulePhase is the lifecycle phase of the module
// +kubebuilder:validation:Enum=Pending;Installing;Installed;Failed
type ModulePhase string

const (
	ModulePhasePending    ModulePhase = "Pending"
	ModulePhaseInstalling ModulePhase = "Installing"
	ModulePhaseInstalled  ModulePhase = "Installed"
	ModulePhaseFailed     ModulePhase = "Failed"
)

// NodeFailure records node installation failure info
type NodeFailure struct {
	NodeName string `json:"nodeName"`
	Reason   string `json:"reason"`
}

// GTP5GModuleStatus defines the observed state of GTP5GModule
type GTP5GModuleStatus struct {
	// Phase indicates current status
	// +optional
	Phase ModulePhase `json:"phase,omitempty"`

	// InstalledNodes is the list of nodes with module successfully installed
	// +optional
	InstalledNodes []string `json:"installedNodes,omitempty"`

	// FailedNodes is the list of nodes where installation failed
	// +optional
	FailedNodes []NodeFailure `json:"failedNodes,omitempty"`

	// Message provides human-readable status information
	// +optional
	Message string `json:"message,omitempty"`

	// LastUpdateTime is the last update timestamp
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Cluster
// +kubebuilder:printcolumn:name="Version",type=string,JSONPath=`.spec.version`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Installed",type=integer,JSONPath=`.status.installedNodes`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// GTP5GModule is the Schema for the gtp5gmodules API
type GTP5GModule struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GTP5GModuleSpec   `json:"spec,omitempty"`
	Status GTP5GModuleStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GTP5GModuleList contains a list of GTP5GModule
type GTP5GModuleList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GTP5GModule `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GTP5GModule{}, &GTP5GModuleList{})
}
