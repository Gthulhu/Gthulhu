package service

import (
	"context"
	"fmt"
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
)

type runtimeConfigDMAdapter interface {
	ApplyRuntimeConfig(ctx context.Context, decisionMaker *domain.DecisionMakerPod, config domain.RuntimeSchedulerConfig) error
	GetRuntimeConfigStatus(ctx context.Context, decisionMaker *domain.DecisionMakerPod) (domain.RuntimeConfigApplyResult, error)
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

	dmAdapter, ok := svc.DMAdapter.(runtimeConfigDMAdapter)
	if !ok {
		return nil, errs.NewHTTPStatusError(http.StatusNotImplemented, "decision maker runtime config adapter is not enabled", nil)
	}

	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: domain.LabelSelector{Key: "app", Value: "decisionmaker"},
		NodeIDs:            opt.NodeIDs,
	}
	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return nil, err
	}
	if len(dms) == 0 {
		return nil, errs.NewHTTPStatusError(http.StatusNotFound, "no decision maker pods found", nil)
	}

	results := make([]domain.RuntimeConfigApplyResult, 0, len(dms))
	for _, dm := range dms {
		result := domain.RuntimeConfigApplyResult{
			NodeID: dm.NodeID,
			Host:   dm.Host,
		}
		if dm.State != domain.NodeStateOnline {
			result.Success = false
			result.Error = "decision maker is offline"
			results = append(results, result)
			continue
		}
		if err := dmAdapter.ApplyRuntimeConfig(ctx, dm, opt.Config); err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
		}
		results = append(results, result)
	}

	_ = operator
	return results, nil
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

	results := make([]domain.RuntimeConfigApplyResult, 0, len(dms))
	for _, dm := range dms {
		if dm.State != domain.NodeStateOnline {
			results = append(results, domain.RuntimeConfigApplyResult{
				NodeID:  dm.NodeID,
				Host:    dm.Host,
				Success: false,
				Error:   "decision maker is offline",
			})
			continue
		}

		result, err := dmAdapter.GetRuntimeConfigStatus(ctx, dm)
		if err != nil {
			results = append(results, domain.RuntimeConfigApplyResult{
				NodeID:  dm.NodeID,
				Host:    dm.Host,
				Success: false,
				Error:   err.Error(),
			})
			continue
		}
		if result.NodeID == "" {
			result.NodeID = dm.NodeID
		}
		if result.Host == "" {
			result.Host = dm.Host
		}
		results = append(results, result)
	}

	return results, nil
}
