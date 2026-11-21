package service

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

var ErrNodePolicyNotFound = errors.New("node policy not found")

func (svc *Service) PreviewNodePolicy(ctx context.Context, req domain.NodePolicyPreviewRequest) (*domain.NodePolicyPreview, error) {
	cached := svc.cachedNodePolicies()
	policy := req.Policy
	if policy == nil {
		for _, candidate := range cached {
			if candidate.PolicyID == req.PolicyID && (policy == nil || candidate.Generation > policy.Generation) {
				policy = candidate
			}
		}
		if policy == nil {
			return nil, ErrNodePolicyNotFound
		}
	}
	node := policyNode(*policy, localNodeName())
	result := &domain.NodePolicyPreview{
		Policy: *policy, MatchedTasks: []domain.TaskInfo{}, Conflicts: []domain.NodePolicyConflict{}, Warnings: []domain.NodePolicyWarning{},
	}
	warn := func(code, message string) {
		result.Warnings = append(result.Warnings, domain.NodePolicyWarning{Node: node, Code: code, Message: message})
	}
	if policy.CommandRegex == "" {
		warn("invalidRegex", "commandRegex must not be empty")
		return result, nil
	}
	if _, err := regexp.Compile(policy.CommandRegex); err != nil {
		warn("invalidRegex", err.Error())
		return result, nil
	}
	if svc.taskSource == nil {
		return nil, errors.New("task source unavailable")
	}
	tasks, err := svc.taskSource.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot tasks: %w", err)
	}
	policies := []*domain.NodePolicy{policy}
	seen := make(map[domain.NodePolicy]bool)
	for _, other := range cached {
		if policy.PolicyID != "" && other.PolicyID == policy.PolicyID && other.Generation == policy.Generation {
			continue // the requested version is already represented
		}
		key := *other
		key.Node = ""
		if seen[key] {
			continue
		}
		seen[key] = true
		policies = append(policies, other)
	}

	compiled := compileNodePolicies(policies, func(domain.NodePolicy, error) {})
	var fullLength, kernelThread bool
	err = matchNodePolicies(ctx, tasks, compiled, func(task domain.TaskInfo, matches []*domain.NodePolicy) {
		task.Node = node
		for _, match := range matches {
			if match != policy {
				continue
			}
			result.MatchedTasks = append(result.MatchedTasks, task)
			fullLength = fullLength || len(task.Comm) == 15
			kernelThread = kernelThread || task.KernelThread
			if len(matches) > 1 {
				conflict := domain.NodePolicyConflict{Task: task, Winner: *matches[len(matches)-1]}
				for _, contender := range matches {
					conflict.Policies = append(conflict.Policies, *contender)
				}
				result.Conflicts = append(result.Conflicts, conflict)
			}
			break
		}
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(result.MatchedTasks, func(i, j int) bool { return result.MatchedTasks[i].TID < result.MatchedTasks[j].TID })
	sort.Slice(result.Conflicts, func(i, j int) bool { return result.Conflicts[i].Task.TID < result.Conflicts[j].Task.TID })
	if len(result.MatchedTasks) == 0 {
		warn("noMatches", "no current task matched commandRegex")
	}
	if fullLength || len(result.MatchedTasks) == 0 {
		warn("commLimit", "Linux comm is limited to 15 bytes and may be truncated; matches use comm, not the executable path or command line")
	}
	if kernelThread {
		warn("kernelThread", "commandRegex matches at least one kernel thread")
	}
	return result, nil
}
