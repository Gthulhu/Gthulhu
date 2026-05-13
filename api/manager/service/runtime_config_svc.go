package service

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
)

type runtimeConfigDMAdapter interface {
	ApplyRuntimeConfig(ctx context.Context, decisionMaker *domain.DecisionMakerPod, config domain.RuntimeSchedulerConfig) error
	GetRuntimeConfigStatus(ctx context.Context, decisionMaker *domain.DecisionMakerPod) (domain.RuntimeConfigApplyResult, error)
}

type runtimeConfigRepository interface {
	UpsertNodeRuntimeConfig(ctx context.Context, cfg *domain.NodeRuntimeConfig) error
	QueryNodeRuntimeConfigs(ctx context.Context, opt *domain.QueryNodeRuntimeConfigOptions) error
}

func (svc *Service) ApplyRuntimeConfig(ctx context.Context, operator *domain.Claims, opt *domain.RuntimeConfigApplyOptions) ([]domain.RuntimeConfigApplyResult, error) {
	if svc.K8SAdapter == nil {
		return nil, domain.ErrNoClient
	}
	if opt == nil {
		return nil, errs.NewHTTPStatusError(http.StatusBadRequest, "invalid request", fmt.Errorf("runtime config options is nil"))
	}
	if opt.Config.ConfigVersion == "" {
		return nil, errs.NewHTTPStatusError(http.StatusBadRequest, "configVersion is required", nil)
	}
	opt.Config.Normalize()
	if err := opt.Config.Validate(); err != nil {
		return nil, errs.NewHTTPStatusError(http.StatusBadRequest, "invalid runtime config", err)
	}

	dmAdapter, ok := svc.DMAdapter.(runtimeConfigDMAdapter)
	if !ok {
		return nil, errs.NewHTTPStatusError(http.StatusNotImplemented, "decision maker runtime config adapter is not enabled", nil)
	}
	repo, _ := svc.Repo.(runtimeConfigRepository)
	updatedBy := ""
	if operator != nil {
		updatedBy = operator.UID
	}
	now := time.Now().UnixMilli()

	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: domain.LabelSelector{Key: "app", Value: "decisionmaker"},
		NodeIDs:            opt.NodeIDs,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return nil, err
	}
	if len(dms) == 0 {
		if repo != nil && len(opt.NodeIDs) > 0 {
			results := make([]domain.RuntimeConfigApplyResult, 0, len(opt.NodeIDs))
			for _, nodeID := range opt.NodeIDs {
				result := persistUnreachableRuntimeConfig(ctx, repo, nodeID, opt.Config, updatedBy, now, "decision maker is not discovered")
				results = append(results, result)
			}
			return results, nil
		}
		return nil, errs.NewHTTPStatusError(http.StatusNotFound, "no decision maker pods found", nil)
	}

	results := make([]domain.RuntimeConfigApplyResult, 0, len(dms))
	seenNodes := map[string]struct{}{}
	for _, dm := range dms {
		seenNodes[dm.NodeID] = struct{}{}
		result := domain.RuntimeConfigApplyResult{
			NodeID: dm.NodeID,
			Host:   dm.Host,
		}
		desired := &domain.NodeRuntimeConfig{
			NodeID:        dm.NodeID,
			ConfigVersion: opt.Config.ConfigVersion,
			Config:        opt.Config,
			UpdatedBy:     updatedBy,
			UpdatedAt:     now,
		}
		if repo != nil {
			if err := repo.UpsertNodeRuntimeConfig(ctx, desired); err != nil {
				result.Success = false
				result.Error = fmt.Sprintf("persist desired runtime config: %v", err)
				results = append(results, result)
				continue
			}
		}
		if dm.State != domain.NodeStateOnline {
			result.Success = false
			result.Error = "decision maker is offline"
			result.DesiredConfig = &desired.Config
			results = append(results, result)
			if repo != nil {
				desired.LastApplyResult = result
				_ = repo.UpsertNodeRuntimeConfig(ctx, desired)
			}
			continue
		}
		if err := dmAdapter.ApplyRuntimeConfig(ctx, dm, opt.Config); err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
		}
		result.ConfigVersion = opt.Config.ConfigVersion
		result.DesiredConfig = &desired.Config
		results = append(results, result)
		if repo != nil {
			desired.LastApplyResult = result
			_ = repo.UpsertNodeRuntimeConfig(ctx, desired)
		}
	}
	if repo != nil {
		for _, nodeID := range opt.NodeIDs {
			if _, ok := seenNodes[nodeID]; ok {
				continue
			}
			result := persistUnreachableRuntimeConfig(ctx, repo, nodeID, opt.Config, updatedBy, now, "decision maker is not discovered")
			results = append(results, result)
		}
	}
	return results, nil
}

func persistUnreachableRuntimeConfig(ctx context.Context, repo runtimeConfigRepository, nodeID string, config domain.RuntimeSchedulerConfig, updatedBy string, updatedAt int64, errMsg string) domain.RuntimeConfigApplyResult {
	result := domain.RuntimeConfigApplyResult{
		NodeID:        nodeID,
		Success:       false,
		Error:         errMsg,
		ConfigVersion: config.ConfigVersion,
		DesiredConfig: &config,
		Drift:         true,
	}
	desired := &domain.NodeRuntimeConfig{
		NodeID:          nodeID,
		ConfigVersion:   config.ConfigVersion,
		Config:          config,
		UpdatedBy:       updatedBy,
		UpdatedAt:       updatedAt,
		LastApplyResult: result,
	}
	if err := repo.UpsertNodeRuntimeConfig(ctx, desired); err != nil {
		result.Error = fmt.Sprintf("persist desired runtime config: %v", err)
	}
	return result
}

func (svc *Service) GetRuntimeConfigStatus(ctx context.Context, nodeIDs []string) ([]domain.RuntimeConfigApplyResult, error) {
	if svc.K8SAdapter == nil {
		return nil, domain.ErrNoClient
	}

	dmAdapter, ok := svc.DMAdapter.(runtimeConfigDMAdapter)
	if !ok {
		return nil, errs.NewHTTPStatusError(http.StatusNotImplemented, "decision maker runtime config adapter is not enabled", nil)
	}

	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: domain.LabelSelector{Key: "app", Value: "decisionmaker"},
		NodeIDs:            nodeIDs,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return nil, err
	}

	desiredByNode := map[string]*domain.NodeRuntimeConfig{}
	if repo, ok := svc.Repo.(runtimeConfigRepository); ok {
		queryOpt := &domain.QueryNodeRuntimeConfigOptions{NodeIDs: nodeIDs}
		if err := repo.QueryNodeRuntimeConfigs(ctx, queryOpt); err != nil {
			return nil, err
		}
		for _, desired := range queryOpt.Result {
			desiredByNode[desired.NodeID] = desired
		}
	}

	results := make([]domain.RuntimeConfigApplyResult, 0, len(dms))
	seenNodes := map[string]struct{}{}
	for _, dm := range dms {
		seenNodes[dm.NodeID] = struct{}{}
		desired := desiredByNode[dm.NodeID]
		if dm.State != domain.NodeStateOnline {
			result := domain.RuntimeConfigApplyResult{
				NodeID:  dm.NodeID,
				Host:    dm.Host,
				Success: false,
				Error:   "decision maker is offline",
			}
			attachDesiredRuntimeConfig(&result, desired)
			results = append(results, result)
			continue
		}

		result, err := dmAdapter.GetRuntimeConfigStatus(ctx, dm)
		if err != nil {
			result := domain.RuntimeConfigApplyResult{
				NodeID:  dm.NodeID,
				Host:    dm.Host,
				Success: false,
				Error:   err.Error(),
			}
			attachDesiredRuntimeConfig(&result, desired)
			results = append(results, result)
			continue
		}
		if result.NodeID == "" {
			result.NodeID = dm.NodeID
		}
		if result.Host == "" {
			result.Host = dm.Host
		}
		attachDesiredRuntimeConfig(&result, desired)
		results = append(results, result)
	}

	for nodeID, desired := range desiredByNode {
		if _, ok := seenNodes[nodeID]; ok {
			continue
		}
		result := domain.RuntimeConfigApplyResult{
			NodeID:  nodeID,
			Success: false,
			Error:   "decision maker is not discovered",
		}
		attachDesiredRuntimeConfig(&result, desired)
		results = append(results, result)
	}

	return results, nil
}

func attachDesiredRuntimeConfig(result *domain.RuntimeConfigApplyResult, desired *domain.NodeRuntimeConfig) {
	if result == nil || desired == nil {
		return
	}
	desiredConfig := desired.Config
	result.DesiredConfig = &desiredConfig
	if result.Config == nil {
		result.Drift = true
		return
	}
	appliedConfig := *result.Config
	desiredConfig.Normalize()
	appliedConfig.Normalize()
	result.Drift = !reflect.DeepEqual(desiredConfig, appliedConfig)
}
