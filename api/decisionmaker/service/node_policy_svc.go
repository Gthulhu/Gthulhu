package service

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/util"
)

// ProcessNodePolicies replaces the cached set of node-level scheduling
// policies and rebuilds the merkle root used to detect drift with the
// Manager. Unlike Pod-based intents, node policies are matched against the
// full process table of the node (see resolveNodeSchedulingIntents) rather
// than against a fixed PID discovered via cgroup/proc scanning.
func (svc *Service) ProcessNodePolicies(ctx context.Context, policies []*domain.NodePolicy) error {
	normalized := normalizeNodePolicyInputs(policies)
	sorted := sortNodePoliciesByKey(normalized)
	leafHashes := make([]string, 0, len(sorted))
	for _, policy := range sorted {
		leafHashes = append(leafHashes, hashNodePolicy(policy))
	}
	root := util.BuildMerkleTree(leafHashes)

	svc.nodePolicyCacheMu.Lock()
	svc.nodePolicyCache = normalized
	svc.nodePolicyMerkleRoot = root
	if root != nil {
		svc.nodePolicyMerkleRootHash = root.Hash
	} else {
		svc.nodePolicyMerkleRootHash = ""
	}
	svc.nodePolicyCacheMu.Unlock()

	logger.Logger(ctx).Info().Msgf("processed %d node scheduling policies", len(normalized))
	return nil
}

// resolveNodeSchedulingIntents snapshots every thread on the node (via
// svc.taskSource, normally /proc) and matches each thread's comm name against
// every active node policy's CommandRegex. Matches become domain.SchedulingIntents
// keyed by TID - the entity the scheduler actually runs. Policies are evaluated
// in the same canonical order used for the merkle root, and at most one intent
// is emitted per TID, so two decision makers holding the same policy set (hence
// the same root) resolve to the same result regardless of push order.
func (svc *Service) resolveNodeSchedulingIntents(ctx context.Context) ([]*domain.SchedulingIntents, error) {
	policies := svc.cachedNodePolicies()
	if len(policies) == 0 {
		return nil, nil
	}
	if svc.taskSource == nil {
		return nil, nil
	}

	tasks, err := svc.taskSource.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot tasks: %w", err)
	}

	compiled := compileNodePolicies(policies, func(policy domain.NodePolicy, err error) {
		logger.Logger(ctx).Warn().Err(err).Msgf("invalid commandRegex %q in node policy %s", policy.CommandRegex, policy.PolicyID)
	})
	var results []*domain.SchedulingIntents
	node := localNodeName()
	err = matchNodePolicies(ctx, tasks, compiled, func(task domain.TaskInfo, matches []*domain.NodePolicy) {
		policy := *matches[len(matches)-1]
		if len(matches) > 1 {
			logger.Logger(ctx).Warn().Msgf("overlapping node policies match tid %d (comm %q); policy %s wins (last in canonical order)", task.TID, task.Comm, policy.PolicyID)
		}
		task.Node = policyNode(policy, node)
		results = append(results, &domain.SchedulingIntents{
			Priority: policy.Priority, ExecutionTime: uint64(policy.ExecutionTime), PID: task.TID,
			CommandRegex:    policy.CommandRegex,
			NodePolicyMatch: &domain.NodePolicyMatch{Policy: policy, Task: task, IntendedKey: task.TID},
		})
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(results, func(i, j int) bool { return results[i].PID < results[j].PID })
	return results, nil
}

func (svc *Service) cachedNodePolicies() []*domain.NodePolicy {
	svc.nodePolicyCacheMu.RLock()
	defer svc.nodePolicyCacheMu.RUnlock()
	return svc.nodePolicyCache
}

type compiledNodePolicy struct {
	policy *domain.NodePolicy
	regex  *regexp.Regexp
}

func compileNodePolicies(policies []*domain.NodePolicy, invalid func(domain.NodePolicy, error)) []compiledNodePolicy {
	var compiled []compiledNodePolicy
	for _, policy := range sortNodePoliciesByKey(policies) {
		if policy.CommandRegex == "" {
			continue
		}
		re, err := regexp.Compile(policy.CommandRegex)
		if err != nil {
			invalid(*policy, err)
			continue
		}
		compiled = append(compiled, compiledNodePolicy{policy, re})
	}
	return compiled
}

func matchNodePolicies(ctx context.Context, tasks []domain.TaskInfo, policies []compiledNodePolicy, matched func(domain.TaskInfo, []*domain.NodePolicy)) error {
	matches := make([]*domain.NodePolicy, 0, len(policies))
	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return err
		}
		if task.Comm == pauseCommand {
			continue
		}
		matches = matches[:0]
		for _, policy := range policies {
			if policy.regex.MatchString(task.Comm) {
				matches = append(matches, policy.policy)
			}
		}
		if len(matches) > 0 {
			matched(task, matches)
		}
	}
	return nil
}

func localNodeName() string {
	if node := os.Getenv("NODE_NAME"); node != "" {
		return node
	}
	node, _ := os.Hostname()
	return node
}

func policyNode(policy domain.NodePolicy, fallback string) string {
	if policy.Node != "" {
		return policy.Node
	}
	return fallback
}

// DeleteNodePolicyByID removes a single node policy from the cache by ID.
func (svc *Service) DeleteNodePolicyByID(ctx context.Context, policyID string) error {
	svc.nodePolicyCacheMu.Lock()
	defer svc.nodePolicyCacheMu.Unlock()

	remaining := make([]*domain.NodePolicy, 0, len(svc.nodePolicyCache))
	for _, policy := range svc.nodePolicyCache {
		if policy != nil && policy.PolicyID == policyID {
			continue
		}
		remaining = append(remaining, policy)
	}
	svc.nodePolicyCache = remaining
	svc.rebuildNodePolicyMerkleRootLocked()
	logger.Logger(ctx).Info().Msgf("deleted node policy %s", policyID)
	return nil
}

// DeleteAllNodePolicies clears every cached node policy.
func (svc *Service) DeleteAllNodePolicies(ctx context.Context) error {
	svc.nodePolicyCacheMu.Lock()
	defer svc.nodePolicyCacheMu.Unlock()
	count := len(svc.nodePolicyCache)
	svc.nodePolicyCache = nil
	svc.rebuildNodePolicyMerkleRootLocked()
	logger.Logger(ctx).Info().Msgf("deleted all %d node policies", count)
	return nil
}

// rebuildNodePolicyMerkleRootLocked recomputes the merkle root from
// svc.nodePolicyCache. Callers must hold svc.nodePolicyCacheMu.
func (svc *Service) rebuildNodePolicyMerkleRootLocked() {
	sorted := sortNodePoliciesByKey(normalizeNodePolicyInputs(svc.nodePolicyCache))
	leafHashes := make([]string, 0, len(sorted))
	for _, policy := range sorted {
		leafHashes = append(leafHashes, hashNodePolicy(policy))
	}
	root := util.BuildMerkleTree(leafHashes)
	svc.nodePolicyMerkleRoot = root
	if root != nil {
		svc.nodePolicyMerkleRootHash = root.Hash
	} else {
		svc.nodePolicyMerkleRootHash = ""
	}
}

// GetNodePolicyMerkleRootHash returns the current merkle root hash of the
// cached node policies, used by the Manager to detect drift after a
// Decision Maker restart.
func (svc *Service) GetNodePolicyMerkleRootHash() string {
	svc.nodePolicyCacheMu.RLock()
	defer svc.nodePolicyCacheMu.RUnlock()
	return svc.nodePolicyMerkleRootHash
}

func normalizeNodePolicyInputs(policies []*domain.NodePolicy) []*domain.NodePolicy {
	results := make([]*domain.NodePolicy, 0, len(policies))
	for _, policy := range policies {
		if policy == nil {
			continue
		}
		results = append(results, policy)
	}
	return results
}

func sortNodePoliciesByKey(policies []*domain.NodePolicy) []*domain.NodePolicy {
	results := make([]*domain.NodePolicy, 0, len(policies))
	results = append(results, policies...)
	sort.Slice(results, func(i, j int) bool {
		if results[i].PolicyID != results[j].PolicyID {
			return results[i].PolicyID+"|" < results[j].PolicyID+"|"
		}
		if results[i].Generation != results[j].Generation {
			return results[i].Generation < results[j].Generation
		}
		return nodePolicySortKey(results[i]) < nodePolicySortKey(results[j])
	})
	return results
}

func nodePolicySortKey(policy *domain.NodePolicy) string {
	return strings.Join([]string{
		policy.PolicyID,
		policy.NodeID,
		policy.CommandRegex,
		strconv.Itoa(policy.Priority),
		strconv.FormatInt(policy.ExecutionTime, 10),
	}, "|")
}

func hashNodePolicy(policy *domain.NodePolicy) string {
	serialized := strings.Join([]string{
		"policyID=" + policy.PolicyID,
		"nodeID=" + policy.NodeID,
		"commandRegex=" + policy.CommandRegex,
		"priority=" + strconv.Itoa(policy.Priority),
		"executionTime=" + strconv.FormatInt(policy.ExecutionTime, 10),
	}, "|")
	if policy.Generation != 0 {
		serialized += "|generation=" + strconv.FormatInt(policy.Generation, 10)
	}
	return util.HashStringSHA256Hex(serialized)
}
