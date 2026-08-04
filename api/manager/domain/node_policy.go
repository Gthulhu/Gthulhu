package domain

import (
	"github.com/Gthulhu/api/pkg/util"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// DRASelector matches nodes that expose a given Dynamic Resource Allocation
// (DRA, resource.k8s.io) device class, optionally narrowed by device
// attribute key/value pairs (as published in ResourceSlice objects).
type DRASelector struct {
	DeviceClass string          `bson:"deviceClass,omitempty"`
	Attributes  []LabelSelector `bson:"attributes,omitempty"`
}

// NodeSchedulingPolicy describes how to find processes that are not
// necessarily part of any Pod's container cgroup (kthreads, host daemons,
// etc.) on a set of matching nodes. Node matching is the AND of every
// selector type that is non-empty:
//   - NodeSelectors: node label selectors
//   - NodeNames: explicit node names
//   - DRASelectors: nodes that have the referenced DRA device class/attributes allocated
//
// Process matching happens by executable/comm name via CommandRegex - there
// is no Pod/K8sNamespace concept here.
type NodeSchedulingPolicy struct {
	BaseEntity    `bson:",inline"`
	NodeSelectors []LabelSelector `bson:"nodeSelectors,omitempty"`
	NodeNames     []string        `bson:"nodeNames,omitempty"`
	DRASelectors  []DRASelector   `bson:"draSelectors,omitempty"`
	CommandRegex  string          `bson:"commandRegex,omitempty"`
	Priority      int             `bson:"priority,omitempty"`
	ExecutionTime int64           `bson:"executionTime,omitempty"`
}

// NodeSchedulingIntent is the per-node resolution of a NodeSchedulingPolicy,
// analogous to ScheduleIntent but keyed by NodeID + PolicyID since there is
// no Pod to key on.
type NodeSchedulingIntent struct {
	BaseEntity    `bson:",inline"`
	PolicyID      bson.ObjectID `bson:"policyID,omitempty"`
	NodeID        string        `bson:"nodeID,omitempty"`
	CommandRegex  string        `bson:"commandRegex,omitempty"`
	Priority      int           `bson:"priority,omitempty"`
	ExecutionTime int64         `bson:"executionTime,omitempty"`
	State         IntentState   `bson:"state,omitempty"`
}

func NewNodeSchedulingIntent(policy *NodeSchedulingPolicy, node *Node) NodeSchedulingIntent {
	return NodeSchedulingIntent{
		BaseEntity:    NewBaseEntity(util.Ptr(policy.CreatorID), util.Ptr(policy.UpdaterID)),
		PolicyID:      policy.ID,
		NodeID:        node.Name,
		CommandRegex:  policy.CommandRegex,
		Priority:      policy.Priority,
		ExecutionTime: policy.ExecutionTime,
		State:         IntentStateInitialized,
	}
}
