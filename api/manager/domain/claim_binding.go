package domain

import "k8s.io/apimachinery/pkg/types"

// ClaimDeviceBinding describes an observed allocation, not an execution policy.
// Device identity comes exclusively from ResourceClaim.status.allocation.
type ClaimDeviceBinding struct {
	ClaimUID        types.UID
	ClaimGeneration int64
	ClaimNamespace  string
	ClaimName       string
	PodUID          types.UID
	NodeName        string
	Request         string
	Driver          string
	Pool            string
	Device          string
	NUMANodes       []int
	PCIeRoot        string
	PCIBusID        string

	// Empty provenance means topology has not been resolved.
	ResourceSliceUID        types.UID
	ResourceSliceGeneration int64
	PoolGeneration          int64
}

// ClaimBindingReader is deliberately separate from scheduling operations.
type ClaimBindingReader interface {
	// QueryClaimDeviceBindings returns a detached, deterministically ordered
	// snapshot and whether the initial Pod/ResourceClaim lists have synced.
	// An empty podUID selects all pods. This method never calls Kubernetes.
	QueryClaimDeviceBindings(podUID types.UID) ([]ClaimDeviceBinding, bool)
}
