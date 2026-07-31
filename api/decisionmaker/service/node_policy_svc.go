package service

import (
	"context"
	"fmt"
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

// resolveNodeSchedulingIntents takes a snapshot of every running process on
// the node (via svc.processSource, normally /proc, or the eBPF process
// monitor when available) and matches each process' comm name against every
// active node policy's CommandRegex. Matches are converted into the same
// domain.SchedulingIntents struct already consumed by the scheduler daemon,
// so no changes are required downstream of GET /api/v1/scheduling/strategies.
func (svc *Service) resolveNodeSchedulingIntents(ctx context.Context) ([]*domain.SchedulingIntents, error) {
	svc.nodePolicyCacheMu.RLock()
	policies := svc.nodePolicyCache
	svc.nodePolicyCacheMu.RUnlock()

	if len(policies) == 0 {
		return nil, nil
	}
	if svc.processSource == nil {
		return nil, nil
	}

	processes, err := svc.processSource.Snapshot(ctx)
	if err != nil {
		return nil, fmt.Errorf("snapshot processes: %w", err)
	}

	var results []*domain.SchedulingIntents
	for _, policy := range policies {
		if policy == nil || policy.CommandRegex == "" {
			continue
		}
		re, err := regexp.Compile(policy.CommandRegex)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("invalid commandRegex %q in node policy %s", policy.CommandRegex, policy.PolicyID)
			continue
		}
		for pid, comm := range processes {
			if comm == pauseCommand {
				continue
			}
			if !re.MatchString(comm) {
				continue
			}
			results = append(results, &domain.SchedulingIntents{
				Priority:      policy.Priority,
				ExecutionTime: uint64(policy.ExecutionTime),
				PID:           pid,
				CommandRegex:  policy.CommandRegex,
			})
		}
	}
	return results, nil
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
	return util.HashStringSHA256Hex(serialized)
}
