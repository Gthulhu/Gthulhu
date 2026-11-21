package service

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"slices"
	"sync"
	"time"

	dmdomain "github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/Gthulhu/api/pkg/logger"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const (
	nodePreviewWorkers      = 8
	nodePreviewTimeout      = 5 * time.Second
	nodePreviewTotalTimeout = 15 * time.Second
)

// PreviewNodeSchedulingPolicy resolves the same selectors as CRUD without
// writing any policy or intent. A failed DM contributes a warning, not a
// failure of the other nodes' previews.
func (svc *Service) PreviewNodeSchedulingPolicy(ctx context.Context, operator *domain.Claims, policyID string, policy *domain.NodeSchedulingPolicy) (*domain.NodeSchedulingPolicyPreview, error) {
	if (policyID == "") == (policy == nil) {
		return nil, errs.NewHTTPStatusError(http.StatusBadRequest, "provide exactly one of policyId or policy", nil)
	}
	if policyID != "" {
		id, err := bson.ObjectIDFromHex(policyID)
		if err != nil {
			return nil, errs.NewHTTPStatusError(http.StatusBadRequest, "invalid node policy ID", err)
		}
		operatorID, err := operator.GetBsonObjectUID()
		if err != nil {
			return nil, errs.NewHTTPStatusError(http.StatusUnauthorized, "invalid operator ID", err)
		}
		query := &domain.QueryNodePolicyOptions{IDs: []bson.ObjectID{id}, CreatorIDs: []bson.ObjectID{operatorID}}
		if err := svc.Repo.QueryNodePolicies(ctx, query); err != nil {
			return nil, fmt.Errorf("query node policy for preview: %w", err)
		}
		if len(query.Result) == 0 {
			return nil, errs.NewHTTPStatusError(http.StatusNotFound, "node policy not found or you don't have permission to preview it", nil)
		}
		policyID = id.Hex()
		policy = query.Result[0]
	}
	result := &domain.NodeSchedulingPolicyPreview{
		Policy:       policy,
		MatchedNodes: []string{},
		MatchedTasks: []dmdomain.TaskInfo{},
		Conflicts:    []dmdomain.NodePolicyConflict{},
		Warnings:     []dmdomain.NodePolicyWarning{},
	}

	var invalidRegex string
	if policy.CommandRegex == "" {
		invalidRegex = "commandRegex must not be empty"
	} else if _, err := regexp.Compile(policy.CommandRegex); err != nil {
		invalidRegex = err.Error()
	}
	if invalidRegex != "" {
		result.Warnings = append(result.Warnings, dmdomain.NodePolicyWarning{Code: "invalidRegex", Message: invalidRegex})
		return result, nil
	}

	nodes, err := svc.resolveNodesForPolicy(ctx, policy)
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		result.Warnings = append(result.Warnings, dmdomain.NodePolicyWarning{
			Code: "no_matching_nodes", Message: "no nodes match the policy selectors",
		})
		return result, nil
	}
	for _, node := range nodes {
		result.MatchedNodes = append(result.MatchedNodes, node.Name)
	}
	slices.Sort(result.MatchedNodes)

	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: dmLabelSelector,
		NodeIDs:            result.MatchedNodes,
	})
	if err != nil {
		return nil, fmt.Errorf("query decision maker pods for preview: %w", err)
	}
	byNode := make(map[string]*domain.DecisionMakerPod, len(dms))
	for _, dm := range dms {
		if dm == nil || dm.State != domain.NodeStateOnline || dm.Host == "" || dm.Port <= 0 {
			continue
		}
		// Select one stable endpoint when pods overlap during rollout.
		current := byNode[dm.NodeID]
		if current == nil || dm.Host < current.Host || (dm.Host == current.Host && dm.Port < current.Port) {
			byNode[dm.NodeID] = dm
		}
	}

	type nodeResult struct {
		preview *dmdomain.NodePolicyPreview
		warning dmdomain.NodePolicyWarning
	}
	perNode := make([]nodeResult, len(nodes))
	fanoutCtx, cancel := context.WithTimeout(ctx, nodePreviewTotalTimeout)
	defer cancel()
	var workers sync.WaitGroup
	workerCount := min(nodePreviewWorkers, len(nodes))
	for worker := range workerCount {
		workers.Go(func() {
			for i := worker; i < len(nodes); i += workerCount {
				node := result.MatchedNodes[i]
				dm := byNode[node]
				if dm == nil {
					perNode[i].warning = dmdomain.NodePolicyWarning{
						Node: node, Code: "decision_maker_unavailable", Message: "no online Decision Maker for this node",
					}
					continue
				}
				previewCtx, cancel := context.WithTimeout(fanoutCtx, nodePreviewTimeout)
				var preview *dmdomain.NodePolicyPreview
				err := previewCtx.Err()
				if err == nil {
					preview, err = svc.DMAdapter.PreviewNodeSchedulingPolicy(previewCtx, dm, dmdomain.NodePolicyPreviewRequest{
						Policy: &dmdomain.NodePolicy{
							PolicyID:      policyID,
							Node:          node,
							NodeID:        dm.NodeID,
							Generation:    policy.Generation,
							CommandRegex:  policy.CommandRegex,
							Priority:      policy.Priority,
							ExecutionTime: policy.ExecutionTime,
						},
					})
				}
				cancel()
				if err != nil || preview == nil {
					logger.Logger(ctx).Warn().Err(err).Str("node", node).Msg("Decision Maker preview failed")
					perNode[i].warning = dmdomain.NodePolicyWarning{
						Node: node, Code: "decision_maker_preview_failed", Message: "could not preview policy on this node",
					}
					continue
				}
				perNode[i].preview = preview
			}
		})
	}
	workers.Wait()

	for i, r := range perNode {
		if r.preview == nil {
			result.Warnings = append(result.Warnings, r.warning)
			continue
		}
		node := result.MatchedNodes[i]
		for _, task := range r.preview.MatchedTasks {
			task.Node = node
			result.MatchedTasks = append(result.MatchedTasks, task)
		}
		for _, conflict := range r.preview.Conflicts {
			conflict.Task.Node = node
			result.Conflicts = append(result.Conflicts, conflict)
		}
		for _, warning := range r.preview.Warnings {
			warning.Node = node
			result.Warnings = append(result.Warnings, warning)
		}
	}
	return result, nil
}
