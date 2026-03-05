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
	return svc.resyncIntentsToDMs(ctx)
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
			CommandRegex:   strategy.CommandRegex,
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
