package service

import (
	"context"
	"fmt"
	"sort"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/logger"
)

func (svc *Service) ListPodSchedulingMetricValues(ctx context.Context) (*domain.PodSchedulingMetricValuesResult, error) {
	if svc.K8SAdapter == nil || svc.DMAdapter == nil {
		return nil, domain.ErrNoClient
	}

	dmQueryOpt := &domain.QueryDecisionMakerPodsOptions{
		DecisionMakerLabel: domain.LabelSelector{
			Key:   "app",
			Value: "decisionmaker",
		},
	}

	dms, err := svc.K8SAdapter.QueryDecisionMakerPods(ctx, dmQueryOpt)
	if err != nil {
		return nil, fmt.Errorf("query decision maker pods: %w", err)
	}

	result := &domain.PodSchedulingMetricValuesResult{
		Items: make([]*domain.PodSchedulingMetricValue, 0),
	}
	if len(dms) == 0 {
		return result, nil
	}

	aggregated := make(map[string]*domain.PodSchedulingMetricValue)
	for _, dm := range dms {
		if dm.State != domain.NodeStateOnline {
			result.Warnings = append(result.Warnings, fmt.Sprintf("decision maker on node %s is not online", dm.NodeID))
			continue
		}

		items, err := svc.DMAdapter.GetPodSchedulingMetricValues(ctx, dm)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to collect pod scheduling metrics from decision maker %s", dm.NodeID)
			result.Warnings = append(result.Warnings, fmt.Sprintf("failed to collect metrics from node %s", dm.NodeID))
			continue
		}

		for _, item := range items {
			if item == nil {
				continue
			}
			if item.NodeID == "" {
				item.NodeID = dm.NodeID
			}

			key := item.Namespace + "/" + item.PodName + "/" + item.NodeID
			existing, ok := aggregated[key]
			if !ok {
				aggregated[key] = &domain.PodSchedulingMetricValue{
					Namespace:              item.Namespace,
					PodName:                item.PodName,
					NodeID:                 item.NodeID,
					VoluntaryCtxSwitches:   item.VoluntaryCtxSwitches,
					InvoluntaryCtxSwitches: item.InvoluntaryCtxSwitches,
					CPUTimeNs:              item.CPUTimeNs,
					WaitTimeNs:             item.WaitTimeNs,
					RunCount:               item.RunCount,
					CPUMigrations:          item.CPUMigrations,
					SMTMigrations:          item.SMTMigrations,
					L3Migrations:           item.L3Migrations,
					NUMAMigrations:         item.NUMAMigrations,
				}
				continue
			}

			existing.VoluntaryCtxSwitches += item.VoluntaryCtxSwitches
			existing.InvoluntaryCtxSwitches += item.InvoluntaryCtxSwitches
			existing.CPUTimeNs += item.CPUTimeNs
			existing.WaitTimeNs += item.WaitTimeNs
			existing.RunCount += item.RunCount
			existing.CPUMigrations += item.CPUMigrations
			existing.SMTMigrations += item.SMTMigrations
			existing.L3Migrations += item.L3Migrations
			existing.NUMAMigrations += item.NUMAMigrations
		}
	}

	for _, item := range aggregated {
		result.Items = append(result.Items, item)
	}

	sort.Slice(result.Items, func(i, j int) bool {
		if result.Items[i].Namespace != result.Items[j].Namespace {
			return result.Items[i].Namespace < result.Items[j].Namespace
		}
		if result.Items[i].PodName != result.Items[j].PodName {
			return result.Items[i].PodName < result.Items[j].PodName
		}
		return result.Items[i].NodeID < result.Items[j].NodeID
	})

	return result, nil
}
