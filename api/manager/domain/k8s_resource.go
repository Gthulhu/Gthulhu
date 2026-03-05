package domain

// Node represents a Kubernetes node
type Node struct {
	Name   string            `json:"name"`
	Labels map[string]string `json:"labels,omitempty"`
	Status string            `json:"status"`
}

type DecisionMakerPod struct {
	NodeID string
	Port   int
	Host   string
	State  NodeState
}

func (d *DecisionMakerPod) String() string {
	return "(" + d.NodeID + ")" + d.Host + ":" + string(rune(d.Port))
}

type Pod struct {
	Name         string
	K8SNamespace string
	Labels       map[string]string
	PodID        string
	NodeID       string
	Containers   []Container
}

func (p *Pod) LabelsToSelectors() []LabelSelector {
	selectors := make([]LabelSelector, 0, len(p.Labels))
	for k, v := range p.Labels {
		selectors = append(selectors, LabelSelector{
			Key:   k,
			Value: v,
		})
	}
	return selectors
}

type Container struct {
	ContainerID string
	Name        string
	Command     []string
}

// PodProcess represents a process information within a pod
type PodProcess struct {
	PID         int    `json:"pid"`
	Command     string `json:"command"`
	PPID        int    `json:"ppid,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
}

// PodPIDInfo represents pod information with associated processes
type PodPIDInfo struct {
	PodUID    string       `json:"pod_uid"`
	PodID     string       `json:"pod_id,omitempty"`
	Processes []PodProcess `json:"processes"`
}

// PodPIDMappingResponse represents the response from Decision Maker's Pod-PID mapping API
type PodPIDMappingResponse struct {
	Pods      []PodPIDInfo `json:"pods"`
	Timestamp string       `json:"timestamp"`
	NodeName  string       `json:"node_name"`
	NodeID    string       `json:"node_id,omitempty"`
}
