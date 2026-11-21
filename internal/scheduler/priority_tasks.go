package scheduler

import (
	"fmt"
	"log/slog"
	"math"
	"slices"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

type kernelStrategy struct {
	PID           int                     `json:"pid"`
	Priority      int                     `json:"priority"`
	ExecutionTime uint64                  `json:"execution_time"`
	NodePolicy    *domain.NodePolicyMatch `json:"nodePolicyMatch,omitempty"`
}

func (s kernelStrategy) equal(other kernelStrategy) bool {
	if s.PID != other.PID || s.Priority != other.Priority || s.ExecutionTime != other.ExecutionTime {
		return false
	}
	if s.NodePolicy == nil || other.NodePolicy == nil {
		return s.NodePolicy == other.NodePolicy
	}
	return *s.NodePolicy == *other.NodePolicy
}

type priorityTaskReadback struct {
	slicePresent bool
	levelPresent bool
	slice        uint64
	level        uint32
}

func (r priorityTaskReadback) matches(strategy kernelStrategy) bool {
	return r.slicePresent && r.levelPresent && r.slice == strategy.ExecutionTime && r.level == uint32(strategy.Priority)
}

type priorityTaskWriter interface {
	UpdatePriorityTaskWithPrio(key uint32, slice uint64, level uint32) error
	RemovePriorityTask(key uint32) error
}

type kernelPriorityTasks struct {
	writer priorityTaskWriter
	lookup func(uint32) (priorityTaskReadback, error)
	logger *slog.Logger
	last   map[int]kernelStrategy
}

// reconcile accepts an authoritative snapshot, not fetch failures. Retain the
// previous snapshot so removals can report the original task identity even
// after the task exits or the policy disappears from Decision Maker.
func (p *kernelPriorityTasks) reconcile(snapshot []kernelStrategy) {
	next := make(map[int]kernelStrategy, len(snapshot))
	for _, strategy := range snapshot {
		if strategy.PID <= 0 || uint64(strategy.PID) > math.MaxInt32 {
			p.logger.Warn("Invalid kernel strategy PID", "pid", strategy.PID)
			continue
		}
		if m := strategy.NodePolicy; m != nil && (strategy.PID != m.IntendedKey || strategy.PID != m.Task.TID) {
			p.logger.Warn("Invalid node policy key", matchLogAttrs(strategy)...)
			continue
		}
		next[strategy.PID] = strategy // same last-wins convention as the plugin
	}

	var changed, removed []int
	for pid, strategy := range next {
		if previous, ok := p.last[pid]; !ok || !strategy.equal(previous) {
			changed = append(changed, pid)
		}
	}
	for pid := range p.last {
		if _, ok := next[pid]; !ok {
			removed = append(removed, pid)
		}
	}
	slices.Sort(changed)
	slices.Sort(removed)
	for _, pid := range changed {
		strategy := next[pid]
		if strategy.Priority <= 0 {
			// qumun treats priority 0 as highest and has no slice-only state.
			// When superseding a boost, attribute removal to its old provenance.
			if previous := p.last[pid]; previous.Priority > 0 {
				strategy = previous
			}
			p.remove(strategy, "non_positive_priority")
		} else {
			p.apply(strategy, p.last[pid])
		}
	}
	for _, pid := range removed {
		p.remove(p.last[pid], "strategy_removed")
	}
	p.last = next
}

func (p *kernelPriorityTasks) apply(strategy, previous kernelStrategy) {
	key := uint32(strategy.PID)
	var err error
	if uint64(strategy.Priority) > math.MaxUint32 {
		err = fmt.Errorf("priority %d exceeds BPF map value width", strategy.Priority)
	} else {
		err = p.writer.UpdatePriorityTaskWithPrio(key, strategy.ExecutionTime, uint32(strategy.Priority))
	}
	replaced := previous.Priority > 0 && previous.NodePolicy != nil &&
		(strategy.NodePolicy == nil || previous.NodePolicy.Policy.PolicyID != strategy.NodePolicy.Policy.PolicyID)
	if strategy.NodePolicy != nil || replaced {
		observed, lookupErr := p.lookup(key)
		if replaced {
			p.logNodePolicyChange("node_policy_remove", previous, "strategy_replaced", &strategy, observed, err, lookupErr)
		}
		if strategy.NodePolicy != nil {
			p.logNodePolicyChange("node_policy_apply", strategy, "", nil, observed, err, lookupErr)
			return
		}
	}
	if err != nil {
		p.logger.Warn("UpdatePriorityTaskWithPrio failed", "error", err, "pid", strategy.PID)
	} else {
		p.logger.Info("Updated priority task", "pid", strategy.PID, "executionTime", strategy.ExecutionTime, "priority", strategy.Priority)
	}
}

func (p *kernelPriorityTasks) remove(strategy kernelStrategy, reason string) {
	err := p.writer.RemovePriorityTask(uint32(strategy.PID))
	if strategy.NodePolicy != nil {
		observed, lookupErr := p.lookup(uint32(strategy.PID))
		p.logNodePolicyChange("node_policy_remove", strategy, reason, nil, observed, err, lookupErr)
	} else if err != nil {
		p.logger.Warn("RemovePriorityTask failed", "error", err, "pid", strategy.PID, "reason", reason)
	} else if reason == "strategy_removed" {
		p.logger.Info("Removed priority task", "pid", strategy.PID)
	}
}

func matchLogAttrs(strategy kernelStrategy) []any {
	m := strategy.NodePolicy
	return []any{
		"policy_id", m.Policy.PolicyID, "generation", m.Policy.Generation,
		"command_regex", m.Policy.CommandRegex,
		"node", m.Task.Node, "tgid", m.Task.TGID, "tid", m.Task.TID,
		"comm", m.Task.Comm, "start_time", m.Task.StartTime,
		"cgroup_path", m.Task.CgroupPath, "pod_uid", m.Task.PodUID,
		"kernel_thread", m.Task.KernelThread, "intended_key", m.IntendedKey,
		"pid", strategy.PID, "priority", strategy.Priority, "execution_time", strategy.ExecutionTime,
	}
}

func (p *kernelPriorityTasks) logNodePolicyChange(event string, strategy kernelStrategy, removalReason string, replacement *kernelStrategy, observed priorityTaskReadback, opErr, lookupErr error) {
	key := uint32(strategy.PID)
	attrs := append(matchLogAttrs(strategy), "lookup_key", key)
	if removalReason != "" {
		attrs = append(attrs, "reason", removalReason)
	}
	if replacement != nil && replacement.NodePolicy != nil {
		attrs = append(attrs, "replacement_policy_id", replacement.NodePolicy.Policy.PolicyID)
	}
	// A successful operation is not readback. Query the exact key on both
	// loaded maps; verify presence and values on apply or replacement, and
	// absence on removal without replacement.
	// This is a point-in-time observation, not proof of a scheduling decision.
	present := observed.slicePresent || observed.levelPresent
	if lookupErr == nil {
		attrs = append(attrs, "slice_present", observed.slicePresent, "priority_present", observed.levelPresent)
		if removalReason != "" {
			attrs = append(attrs, "map_removed", !present)
		}
		if present {
			attrs = append(attrs, "actual_map_key", key)
		}
		if observed.slicePresent {
			attrs = append(attrs, "actual_execution_time", observed.slice)
		}
		if observed.levelPresent {
			attrs = append(attrs, "actual_priority", observed.level)
		}
	}
	switch {
	case opErr != nil:
		attrs = append(attrs, "operation_error", opErr)
		if lookupErr != nil {
			attrs = append(attrs, "readback_error", lookupErr)
		}
		p.logger.Warn(event, append(attrs, "result", "operation_failed")...)
	case lookupErr != nil:
		p.logger.Warn(event, append(attrs, "result", "readback_failed", "readback_error", lookupErr)...)
	case replacement != nil && !observed.matches(*replacement):
		p.logger.Warn(event, append(attrs, "result", "readback_mismatch")...)
	case replacement == nil && removalReason != "" && present:
		p.logger.Warn(event, append(attrs, "result", "readback_mismatch")...)
	case removalReason == "" && !observed.matches(strategy):
		p.logger.Warn(event, append(attrs, "result", "readback_mismatch")...)
	default:
		p.logger.Info(event, append(attrs, "result", "verified")...)
	}
}
