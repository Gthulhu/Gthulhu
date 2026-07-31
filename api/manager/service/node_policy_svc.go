package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/v2/bson"
)

var dmLabelSelector = domain.LabelSelector{Key: "app", Value: "decisionmaker"}

// resolveNodesForPolicy returns the set of nodes that match every non-empty
// selector type on the policy (AND semantics across selector types). When no
// selector is set at all, no nodes match (an empty policy should not
// silently apply everywhere).
func (svc *Service) resolveNodesForPolicy(ctx context.Context, policy *domain.NodeSchedulingPolicy) ([]*domain.Node, error) {
	if len(policy.NodeSelectors) == 0 && len(policy.NodeNames) == 0 && len(policy.DRASelectors) == 0 {
		return nil, nil
	}

	var candidates map[string]*domain.Node
	haveCandidates := false

	if len(policy.NodeSelectors) > 0 || len(policy.NodeNames) > 0 {
		nodes, err := svc.K8SAdapter.QueryNodesBySelectors(ctx, &domain.QueryNodesOptions{
			NodeSelectors: policy.NodeSelectors,
			NodeNames:     policy.NodeNames,
		})
		if err != nil {
			return nil, fmt.Errorf("query nodes by selectors: %w", err)
		}
		candidates = intersectNodes(candidates, nodes, haveCandidates)
		haveCandidates = true
	}

	if len(policy.DRASelectors) > 0 {
		nodes, err := svc.K8SAdapter.QueryNodesByDRA(ctx, &domain.QueryNodesByDRAOptions{
			DRASelectors: policy.DRASelectors,
		})
		if err != nil {
			return nil, fmt.Errorf("query nodes by DRA: %w", err)
		}
		candidates = intersectNodes(candidates, nodes, haveCandidates)
		haveCandidates = true
	}

	results := make([]*domain.Node, 0, len(candidates))
	for _, node := range candidates {
		results = append(results, node)
	}
	return results, nil
}

// intersectNodes intersects the running candidate set with a newly queried
// node list. On the first call (haveCandidates == false) it simply seeds the
// candidate set.
func intersectNodes(candidates map[string]*domain.Node, nodes []*domain.Node, haveCandidates bool) map[string]*domain.Node {
	byName := make(map[string]*domain.Node, len(nodes))
	for _, node := range nodes {
		byName[node.Name] = node
	}
	if !haveCandidates {
		return byName
	}
	result := make(map[string]*domain.Node)
	for name, node := range candidates {
		if _, ok := byName[name]; ok {
			result[name] = node
		}
	}
	return result
}

// CreateNodeSchedulingPolicy resolves the set of matching nodes for the
// policy, persists it alongside one NodeSchedulingIntent per matched node,
// and pushes the intents to each matched node's Decision Maker.
func (svc *Service) CreateNodeSchedulingPolicy(ctx context.Context, operator *domain.Claims, policy *domain.NodeSchedulingPolicy) error {
	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	nodes, err := svc.resolveNodesForPolicy(ctx, policy)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "no nodes match the policy criteria", fmt.Errorf("no nodes found for the given selectors"))
	}

	policy.BaseEntity = domain.NewBaseEntity(&operatorID, &operatorID)

	intents := make([]*domain.NodeSchedulingIntent, 0, len(nodes))
	for _, node := range nodes {
		intent := domain.NewNodeSchedulingIntent(policy, node)
		intents = append(intents, &intent)
	}

	if err := svc.Repo.InsertNodePolicyAndIntents(ctx, policy, intents); err != nil {
		return fmt.Errorf("insert node policy and intents into repository: %w", err)
	}

	return svc.sendNodeIntentsToDMs(ctx, intents)
}

// sendNodeIntentsToDMs groups the given intents by their matched node's
// Decision Maker pod and dispatches them, marking each intent as sent.
func (svc *Service) sendNodeIntentsToDMs(ctx context.Context, intents []*domain.NodeSchedulingIntent) error {
	if len(intents) == 0 {
		return nil
	}

	nodeIDsMap := make(map[string]struct{})
	nodeIDs := make([]string, 0)
	for _, intent := range intents {
		if _, exists := nodeIDsMap[intent.NodeID]; !exists {
			nodeIDsMap[intent.NodeID] = struct{}{}
			nodeIDs = append(nodeIDs, intent.NodeID)
		}
	}

	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabelSelector,
		NodeIDs:            nodeIDs,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return err
	}
	if len(dms) == 0 {
		logger.Logger(ctx).Warn().Msgf("no decision maker pods found for node scheduling intents, opts:%+v", dmQueryOpt)
		return nil
	}

	nodeIDIntentsMap := make(map[string][]*domain.NodeSchedulingIntent)
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
	for host, nodeIntents := range nodeIDIntentsMap {
		dmPod := nodeIDDMap[host]
		if err := svc.DMAdapter.SendNodeSchedulingPolicies(ctx, dmPod, nodeIntents); err != nil {
			return fmt.Errorf("send node scheduling intents to decision maker %s: %w", host, err)
		}
		if err := svc.Repo.BatchUpdateNodeIntentsState(ctx, nodeIDIntentIDsMap[host], domain.IntentStateSent); err != nil {
			return fmt.Errorf("update node intents state: %w", err)
		}
		logger.Logger(ctx).Info().Msgf("sent %d node scheduling intents to decision maker %s", len(nodeIntents), host)
	}
	return nil
}

func (svc *Service) ListNodeSchedulingPolicies(ctx context.Context, filterOpts *domain.QueryNodePolicyOptions) error {
	return svc.Repo.QueryNodePolicies(ctx, filterOpts)
}

func (svc *Service) ListNodeSchedulingIntents(ctx context.Context, filterOpts *domain.QueryNodeIntentOptions) error {
	return svc.Repo.QueryNodeIntents(ctx, filterOpts)
}

// UpdateNodeSchedulingPolicy re-resolves matched nodes for the policy and
// replaces the intents accordingly, notifying decision makers of both the
// removed and newly added intents.
func (svc *Service) UpdateNodeSchedulingPolicy(ctx context.Context, operator *domain.Claims, policyID string, policy *domain.NodeSchedulingPolicy) error {
	policyObjID, err := bson.ObjectIDFromHex(policyID)
	if err != nil {
		return errors.WithMessagef(err, "invalid node policy ID %s", policyID)
	}

	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	queryOpt := &domain.QueryNodePolicyOptions{
		IDs:        []bson.ObjectID{policyObjID},
		CreatorIDs: []bson.ObjectID{operatorID},
	}
	if err := svc.Repo.QueryNodePolicies(ctx, queryOpt); err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "node policy not found or you don't have permission to update it", nil)
	}
	currentPolicy := queryOpt.Result[0]

	nodes, err := svc.resolveNodesForPolicy(ctx, policy)
	if err != nil {
		return err
	}
	if len(nodes) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "no nodes match the policy criteria", fmt.Errorf("no nodes found for the given selectors"))
	}

	oldIntentQuery := &domain.QueryNodeIntentOptions{
		PolicyIDs: []bson.ObjectID{policyObjID},
	}
	if err := svc.Repo.QueryNodeIntents(ctx, oldIntentQuery); err != nil {
		return fmt.Errorf("query node intents for policy: %w", err)
	}

	policy.ID = policyObjID
	policy.CreatedTime = currentPolicy.CreatedTime
	policy.CreatorID = currentPolicy.CreatorID
	policy.UpdaterID = operatorID

	if err := svc.Repo.UpdateNodePolicy(ctx, policy); err != nil {
		return fmt.Errorf("update node policy: %w", err)
	}
	if err := svc.Repo.DeleteNodeIntentsByPolicyID(ctx, policyObjID); err != nil {
		return fmt.Errorf("delete node intents by policy ID: %w", err)
	}

	intents := make([]*domain.NodeSchedulingIntent, 0, len(nodes))
	for _, node := range nodes {
		intent := domain.NewNodeSchedulingIntent(policy, node)
		intents = append(intents, &intent)
	}
	if err := svc.Repo.InsertNodeIntents(ctx, intents); err != nil {
		return fmt.Errorf("insert node intents into repository: %w", err)
	}

	if len(oldIntentQuery.Result) > 0 {
		svc.notifyDMsDeleteNodeIntents(ctx, oldIntentQuery.Result)
	}

	if err := svc.sendNodeIntentsToDMs(ctx, intents); err != nil {
		return err
	}

	logger.Logger(ctx).Info().Msgf("updated node policy %s and regenerated intents", policyID)
	return nil
}

// DeleteNodeSchedulingPolicy deletes the policy and its associated intents,
// notifying decision makers on matched nodes so they clear their in-memory
// cache.
func (svc *Service) DeleteNodeSchedulingPolicy(ctx context.Context, operator *domain.Claims, policyID string) error {
	policyObjID, err := bson.ObjectIDFromHex(policyID)
	if err != nil {
		return errors.WithMessagef(err, "invalid node policy ID %s", policyID)
	}

	operatorID, err := operator.GetBsonObjectUID()
	if err != nil {
		return errors.WithMessagef(err, "invalid operator ID %s", operator.UID)
	}

	queryOpt := &domain.QueryNodePolicyOptions{
		IDs:        []bson.ObjectID{policyObjID},
		CreatorIDs: []bson.ObjectID{operatorID},
	}
	if err := svc.Repo.QueryNodePolicies(ctx, queryOpt); err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "node policy not found or you don't have permission to delete it", nil)
	}

	intentQueryOpt := &domain.QueryNodeIntentOptions{
		PolicyIDs: []bson.ObjectID{policyObjID},
	}
	if err := svc.Repo.QueryNodeIntents(ctx, intentQueryOpt); err != nil {
		return fmt.Errorf("query node intents for policy: %w", err)
	}

	if err := svc.Repo.DeleteNodeIntentsByPolicyID(ctx, policyObjID); err != nil {
		return fmt.Errorf("delete node intents by policy ID: %w", err)
	}
	if err := svc.Repo.DeleteNodePolicy(ctx, policyObjID); err != nil {
		return fmt.Errorf("delete node policy: %w", err)
	}

	if len(intentQueryOpt.Result) > 0 {
		svc.notifyDMsDeleteNodeIntents(ctx, intentQueryOpt.Result)
	}

	logger.Logger(ctx).Info().Msgf("deleted node policy %s and its associated intents", policyID)
	return nil
}

// DeleteNodeSchedulingIntents deletes specific node intents owned by the
// operator.
func (svc *Service) DeleteNodeSchedulingIntents(ctx context.Context, operator *domain.Claims, intentIDs []string) error {
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
			return errors.WithMessagef(err, "invalid node intent ID %s", id)
		}
		intentObjIDs = append(intentObjIDs, objID)
	}

	queryOpt := &domain.QueryNodeIntentOptions{
		IDs:        intentObjIDs,
		CreatorIDs: []bson.ObjectID{operatorID},
	}
	if err := svc.Repo.QueryNodeIntents(ctx, queryOpt); err != nil {
		return err
	}
	if len(queryOpt.Result) == 0 {
		return errs.NewHTTPStatusError(http.StatusNotFound, "one or more node intents not found or you don't have permission to delete them", nil)
	}

	if err := svc.Repo.DeleteNodeIntents(ctx, intentObjIDs); err != nil {
		return fmt.Errorf("delete node intents: %w", err)
	}

	svc.notifyDMsDeleteNodeIntents(ctx, queryOpt.Result)
	return nil
}

// notifyDMsDeleteNodeIntents notifies the decision makers on nodes touched
// by the given intents to remove the corresponding policy from their
// in-memory cache. Since a NodeSchedulingIntent is always a (policy, node)
// pair, removing "this intent" on the Decision Maker means removing that
// policy for that node.
func (svc *Service) notifyDMsDeleteNodeIntents(ctx context.Context, intents []*domain.NodeSchedulingIntent) {
	if len(intents) == 0 || svc.DMAdapter == nil || svc.K8SAdapter == nil {
		return
	}

	nodeIDsMap := make(map[string]struct{})
	nodeIDs := make([]string, 0)
	policyIDsByNode := make(map[string]map[string]struct{})
	for _, intent := range intents {
		if _, exists := nodeIDsMap[intent.NodeID]; !exists {
			nodeIDsMap[intent.NodeID] = struct{}{}
			nodeIDs = append(nodeIDs, intent.NodeID)
		}
		if policyIDsByNode[intent.NodeID] == nil {
			policyIDsByNode[intent.NodeID] = make(map[string]struct{})
		}
		policyIDsByNode[intent.NodeID][intent.PolicyID.Hex()] = struct{}{}
	}

	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabelSelector,
		NodeIDs:            nodeIDs,
	}
	dmPods, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		logger.Logger(ctx).Warn().Err(err).Msg("failed to query decision maker pods for node intent deletion notification")
		return
	}

	for _, dmPod := range dmPods {
		if dmPod.State != domain.NodeStateOnline {
			continue
		}
		for policyID := range policyIDsByNode[dmPod.NodeID] {
			deleteReq := &domain.DeleteNodeIntentsRequest{PolicyID: policyID}
			if err := svc.DMAdapter.DeleteNodeSchedulingIntents(ctx, dmPod, deleteReq); err != nil {
				logger.Logger(ctx).Warn().Err(err).Msgf("failed to notify decision maker %s to delete node intents for policy %s", dmPod.NodeID, policyID)
			}
		}
	}
}
