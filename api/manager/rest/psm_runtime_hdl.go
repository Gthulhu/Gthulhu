package rest

import (
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
)

type PodSchedulingMetricValueItem struct {
	Namespace              string `json:"namespace"`
	PodName                string `json:"podName"`
	NodeID                 string `json:"nodeID,omitempty"`
	VoluntaryCtxSwitches   uint64 `json:"voluntaryCtxSwitches"`
	InvoluntaryCtxSwitches uint64 `json:"involuntaryCtxSwitches"`
	CPUTimeNs              uint64 `json:"cpuTimeNs"`
	WaitTimeNs             uint64 `json:"waitTimeNs"`
	RunCount               uint64 `json:"runCount"`
	CPUMigrations          uint64 `json:"cpuMigrations"`
	SMTMigrations          uint64 `json:"smtMigrations"`
	L3Migrations           uint64 `json:"l3Migrations"`
	NUMAMigrations         uint64 `json:"numaMigrations"`
}

type ListPodSchedulingMetricValuesResponse struct {
	Items    []*PodSchedulingMetricValueItem `json:"items"`
	Warnings []string                        `json:"warnings,omitempty"`
}

// ListPodSchedulingMetricValues godoc
// @Summary List collected pod scheduling metrics
// @Description List the latest pod-level eBPF scheduling metrics collected from decision makers.
// @Tags PodSchedulingMetrics
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListPodSchedulingMetricValuesResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/pod-scheduling-metrics/runtime [get]
func (h *Handler) ListPodSchedulingMetricValues(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	result, err := h.Svc.ListPodSchedulingMetricValues(ctx)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := &ListPodSchedulingMetricValuesResponse{
		Items:    make([]*PodSchedulingMetricValueItem, 0, len(result.Items)),
		Warnings: result.Warnings,
	}
	for _, item := range result.Items {
		resp.Items = append(resp.Items, domainPodSchedulingMetricValueToResponse(item))
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(resp))
}

func domainPodSchedulingMetricValueToResponse(item *domain.PodSchedulingMetricValue) *PodSchedulingMetricValueItem {
	return &PodSchedulingMetricValueItem{
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
}
