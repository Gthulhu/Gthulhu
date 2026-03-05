package domain

import (
	"github.com/Gthulhu/api/pkg/util"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type ScheduleStrategy struct {
	BaseEntity        `bson:",inline"`
	StrategyNamespace string          `bson:"strategyNamespace,omitempty"`
	LabelSelectors    []LabelSelector `bson:"labelSelectors,omitempty"`
	K8sNamespace      []string        `bson:"k8sNamespace,omitempty"`
	CommandRegex      string          `bson:"commandRegex,omitempty"`
	Priority          int             `bson:"priority,omitempty"`
	ExecutionTime     int64           `bson:"executionTime,omitempty"`
}

func NewScheduleIntent(strategy *ScheduleStrategy, pod *Pod) ScheduleIntent {
	return ScheduleIntent{
		BaseEntity:    NewBaseEntity(util.Ptr(strategy.CreatorID), util.Ptr(strategy.UpdaterID)),
		StrategyID:    strategy.ID,
		PodID:         pod.PodID,
		NodeID:        pod.NodeID,
		K8sNamespace:  pod.K8SNamespace,
		CommandRegex:  strategy.CommandRegex,
		Priority:      strategy.Priority,
		ExecutionTime: strategy.ExecutionTime,
		PodLabels:     pod.Labels,
		State:         IntentStateInitialized,
		PodName:       pod.Name,
	}
}

type ScheduleIntent struct {
	BaseEntity    `bson:",inline"`
	StrategyID    bson.ObjectID     `bson:"strategyID,omitempty"`
	PodID         string            `bson:"podID,omitempty"`
	PodName       string            `bson:"podName,omitempty"`
	NodeID        string            `bson:"nodeID,omitempty"`
	K8sNamespace  string            `bson:"k8sNamespace,omitempty"`
	CommandRegex  string            `bson:"commandRegex,omitempty"`
	Priority      int               `bson:"priority,omitempty"`
	ExecutionTime int64             `bson:"executionTime,omitempty"`
	PodLabels     map[string]string `bson:"podLabels,omitempty"`
	State         IntentState       `bson:"state,omitempty"`
}

type LabelSelector struct {
	Key   string `bson:"key,omitempty"`
	Value string `bson:"value,omitempty"`
}
