package domain

// PodSchedulingMetrics represents a PodSchedulingMetrics CRD instance.
type PodSchedulingMetrics struct {
	BaseEntity                `bson:",inline"`
	LabelSelectors            []LabelSelector `bson:"labelSelectors,omitempty"`
	K8sNamespaces             []string        `bson:"k8sNamespaces,omitempty"`
	CommandRegex              string          `bson:"commandRegex,omitempty"`
	CollectionIntervalSeconds int32           `bson:"collectionIntervalSeconds,omitempty"`
	Enabled                   bool            `bson:"enabled"`
	Metrics                   *PSMMetrics     `bson:"metrics,omitempty"`
	Scaling                   *PSMScaling     `bson:"scaling,omitempty"`
}

// PSMMetrics controls which scheduling metrics to collect.
type PSMMetrics struct {
	VoluntaryCtxSwitches   bool `bson:"voluntaryCtxSwitches"`
	InvoluntaryCtxSwitches bool `bson:"involuntaryCtxSwitches"`
	CPUTimeNs              bool `bson:"cpuTimeNs"`
	WaitTimeNs             bool `bson:"waitTimeNs"`
	RunCount               bool `bson:"runCount"`
	CPUMigrations          bool `bson:"cpuMigrations"`
}

// PSMScaling contains optional KEDA auto-scaling hints.
type PSMScaling struct {
	Enabled         bool               `bson:"enabled"`
	MetricName      string             `bson:"metricName,omitempty"`
	TargetValue     string             `bson:"targetValue,omitempty"`
	ScaleTargetRef  *PSMScaleTargetRef `bson:"scaleTargetRef,omitempty"`
	MinReplicaCount int32              `bson:"minReplicaCount,omitempty"`
	MaxReplicaCount int32              `bson:"maxReplicaCount,omitempty"`
	CooldownPeriod  int32              `bson:"cooldownPeriod,omitempty"`
}

// PSMScaleTargetRef identifies the workload to scale.
type PSMScaleTargetRef struct {
	APIVersion string `bson:"apiVersion,omitempty"`
	Kind       string `bson:"kind,omitempty"`
	Name       string `bson:"name,omitempty"`
}

// QueryPSMOptions is the query option struct for listing PodSchedulingMetrics.
type QueryPSMOptions struct {
	IDs        []interface{} // either bson.ObjectID or string names
	CreatorIDs []interface{}
	Result     []*PodSchedulingMetrics
}
