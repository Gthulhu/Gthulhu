package rest

import (
	"context"
	"net/http"
	"strings"

	"github.com/Gthulhu/api/manager/domain"
)

type ApplyRuntimeConfigRequest struct {
	NodeIDs []string                      `json:"nodeIds,omitempty"`
	Config  domain.RuntimeSchedulerConfig `json:"config"`
}

type ApplyRuntimeConfigResponse struct {
	Results []domain.RuntimeConfigApplyResult `json:"results"`
}

func (h *Handler) ApplyRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req ApplyRuntimeConfigRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	svc, ok := h.Svc.(interface {
		ApplyRuntimeConfig(ctx context.Context, operator *domain.Claims, opt *domain.RuntimeConfigApplyOptions) ([]domain.RuntimeConfigApplyResult, error)
	})
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusNotImplemented, "Runtime config is not enabled", nil)
		return
	}

	results, err := svc.ApplyRuntimeConfig(ctx, &claims, &domain.RuntimeConfigApplyOptions{
		NodeIDs: req.NodeIDs,
		Config:  req.Config,
	})
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(&ApplyRuntimeConfigResponse{Results: results}))
}

func (h *Handler) GetRuntimeConfigStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodeIDsParam := strings.TrimSpace(r.URL.Query().Get("nodeIds"))
	nodeIDs := make([]string, 0)
	if nodeIDsParam != "" {
		for _, nodeID := range strings.Split(nodeIDsParam, ",") {
			nodeID = strings.TrimSpace(nodeID)
			if nodeID != "" {
				nodeIDs = append(nodeIDs, nodeID)
			}
		}
	}

	svc, ok := h.Svc.(interface {
		GetRuntimeConfigStatus(ctx context.Context, nodeIDs []string) ([]domain.RuntimeConfigApplyResult, error)
	})
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusNotImplemented, "Runtime config is not enabled", nil)
		return
	}

	results, err := svc.GetRuntimeConfigStatus(ctx, nodeIDs)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(&ApplyRuntimeConfigResponse{Results: results}))
}
