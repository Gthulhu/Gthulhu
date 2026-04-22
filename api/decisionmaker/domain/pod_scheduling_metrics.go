// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package domain

// PodSchedulingMetricsSpec mirrors the PodSchedulingMetrics CRD spec.
// It tells the collector which pods to watch and which metrics to gather.
type PodSchedulingMetricsSpec struct {
	LabelSelectors            []LabelSelector  `json:"labelSelectors"`
	K8sNamespaces             []string         `json:"k8sNamespaces,omitempty"`
	CommandRegex              string           `json:"commandRegex,omitempty"`
	CollectionIntervalSeconds int32            `json:"collectionIntervalSeconds,omitempty"`
	Enabled                   bool             `json:"enabled"`
	Metrics                   MetricsSelection `json:"metrics,omitempty"`
	Scaling                   *ScalingHints    `json:"scaling,omitempty"`
	CreatorID                 string           `json:"creatorID,omitempty"`
	UpdaterID                 string           `json:"updaterID,omitempty"`
	CreatedTime               int64            `json:"createdTime,omitempty"`
	UpdatedTime               int64            `json:"updatedTime,omitempty"`
}

// MetricsSelection controls which scheduling metrics to collect.
type MetricsSelection struct {
	VoluntaryCtxSwitches   bool `json:"voluntaryCtxSwitches"`
	InvoluntaryCtxSwitches bool `json:"involuntaryCtxSwitches"`
	CpuTimeNs              bool `json:"cpuTimeNs"`
	WaitTimeNs             bool `json:"waitTimeNs"`
	RunCount               bool `json:"runCount"`
	CpuMigrations          bool `json:"cpuMigrations"`
}

// DefaultMetricsSelection returns the default set of metrics to collect.
func DefaultMetricsSelection() MetricsSelection {
	return MetricsSelection{
		VoluntaryCtxSwitches:   true,
		InvoluntaryCtxSwitches: true,
		CpuTimeNs:              true,
		WaitTimeNs:             false,
		RunCount:               false,
		CpuMigrations:          false,
	}
}

// ScalingHints provides KEDA auto-scaling configuration hints.
type ScalingHints struct {
	Enabled         bool            `json:"enabled"`
	MetricName      string          `json:"metricName,omitempty"`
	TargetValue     string          `json:"targetValue,omitempty"`
	ScaleTargetRef  *ScaleTargetRef `json:"scaleTargetRef,omitempty"`
	MinReplicaCount int32           `json:"minReplicaCount,omitempty"`
	MaxReplicaCount int32           `json:"maxReplicaCount,omitempty"`
	CooldownPeriod  int32           `json:"cooldownPeriod,omitempty"`
}

// ScaleTargetRef identifies the workload to scale.
type ScaleTargetRef struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name"`
}

// PodSchedulingMetricsStatus reports the runtime state of the metric collection.
type PodSchedulingMetricsStatus struct {
	Phase              string      `json:"phase,omitempty"`
	MatchedPodCount    int32       `json:"matchedPodCount,omitempty"`
	LastCollectionTime string      `json:"lastCollectionTime,omitempty"`
	Conditions         []Condition `json:"conditions,omitempty"`
}

// Condition describes a single aspect of the current state.
type Condition struct {
	Type               string `json:"type"`
	Status             string `json:"status"` // "True", "False", "Unknown"
	LastTransitionTime string `json:"lastTransitionTime,omitempty"`
	Reason             string `json:"reason,omitempty"`
	Message            string `json:"message,omitempty"`
}

// PodSchedulingMetrics is the full in-memory representation of the CRD.
type PodSchedulingMetrics struct {
	Name      string                     `json:"name"`
	Namespace string                     `json:"namespace"`
	Spec      PodSchedulingMetricsSpec   `json:"spec"`
	Status    PodSchedulingMetricsStatus `json:"status,omitempty"`
}

// ---- Collected data model (output of the eBPF collector) ----

// TaskSchedMetrics holds per-PID scheduling metrics collected from eBPF maps.
type TaskSchedMetrics struct {
	PID                    uint32 `json:"pid"`
	TGID                   uint32 `json:"tgid"`
	VoluntaryCtxSwitches   uint64 `json:"voluntaryCtxSwitches"`
	InvoluntaryCtxSwitches uint64 `json:"involuntaryCtxSwitches"`
	CpuTimeNs              uint64 `json:"cpuTimeNs"`
	WaitTimeNs             uint64 `json:"waitTimeNs"`
	RunCount               uint64 `json:"runCount"`
	CpuMigrations          uint32 `json:"cpuMigrations"`
	SMTMigrations          uint32 `json:"smtMigrations"`
	L3Migrations           uint32 `json:"l3Migrations"`
	NUMAMigrations         uint32 `json:"numaMigrations"`
	LastCPU                uint32 `json:"lastCpu"`
}

// PodSchedMetrics holds aggregated scheduling metrics for a single pod.
type PodSchedMetrics struct {
	PodName                string `json:"podName"`
	PodUID                 string `json:"podUID"`
	Namespace              string `json:"namespace"`
	NodeName               string `json:"nodeName"`
	VoluntaryCtxSwitches   uint64 `json:"voluntaryCtxSwitches"`
	InvoluntaryCtxSwitches uint64 `json:"involuntaryCtxSwitches"`
	CpuTimeNs              uint64 `json:"cpuTimeNs"`
	WaitTimeNs             uint64 `json:"waitTimeNs"`
	RunCount               uint64 `json:"runCount"`
	CpuMigrations          uint32 `json:"cpuMigrations"`
	SMTMigrations          uint32 `json:"smtMigrations"`
	L3Migrations           uint32 `json:"l3Migrations"`
	NUMAMigrations         uint32 `json:"numaMigrations"`
	ProcessCount           int    `json:"processCount"`
}
