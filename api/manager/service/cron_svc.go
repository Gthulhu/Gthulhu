package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/util"
	"go.mongodb.org/mongo-driver/v2/bson"
)

// ReconcileIntents performs a full reconciliation of scheduling intents.
// It handles three scenarios:
//  1. Manager restart: re-sends all intents from DB to DM pods
//  2. Decision Maker restart: detects Merkle root mismatch and re-sends intents
//  3. Pod restart: detects stale intents (pods that no longer exist) and refreshes them
func (svc *Service) ReconcileIntents(ctx context.Context) error {
	if svc.K8SAdapter == nil {
		return domain.ErrNoClient
	}

	// Step 1: Refresh stale intents (handle pod restarts)
	if err := svc.refreshStaleIntents(ctx); err != nil {
		logger.Logger(ctx).Warn().Err(err).Msg("failed to refresh stale intents during reconciliation")
	}

	// Step 2: Re-send intents to DM pods where Merkle root doesn't match
	if err := svc.resyncIntentsToDMs(ctx); err != nil {
		return err
	}

	// Step 3: Re-apply persisted node runtime configs to online DM pods
	if err := svc.resyncRuntimeConfigsToDMs(ctx); err != nil {
		logger.Logger(ctx).Warn().Err(err).Msg("failed to resync runtime configs during reconciliation")
	}

	// Step 4: Refresh node scheduling intents (node label/DRA drift, nodes added/removed)
	if err := svc.refreshStaleNodeIntents(ctx); err != nil {
		logger.Logger(ctx).Warn().Err(err).Msg("failed to refresh stale node intents during reconciliation")
	}

	// Step 5: Re-send node intents to DM pods where Merkle root doesn't match
	if err := svc.resyncNodeIntentsToDMs(ctx); err != nil {
		logger.Logger(ctx).Warn().Err(err).Msg("failed to resync node intents during reconciliation")
	}
	return nil
}

func (svc *Service) resyncRuntimeConfigsToDMs(ctx context.Context) error {
	repo, ok := svc.Repo.(runtimeConfigRepository)
	if !ok {
		return nil
	}
	dmAdapter, ok := svc.DMAdapter.(runtimeConfigDMAdapter)
	if !ok {
		return nil
	}
	queryOpt := &domain.QueryNodeRuntimeConfigOptions{}
	if err := repo.QueryNodeRuntimeConfigs(ctx, queryOpt); err != nil {
		return fmt.Errorf("query node runtime configs: %w", err)
	}
	if len(queryOpt.Result) == 0 {
		return nil
	}

	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: domain.LabelSelector{Key: "app", Value: "decisionmaker"},
	})
	if err != nil {
		return fmt.Errorf("query decision maker pods: %w", err)
	}
	dmByNode := make(map[string]*domain.DecisionMakerPod, len(dms))
	for _, dm := range dms {
		dmByNode[dm.NodeID] = dm
	}

	for _, desired := range queryOpt.Result {
		dm := dmByNode[desired.NodeID]
		if dm == nil || dm.State != domain.NodeStateOnline {
			continue
		}
		result := domain.RuntimeConfigApplyResult{NodeID: dm.NodeID, Host: dm.Host, ConfigVersion: desired.ConfigVersion}
		if err := dmAdapter.ApplyRuntimeConfig(ctx, dm, desired.Config); err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
		}
		desired.LastApplyResult = result
		if err := repo.UpsertNodeRuntimeConfig(ctx, desired); err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to update runtime config apply result for node %s", desired.NodeID)
		}
	}
	return nil
}

// refreshStaleIntents checks all strategies for pods that no longer exist
// and creates new intents for replacement pods.
func (svc *Service) refreshStaleIntents(ctx context.Context) error {
	if svc.Repo == nil {
		return fmt.Errorf("repository is nil")
	}

	strategyOpt := &domain.QueryStrategyOptions{}
	if err := svc.Repo.QueryStrategies(ctx, strategyOpt); err != nil {
		return fmt.Errorf("query strategies: %w", err)
	}

	for _, strategy := range strategyOpt.Result {
		queryOpt := &domain.QueryPodsOptions{
			K8SNamespace:   strategy.K8sNamespace,
			LabelSelectors: strategy.LabelSelectors,
		}
		currentPods, err := svc.K8SAdapter.QueryPods(ctx, queryOpt)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to query pods for strategy %s", strategy.ID.Hex())
			continue
		}

		intentOpt := &domain.QueryIntentOptions{
			StrategyIDs: []bson.ObjectID{strategy.ID},
		}
		if err := svc.Repo.QueryIntents(ctx, intentOpt); err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to query intents for strategy %s", strategy.ID.Hex())
			continue
		}

		currentPodIDs := make(map[string]*domain.Pod, len(currentPods))
		for _, pod := range currentPods {
			currentPodIDs[pod.PodID] = pod
		}
		existingIntentPodIDs := make(map[string]*domain.ScheduleIntent, len(intentOpt.Result))
		for _, intent := range intentOpt.Result {
			existingIntentPodIDs[intent.PodID] = intent
		}

		// Delete stale intents (pod no longer exists in K8S)
		staleIntentIDs := make([]bson.ObjectID, 0)
		stalePodIDs := make([]string, 0)
		staleNodeIDsMap := make(map[string]struct{})
		for _, intent := range intentOpt.Result {
			if _, exists := currentPodIDs[intent.PodID]; !exists {
				staleIntentIDs = append(staleIntentIDs, intent.ID)
				stalePodIDs = append(stalePodIDs, intent.PodID)
				staleNodeIDsMap[intent.NodeID] = struct{}{}
			}
		}
		if len(staleIntentIDs) > 0 {
			if err := svc.Repo.DeleteIntents(ctx, staleIntentIDs); err != nil {
				logger.Logger(ctx).Warn().Err(err).Msgf("failed to delete stale intents for strategy %s", strategy.ID.Hex())
			} else {
				logger.Logger(ctx).Info().Msgf("deleted %d stale intents for strategy %s (stale pods: %v)", len(staleIntentIDs), strategy.ID.Hex(), stalePodIDs)
			}

			// Notify decision makers to remove stale pod intents from their in-memory cache
			svc.notifyDMsDeleteIntents(ctx, staleNodeIDsMap, stalePodIDs)
		}

		// Create new intents for pods that don't have intents yet
		newIntents := make([]*domain.ScheduleIntent, 0)
		for _, pod := range currentPods {
			if _, exists := existingIntentPodIDs[pod.PodID]; !exists {
				intent := domain.NewScheduleIntent(strategy, pod)
				newIntents = append(newIntents, &intent)
			}
		}
		if len(newIntents) > 0 {
			if err := svc.Repo.InsertIntents(ctx, newIntents); err != nil {
				logger.Logger(ctx).Warn().Err(err).Msgf("failed to insert new intents for strategy %s", strategy.ID.Hex())
			} else {
				logger.Logger(ctx).Info().Msgf("created %d new intents for strategy %s", len(newIntents), strategy.ID.Hex())
			}
		}
	}
	return nil
}

// resyncIntentsToDMs compares Merkle roots between Manager DB and each DM pod.
// When a mismatch is detected (e.g. DM restarted and lost in-memory intents),
// all intents for that node are re-sent.
func (svc *Service) resyncIntentsToDMs(ctx context.Context) error {
	dmLabel := domain.LabelSelector{
		Key:   "app",
		Value: "decisionmaker",
	}
	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabel,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return err
	}
	if len(dms) == 0 {
		logger.Logger(ctx).Warn().Msg("no decision maker pods found for intent reconciliation")
		return nil
	}

	queryOpt := &domain.QueryIntentOptions{}
	if err := svc.Repo.QueryIntents(ctx, queryOpt); err != nil {
		return err
	}

	expectedRootsByNode := buildExpectedIntentRootsByNode(queryOpt.Result)
	emptyRootHash := util.BuildMerkleTree(nil).Hash

	// Group intents by NodeID
	intentsPerNode := make(map[string][]*domain.ScheduleIntent)
	intentIDsPerNode := make(map[string][]bson.ObjectID)
	for _, intent := range queryOpt.Result {
		intentsPerNode[intent.NodeID] = append(intentsPerNode[intent.NodeID], intent)
		intentIDsPerNode[intent.NodeID] = append(intentIDsPerNode[intent.NodeID], intent.ID)
	}

	for _, dm := range dms {
		if dm.State != domain.NodeStateOnline {
			continue
		}
		if svc.DMAdapter == nil {
			return fmt.Errorf("decision maker adapter is nil")
		}
		rootHash, err := svc.DMAdapter.GetIntentMerkleRoot(ctx, dm)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to get merkle root from dm %s", dm)
			continue
		}
		expectedRoot := expectedRootsByNode[dm.NodeID]
		if expectedRoot == "" {
			expectedRoot = emptyRootHash
		}
		if rootHash == expectedRoot {
			continue
		}

		logger.Logger(ctx).Warn().Msgf("intent merkle mismatch for dm %s: expected=%s actual=%s, re-sending intents", dm, expectedRoot, rootHash)

		nodeIntents := intentsPerNode[dm.NodeID]
		if len(nodeIntents) == 0 {
			// No intents remain for this node, but DM still has stale data → tell it to clear everything
			deleteReq := &domain.DeleteIntentsRequest{All: true}
			if err := svc.DMAdapter.DeleteSchedulingIntents(ctx, dm, deleteReq); err != nil {
				logger.Logger(ctx).Warn().Err(err).Msgf("failed to notify dm %s to clear all intents", dm)
			} else {
				logger.Logger(ctx).Info().Msgf("notified dm %s to clear all intents (no intents remain)", dm)
			}
			continue
		}
		err = svc.DMAdapter.SendSchedulingIntent(ctx, dm, nodeIntents)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to re-send intents to dm %s", dm)
			continue
		}
		err = svc.Repo.BatchUpdateIntentsState(ctx, intentIDsPerNode[dm.NodeID], domain.IntentStateSent)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to update intent states for dm %s", dm)
		}
		logger.Logger(ctx).Info().Msgf("re-sent %d intents to dm %s", len(nodeIntents), dm)
	}
	return nil
}

// notifyDMsDeleteIntents notifies the decision maker pods on the given nodes
// to remove the specified pod intents from their in-memory cache.
func (svc *Service) notifyDMsDeleteIntents(ctx context.Context, nodeIDsMap map[string]struct{}, podIDs []string) {
	if len(nodeIDsMap) == 0 || len(podIDs) == 0 {
		return
	}
	if svc.DMAdapter == nil || svc.K8SAdapter == nil {
		return
	}

	nodeIDs := make([]string, 0, len(nodeIDsMap))
	for nodeID := range nodeIDsMap {
		nodeIDs = append(nodeIDs, nodeID)
	}

	dmLabel := domain.LabelSelector{
		Key:   "app",
		Value: "decisionmaker",
	}
	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabel,
		NodeIDs:            nodeIDs,
	}
	dmPods, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		logger.Logger(ctx).Warn().Err(err).Msg("failed to query decision maker pods for stale intent deletion notification")
		return
	}

	deleteReq := &domain.DeleteIntentsRequest{
		PodIDs: podIDs,
	}
	for _, dmPod := range dmPods {
		if dmPod.State != domain.NodeStateOnline {
			continue
		}
		if err := svc.DMAdapter.DeleteSchedulingIntents(ctx, dmPod, deleteReq); err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to notify dm %s to delete stale intents for pods %v", dmPod.NodeID, podIDs)
		} else {
			logger.Logger(ctx).Info().Msgf("notified dm %s to delete intents for stale pods %v", dmPod.NodeID, podIDs)
		}
	}
}

// CheckDMIntents is kept for backwards compatibility. It delegates to ReconcileIntents.
func (svc *Service) CheckDMIntents(ctx context.Context) error {
	return svc.ReconcileIntents(ctx)
}

// refreshStaleNodeIntents re-resolves matched nodes for every persisted
// NodeSchedulingPolicy and reconciles the intents: nodes that no longer
// match (e.g. label/DRA drift, node removed) have their intent deleted and
// the DM notified; newly matching nodes get a fresh intent inserted and
// sent to their DM.
func (svc *Service) refreshStaleNodeIntents(ctx context.Context) error {
	if svc.Repo == nil {
		return fmt.Errorf("repository is nil")
	}

	policyOpt := &domain.QueryNodePolicyOptions{}
	if err := svc.Repo.QueryNodePolicies(ctx, policyOpt); err != nil {
		return fmt.Errorf("query node policies: %w", err)
	}

	for _, policy := range policyOpt.Result {
		currentNodes, err := svc.resolveNodesForPolicy(ctx, policy)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to resolve nodes for node policy %s", policy.ID.Hex())
			continue
		}

		intentOpt := &domain.QueryNodeIntentOptions{
			PolicyIDs: []bson.ObjectID{policy.ID},
		}
		if err := svc.Repo.QueryNodeIntents(ctx, intentOpt); err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to query node intents for policy %s", policy.ID.Hex())
			continue
		}

		currentNodeNames := make(map[string]*domain.Node, len(currentNodes))
		for _, node := range currentNodes {
			currentNodeNames[node.Name] = node
		}
		existingIntentNodeIDs := make(map[string]*domain.NodeSchedulingIntent, len(intentOpt.Result))
		for _, intent := range intentOpt.Result {
			existingIntentNodeIDs[intent.NodeID] = intent
		}

		// Delete stale intents (node no longer matches the policy)
		staleIntentIDs := make([]bson.ObjectID, 0)
		staleIntents := make([]*domain.NodeSchedulingIntent, 0)
		for _, intent := range intentOpt.Result {
			if _, exists := currentNodeNames[intent.NodeID]; !exists {
				staleIntentIDs = append(staleIntentIDs, intent.ID)
				staleIntents = append(staleIntents, intent)
			}
		}
		if len(staleIntentIDs) > 0 {
			if err := svc.Repo.DeleteNodeIntents(ctx, staleIntentIDs); err != nil {
				logger.Logger(ctx).Warn().Err(err).Msgf("failed to delete stale node intents for policy %s", policy.ID.Hex())
			} else {
				logger.Logger(ctx).Info().Msgf("deleted %d stale node intents for policy %s", len(staleIntentIDs), policy.ID.Hex())
			}
			svc.notifyDMsDeleteNodeIntents(ctx, staleIntents)
		}

		// Create new intents for nodes that don't have intents yet
		newIntents := make([]*domain.NodeSchedulingIntent, 0)
		for _, node := range currentNodes {
			if _, exists := existingIntentNodeIDs[node.Name]; !exists {
				intent := domain.NewNodeSchedulingIntent(policy, node)
				newIntents = append(newIntents, &intent)
			}
		}
		if len(newIntents) > 0 {
			if err := svc.Repo.InsertNodeIntents(ctx, newIntents); err != nil {
				logger.Logger(ctx).Warn().Err(err).Msgf("failed to insert new node intents for policy %s", policy.ID.Hex())
				continue
			}
			logger.Logger(ctx).Info().Msgf("created %d new node intents for policy %s", len(newIntents), policy.ID.Hex())
			if err := svc.sendNodeIntentsToDMs(ctx, newIntents); err != nil {
				logger.Logger(ctx).Warn().Err(err).Msgf("failed to send new node intents for policy %s", policy.ID.Hex())
			}
		}
	}
	return nil
}

// resyncNodeIntentsToDMs compares Merkle roots between Manager DB and each
// DM pod's node-policy cache. When a mismatch is detected (e.g. DM restarted
// and lost its in-memory node policy cache), all node intents for that node
// are re-sent.
func (svc *Service) resyncNodeIntentsToDMs(ctx context.Context) error {
	if svc.Repo == nil || svc.K8SAdapter == nil {
		return nil
	}

	queryOpt := &domain.QueryNodeIntentOptions{}
	if err := svc.Repo.QueryNodeIntents(ctx, queryOpt); err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		// Nothing to reconcile; avoid an unnecessary DM pod lookup.
		return nil
	}

	dmLabel := domain.LabelSelector{
		Key:   "app",
		Value: "decisionmaker",
	}
	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabel,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return err
	}
	if len(dms) == 0 {
		logger.Logger(ctx).Warn().Msg("no decision maker pods found for node intent reconciliation")
		return nil
	}

	expectedRootsByNode := buildExpectedNodeIntentRootsByNode(queryOpt.Result)
	emptyRootHash := util.BuildMerkleTree(nil).Hash

	intentsPerNode := make(map[string][]*domain.NodeSchedulingIntent)
	intentIDsPerNode := make(map[string][]bson.ObjectID)
	for _, intent := range queryOpt.Result {
		intentsPerNode[intent.NodeID] = append(intentsPerNode[intent.NodeID], intent)
		intentIDsPerNode[intent.NodeID] = append(intentIDsPerNode[intent.NodeID], intent.ID)
	}

	for _, dm := range dms {
		if dm.State != domain.NodeStateOnline {
			continue
		}
		if svc.DMAdapter == nil {
			return fmt.Errorf("decision maker adapter is nil")
		}
		rootHash, err := svc.DMAdapter.GetNodePolicyMerkleRoot(ctx, dm)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to get node policy merkle root from dm %s", dm)
			continue
		}
		expectedRoot := expectedRootsByNode[dm.NodeID]
		if expectedRoot == "" {
			expectedRoot = emptyRootHash
		}
		if rootHash == expectedRoot {
			continue
		}

		logger.Logger(ctx).Warn().Msgf("node intent merkle mismatch for dm %s: expected=%s actual=%s, re-sending node intents", dm, expectedRoot, rootHash)

		nodeIntents := intentsPerNode[dm.NodeID]
		if len(nodeIntents) == 0 {
			deleteReq := &domain.DeleteNodeIntentsRequest{All: true}
			if err := svc.DMAdapter.DeleteNodeSchedulingIntents(ctx, dm, deleteReq); err != nil {
				logger.Logger(ctx).Warn().Err(err).Msgf("failed to notify dm %s to clear all node intents", dm)
			} else {
				logger.Logger(ctx).Info().Msgf("notified dm %s to clear all node intents (no node intents remain)", dm)
			}
			continue
		}
		if err := svc.DMAdapter.SendNodeSchedulingPolicies(ctx, dm, nodeIntents); err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to re-send node intents to dm %s", dm)
			continue
		}
		if err := svc.Repo.BatchUpdateNodeIntentsState(ctx, intentIDsPerNode[dm.NodeID], domain.IntentStateSent); err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to update node intent states for dm %s", dm)
		}
		logger.Logger(ctx).Info().Msgf("re-sent %d node intents to dm %s", len(nodeIntents), dm)
	}
	return nil
}

func sortNodeIntentsByKey(intents []*domain.NodeSchedulingIntent) []*domain.NodeSchedulingIntent {
	results := make([]*domain.NodeSchedulingIntent, 0, len(intents))
	for _, intent := range intents {
		if intent != nil {
			results = append(results, intent)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return nodeIntentSortKey(results[i]) < nodeIntentSortKey(results[j])
	})
	return results
}

func nodeIntentSortKey(intent *domain.NodeSchedulingIntent) string {
	return strings.Join([]string{
		intent.NodeID,
		intent.PolicyID.Hex(),
		intent.CommandRegex,
		strconv.Itoa(intent.Priority),
		strconv.FormatInt(intent.ExecutionTime, 10),
	}, "|")
}

func hashNodeSchedulingIntent(intent *domain.NodeSchedulingIntent) string {
	serialized := strings.Join([]string{
		"nodeID=" + intent.NodeID,
		"policyID=" + intent.PolicyID.Hex(),
		"commandRegex=" + intent.CommandRegex,
		"priority=" + strconv.Itoa(intent.Priority),
		"executionTime=" + strconv.FormatInt(intent.ExecutionTime, 10),
	}, "|")
	return util.HashStringSHA256Hex(serialized)
}

func buildExpectedNodeIntentRootsByNode(intents []*domain.NodeSchedulingIntent) map[string]string {
	byNode := make(map[string][]*domain.NodeSchedulingIntent)
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		byNode[intent.NodeID] = append(byNode[intent.NodeID], intent)
	}
	roots := make(map[string]string, len(byNode))
	for nodeID, nodeIntents := range byNode {
		roots[nodeID] = buildNodeSchedulingIntentMerkleRoot(nodeIntents)
	}
	return roots
}

func buildNodeSchedulingIntentMerkleRoot(intents []*domain.NodeSchedulingIntent) string {
	leafHashes := make([]string, 0, len(intents))
	sortedIntents := sortNodeIntentsByKey(intents)
	for _, intent := range sortedIntents {
		leafHashes = append(leafHashes, hashNodeSchedulingIntent(intent))
	}
	return util.BuildMerkleTree(leafHashes).Hash
}

func sortScheduleIntentsByKey(intents []*domain.ScheduleIntent) []*domain.ScheduleIntent {
	normalized := normalizeScheduleIntents(intents)
	results := make([]*domain.ScheduleIntent, 0, len(normalized))
	results = append(results, normalized...)
	sort.Slice(results, func(i, j int) bool {
		return scheduleIntentSortKey(results[i]) < scheduleIntentSortKey(results[j])
	})
	return results
}

func normalizeScheduleIntents(intents []*domain.ScheduleIntent) []*domain.ScheduleIntent {
	results := make([]*domain.ScheduleIntent, 0, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		results = append(results, intent)
	}
	return results
}

func scheduleIntentSortKey(intent *domain.ScheduleIntent) string {
	labels := make([]string, 0, len(intent.PodLabels))
	for key, value := range intent.PodLabels {
		labels = append(labels, key+"="+value)
	}
	sort.Strings(labels)
	return strings.Join([]string{
		intent.PodName,
		intent.PodID,
		intent.NodeID,
		intent.K8sNamespace,
		intent.CommandRegex,
		strconv.Itoa(intent.Priority),
		strconv.FormatInt(intent.ExecutionTime, 10),
		strings.Join(labels, ","),
	}, "|")
}

func hashScheduleIntent(intent *domain.ScheduleIntent) string {
	labels := make([]string, 0, len(intent.PodLabels))
	for key, value := range intent.PodLabels {
		labels = append(labels, key+"="+value)
	}
	sort.Strings(labels)
	serialized := strings.Join([]string{
		"podName=" + intent.PodName,
		"podID=" + intent.PodID,
		"nodeID=" + intent.NodeID,
		"k8sNamespace=" + intent.K8sNamespace,
		"commandRegex=" + intent.CommandRegex,
		"priority=" + strconv.Itoa(intent.Priority),
		"executionTime=" + strconv.FormatInt(intent.ExecutionTime, 10),
		"podLabels=" + strings.Join(labels, ","),
	}, "|")
	return util.HashStringSHA256Hex(serialized)
}

func buildExpectedIntentRootsByNode(intents []*domain.ScheduleIntent) map[string]string {
	byNode := make(map[string][]*domain.ScheduleIntent)
	for _, intent := range normalizeScheduleIntents(intents) {
		byNode[intent.NodeID] = append(byNode[intent.NodeID], intent)
	}
	roots := make(map[string]string, len(byNode))
	for nodeID, nodeIntents := range byNode {
		roots[nodeID] = buildScheduleIntentMerkleRoot(nodeIntents)
	}
	return roots
}

func buildScheduleIntentMerkleRoot(intents []*domain.ScheduleIntent) string {
	leafHashes := make([]string, 0, len(intents))
	sortedIntents := sortScheduleIntentsByKey(intents)
	for _, intent := range sortedIntents {
		leafHashes = append(leafHashes, hashScheduleIntent(intent))
	}
	return util.BuildMerkleTree(leafHashes).Hash
}
