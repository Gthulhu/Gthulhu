package service

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (svc *Service) CreateScheduleStrategy(ctx context.Context, operator *domain.Claims, strategy *domain.ScheduleStrategy) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}
	queryOpt := &domain.QueryPodsOptions{
		K8SNamespace:   strategy.K8sNamespace,
		LabelSelectors: strategy.LabelSelectors,
		CommandRegex:   strategy.CommandRegex,
	}
	pods, err := svc.K8SAdapter.QueryPods(ctx, queryOpt)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "no pods match the strategy criteria", fmt.Errorf("no pods found for the given namespaces and label selectors, opts:%+v", queryOpt))
	}

	logger.Logger(ctx).Debug().Msgf("found %d pods matching the strategy criteria", len(pods))

	strategy.BaseEntity = domain.NewBaseEntity(&operatorID, &operatorID)

	intents := make([]*domain.ScheduleIntent, 0, len(pods))
	nodeIDsMap := make(map[string]struct{})
	nodeIDs := make([]string, 0)
	for _, pod := range pods {
		intent := domain.NewScheduleIntent(strategy, pod)
		intents = append(intents, &intent)
		if _, exists := nodeIDsMap[pod.NodeID]; !exists {
			nodeIDsMap[pod.NodeID] = struct{}{}
			nodeIDs = append(nodeIDs, pod.NodeID)
		}
	}

	err = svc.Repo.InsertStrategyAndIntents(ctx, strategy, intents)
	if err != nil {
		return fmt.Errorf("insert strategy and intents into repository: %w", err)
	}

	dmLabel := domain.LabelSelector{
		Key:   "app",
		Value: "decisionmaker",
	}

	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabel,
		NodeIDs:            nodeIDs,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return err
	}
	if len(dms) == 0 {
		logger.Logger(ctx).Warn().Msgf("no decision maker pods found for scheduling intents, opts:%+v", dmQueryOpt)
		return nil
	}

	logger.Logger(ctx).Debug().Msgf("found %d decision maker pods for scheduling intents", len(dms))

	nodeIDIntentsMap := make(map[string][]*domain.ScheduleIntent)
	nodeIDIntentIDsMap := make(map[string][]bson.ObjectID)
	nodeIDDMap := make(map[string]*domain.DecisionMakerPod)
	for _, dmPod := range dms {
		for _, intent := range intents {
			if intent.NodeID == dmPod.NodeID {
				nodeIDIntentIDsMap[dmPod.Host] = append(nodeIDIntentIDsMap[dmPod.Host], intent.ID)
				nodeIDIntentsMap[dmPod.Host] = append(nodeIDIntentsMap[dmPod.Host], intent)
				nodeIDDMap[dmPod.Host] = dmPod
			}
		}
	}
	for host, intents := range nodeIDIntentsMap {
		dmPod := nodeIDDMap[host]
		err = svc.DMAdapter.SendSchedulingIntent(ctx, dmPod, intents)
		if err != nil {
			return fmt.Errorf("send scheduling intents to decision maker %s: %w", host, err)
		}
		err = svc.Repo.BatchUpdateIntentsState(ctx, nodeIDIntentIDsMap[host], domain.IntentStateSent)
		if err != nil {
			return fmt.Errorf("insert strategy and intents into repository: %w", err)
		}
		logger.Logger(ctx).Info().Msgf("sent %d scheduling intents to decision maker %s", len(intents), host)
	}
	return nil
}

func (svc *Service) ListScheduleStrategies(ctx context.Context, filterOpts *domain.QueryStrategyOptions) error {
	return svc.Repo.QueryStrategies(ctx, filterOpts)
}

func (svc *Service) ListScheduleIntents(ctx context.Context, filterOpts *domain.QueryIntentOptions) error {
	return svc.Repo.QueryIntents(ctx, filterOpts)
}

func (svc *Service) UpdateScheduleStrategy(ctx context.Context, operator *domain.Claims, strategyID string, strategy *domain.ScheduleStrategy) error {
	strategyObjID, err := bson.ObjectIDFromHex(strategyID)
	if err != nil {
		return errors.WithMessagef(err, "invalid strategy ID %s", strategyID)
	}

	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	// Validate ownership and load existing strategy
	queryOpt := &domain.QueryStrategyOptions{
		IDs:        []bson.ObjectID{strategyObjID},
		CreatorIDs: []bson.ObjectID{operatorID},
	}
	if err := svc.Repo.QueryStrategies(ctx, queryOpt); err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "strategy not found or you don't have permission to update it", nil)
	}
	currentStrategy := queryOpt.Result[0]

	// Query pods based on new strategy criteria before making changes
	queryPodsOpt := &domain.QueryPodsOptions{
		K8SNamespace:   strategy.K8sNamespace,
		LabelSelectors: strategy.LabelSelectors,
		CommandRegex:   strategy.CommandRegex,
	}
	pods, err := svc.K8SAdapter.QueryPods(ctx, queryPodsOpt)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "no pods match the strategy criteria", fmt.Errorf("no pods found for the given namespaces and label selectors, opts:%+v", queryPodsOpt))
	}

	// Load existing intents for DM cleanup
	oldIntentQuery := &domain.QueryIntentOptions{
		StrategyIDs: []bson.ObjectID{strategyObjID},
	}
	if err := svc.Repo.QueryIntents(ctx, oldIntentQuery); err != nil {
		return fmt.Errorf("query intents for strategy: %w", err)
	}

	// Update strategy document
	now := time.Now().UnixMilli()
	strategy.ID = strategyObjID
	strategy.CreatedTime = currentStrategy.CreatedTime
	strategy.CreatorID = currentStrategy.CreatorID
	strategy.UpdaterID = operatorID
	strategy.UpdatedTime = now

	if err := svc.Repo.UpdateStrategy(ctx, strategy); err != nil {
		return fmt.Errorf("update strategy: %w", err)
	}

	// Replace intents for the strategy
	if err := svc.Repo.DeleteIntentsByStrategyID(ctx, strategyObjID); err != nil {
		return fmt.Errorf("delete intents by strategy ID: %w", err)
	}

	intents := make([]*domain.ScheduleIntent, 0, len(pods))
	nodeIDsMap := make(map[string]struct{})
	nodeIDs := make([]string, 0)
	for _, pod := range pods {
		intent := domain.NewScheduleIntent(strategy, pod)
		intents = append(intents, &intent)
		if _, exists := nodeIDsMap[pod.NodeID]; !exists {
			nodeIDsMap[pod.NodeID] = struct{}{}
			nodeIDs = append(nodeIDs, pod.NodeID)
		}
	}

	if err := svc.Repo.InsertIntents(ctx, intents); err != nil {
		return fmt.Errorf("insert intents into repository: %w", err)
	}

	// Notify decision makers to remove old intents
	if len(oldIntentQuery.Result) > 0 {
		oldNodeIDsMap := make(map[string]struct{})
		oldPodIDsMap := make(map[string]struct{})
		for _, intent := range oldIntentQuery.Result {
			oldNodeIDsMap[intent.NodeID] = struct{}{}
			oldPodIDsMap[intent.PodID] = struct{}{}
		}
		oldNodeIDs := make([]string, 0, len(oldNodeIDsMap))
		for nodeID := range oldNodeIDsMap {
			oldNodeIDs = append(oldNodeIDs, nodeID)
		}
		oldPodIDs := make([]string, 0, len(oldPodIDsMap))
		for podID := range oldPodIDsMap {
			oldPodIDs = append(oldPodIDs, podID)
		}

		dmLabel := domain.LabelSelector{
			Key:   "app",
			Value: "decisionmaker",
		}
		dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
			DecisionMakerLabel: dmLabel,
			NodeIDs:            oldNodeIDs,
		}
		dmPods, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msg("failed to query decision maker pods for update deletion notification")
		} else if len(oldPodIDs) > 0 {
			deleteReq := &domain.DeleteIntentsRequest{PodIDs: oldPodIDs}
			for _, dmPod := range dmPods {
				if err := svc.DMAdapter.DeleteSchedulingIntents(ctx, dmPod, deleteReq); err != nil {
					logger.Logger(ctx).Warn().Err(err).Msgf("failed to notify decision maker %s to delete intents", dmPod.NodeID)
				}
			}
		}
	}

	// Send new intents to decision makers
	dmLabel := domain.LabelSelector{
		Key:   "app",
		Value: "decisionmaker",
	}
	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabel,
		NodeIDs:            nodeIDs,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return err
	}
	if len(dms) == 0 {
		logger.Logger(ctx).Warn().Msgf("no decision maker pods found for scheduling intents, opts:%+v", dmQueryOpt)
		return nil
	}

	nodeIDIntentsMap := make(map[string][]*domain.ScheduleIntent)
	nodeIDIntentIDsMap := make(map[string][]bson.ObjectID)
	nodeIDDMap := make(map[string]*domain.DecisionMakerPod)
	for _, dmPod := range dms {
		for _, intent := range intents {
			if intent.NodeID == dmPod.NodeID {
				nodeIDIntentIDsMap[dmPod.Host] = append(nodeIDIntentIDsMap[dmPod.Host], intent.ID)
				nodeIDIntentsMap[dmPod.Host] = append(nodeIDIntentsMap[dmPod.Host], intent)
				nodeIDDMap[dmPod.Host] = dmPod
			}
		}
	}
	for host, intents := range nodeIDIntentsMap {
		dmPod := nodeIDDMap[host]
		err = svc.DMAdapter.SendSchedulingIntent(ctx, dmPod, intents)
		if err != nil {
			return fmt.Errorf("send scheduling intents to decision maker %s: %w", host, err)
		}
		err = svc.Repo.BatchUpdateIntentsState(ctx, nodeIDIntentIDsMap[host], domain.IntentStateSent)
		if err != nil {
			return fmt.Errorf("update intents state: %w", err)
		}
		logger.Logger(ctx).Info().Msgf("sent %d scheduling intents to decision maker %s", len(intents), host)
	}

	logger.Logger(ctx).Info().Msgf("updated strategy %s and regenerated intents", strategyID)
	return nil
}

func (svc *Service) DeleteScheduleStrategy(ctx context.Context, operator *domain.Claims, strategyID string) error {
	strategyObjID, err := bson.ObjectIDFromHex(strategyID)
	if err != nil {
		return errors.WithMessagef(err, "invalid strategy ID %s", strategyID)
	}

	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	// Check if strategy exists and belongs to the operator
	queryOpt := &domain.QueryStrategyOptions{
		IDs:        []bson.ObjectID{strategyObjID},
		CreatorIDs: []bson.ObjectID{operatorID},
	}
	err = svc.Repo.QueryStrategies(ctx, queryOpt)
	if err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "strategy not found or you don't have permission to delete it", nil)
	}

	// Query intents associated with this strategy to get node IDs and pod IDs for DM notification
	intentQueryOpt := &domain.QueryIntentOptions{
		StrategyIDs: []bson.ObjectID{strategyObjID},
	}
	err = svc.Repo.QueryIntents(ctx, intentQueryOpt)
	if err != nil {
		return fmt.Errorf("query intents for strategy: %w", err)
	}

	// Collect unique node IDs and pod IDs from intents
	nodeIDsMap := make(map[string]struct{})
	podIDsMap := make(map[string]struct{})
	for _, intent := range intentQueryOpt.Result {
		nodeIDsMap[intent.NodeID] = struct{}{}
		podIDsMap[intent.PodID] = struct{}{}
	}
	nodeIDs := make([]string, 0, len(nodeIDsMap))
	for nodeID := range nodeIDsMap {
		nodeIDs = append(nodeIDs, nodeID)
	}
	podIDs := make([]string, 0, len(podIDsMap))
	for podID := range podIDsMap {
		podIDs = append(podIDs, podID)
	}

	// Delete associated intents first
	err = svc.Repo.DeleteIntentsByStrategyID(ctx, strategyObjID)
	if err != nil {
		return fmt.Errorf("delete intents by strategy ID: %w", err)
	}

	// Delete the strategy
	err = svc.Repo.DeleteStrategy(ctx, strategyObjID)
	if err != nil {
		return fmt.Errorf("delete strategy: %w", err)
	}

	// Notify decision makers to remove intents from their in-memory cache
	if len(nodeIDs) > 0 && len(podIDs) > 0 {
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
			logger.Logger(ctx).Warn().Err(err).Msg("failed to query decision maker pods for deletion notification")
		} else {
			deleteReq := &domain.DeleteIntentsRequest{
				PodIDs: podIDs,
			}
			for _, dmPod := range dmPods {
				if err := svc.DMAdapter.DeleteSchedulingIntents(ctx, dmPod, deleteReq); err != nil {
					logger.Logger(ctx).Warn().Err(err).Msgf("failed to notify decision maker %s to delete intents", dmPod.NodeID)
				}
			}
		}
	}

	logger.Logger(ctx).Info().Msgf("deleted strategy %s and its associated intents", strategyID)
	return nil
}

func (svc *Service) DeleteScheduleIntents(ctx context.Context, operator *domain.Claims, intentIDs []string) error {
	if len(intentIDs) == 0 {
		return nil
	}

	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	intentObjIDs := make([]bson.ObjectID, 0, len(intentIDs))
	for _, id := range intentIDs {
		objID, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return errors.WithMessagef(err, "invalid intent ID %s", id)
		}
		intentObjIDs = append(intentObjIDs, objID)
	}

	// Check if intents exist and belong to the operator
	queryOpt := &domain.QueryIntentOptions{
		IDs:        intentObjIDs,
		CreatorIDs: []bson.ObjectID{operatorID},
	}
	err = svc.Repo.QueryIntents(ctx, queryOpt)
	if err != nil {
		return err
	}

	// Verify that all requested intents exist, are returned by the query,
	// and are owned by the current operator.
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "one or more intents not found or you don't have permission to delete them", nil)
	}

	// Build a set of requested intent IDs for exact ID matching.
	requestedIDs := make(map[bson.ObjectID]struct{}, len(intentObjIDs))
	for _, id := range intentObjIDs {
		requestedIDs[id] = struct{}{}
	}

	matchedCount := 0
	for _, intent := range queryOpt.Result {
		// Ensure the intent belongs to the operator.
		if intent.CreatorID != operatorID {
			return errs.NewHTTPStatusError(http.StatusNotFound, "one or more intents not found or you don't have permission to delete them", nil)
		}

		// Ensure the intent is one of the requested IDs.
		if _, ok := requestedIDs[intent.ID]; ok {
			matchedCount++
		}
	}

	if matchedCount != len(intentObjIDs) {
		return errs.NewHTTPStatusError(http.StatusNotFound, "one or more intents not found or you don't have permission to delete them", nil)
	}

	// Collect unique node IDs and pod IDs for DM notification before deleting
	nodeIDsMap := make(map[string]struct{})
	podIDsMap := make(map[string]struct{})
	for _, intent := range queryOpt.Result {
		nodeIDsMap[intent.NodeID] = struct{}{}
		podIDsMap[intent.PodID] = struct{}{}
	}
	nodeIDs := make([]string, 0, len(nodeIDsMap))
	for nodeID := range nodeIDsMap {
		nodeIDs = append(nodeIDs, nodeID)
	}
	podIDs := make([]string, 0, len(podIDsMap))
	for podID := range podIDsMap {
		podIDs = append(podIDs, podID)
	}

	// Delete the intents
	err = svc.Repo.DeleteIntents(ctx, intentObjIDs)
	if err != nil {
		return fmt.Errorf("delete intents: %w", err)
	}

	// Notify decision makers to remove intents from their in-memory cache
	if len(nodeIDs) > 0 && len(podIDs) > 0 {
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
			logger.Logger(ctx).Warn().Err(err).Msg("failed to query decision maker pods for deletion notification")
		} else {
			deleteReq := &domain.DeleteIntentsRequest{
				PodIDs: podIDs,
			}
			for _, dmPod := range dmPods {
				if err := svc.DMAdapter.DeleteSchedulingIntents(ctx, dmPod, deleteReq); err != nil {
					logger.Logger(ctx).Warn().Err(err).Msgf("failed to notify decision maker %s to delete intents", dmPod.NodeID)
				}
			}
		}
	}

	logger.Logger(ctx).Info().Msgf("deleted %d intents", len(intentIDs))
	return nil
}

func (svc *Service) GetPodPIDMapping(ctx context.Context, nodeID string) (*domain.PodPIDMappingResponse, error) {
	if svc.K8SAdapter == nil {
		return nil, domain.ErrNoClient
	}

	dmLabel := domain.LabelSelector{
		Key:   "app",
		Value: "decisionmaker",
	}
	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabel,
		NodeIDs:            []string{nodeID},
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return nil, fmt.Errorf("query decision maker pods: %w", err)
	}
	if len(dms) == 0 {
		return nil, fmt.Errorf("no decision maker pod found on node %s", nodeID)
	}

	dm := dms[0]
	if dm.State != domain.NodeStateOnline {
		return nil, fmt.Errorf("decision maker on node %s is not online (state: %d)", nodeID, dm.State)
	}

	result, err := svc.DMAdapter.GetPodPIDMapping(ctx, dm)
	if err != nil {
		return nil, fmt.Errorf("get pod-pid mapping from decision maker: %w", err)
	}

	// Set NodeID in response if not already set
	if result.NodeID == "" {
		result.NodeID = nodeID
	}

	return result, nil
}

// ListNodes returns all nodes in the Kubernetes cluster
func (svc *Service) ListNodes(ctx context.Context) ([]*domain.Node, error) {
	if svc.K8SAdapter == nil {
		return nil, domain.ErrNoClient
	}

	return svc.K8SAdapter.ListNodes(ctx)
}
