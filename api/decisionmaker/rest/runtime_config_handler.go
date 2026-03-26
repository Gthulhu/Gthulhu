package rest

import (
	"net/http"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

type ApplyRuntimeConfigRequest struct {
	ConfigVersion     string `json:"configVersion,omitempty"`
	Mode              string `json:"mode,omitempty"`
	SliceNsDefault    uint64 `json:"sliceNsDefault,omitempty"`
	SliceNsMin        uint64 `json:"sliceNsMin,omitempty"`
	KernelMode        bool   `json:"kernelMode,omitempty"`
	MaxTimeWatchdog   bool   `json:"maxTimeWatchdog,omitempty"`
	EarlyProcessing   bool   `json:"earlyProcessing,omitempty"`
	BuiltinIdle       bool   `json:"builtinIdle,omitempty"`
	SchedulerEnabled  bool   `json:"schedulerEnabled"`
	MonitoringEnabled bool   `json:"monitoringEnabled"`
}

type RuntimeConfigStatusResponse struct {
	ConfigVersion string `json:"configVersion,omitempty"`
	Applied       bool   `json:"applied"`
	AppliedAt     string `json:"appliedAt,omitempty"`
	RestartCount  int64  `json:"restartCount,omitempty"`
	LastError     string `json:"lastError,omitempty"`
}

func (h *Handler) ApplyRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ApplyRuntimeConfigRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	err := h.Service.ApplyRuntimeConfig(ctx, domain.RuntimeSchedulerConfig{
		ConfigVersion:     req.ConfigVersion,
		Mode:              req.Mode,
		SliceNsDefault:    req.SliceNsDefault,
		SliceNsMin:        req.SliceNsMin,
		KernelMode:        req.KernelMode,
		MaxTimeWatchdog:   req.MaxTimeWatchdog,
		EarlyProcessing:   req.EarlyProcessing,
		BuiltinIdle:       req.BuiltinIdle,
		SchedulerEnabled:  req.SchedulerEnabled,
		MonitoringEnabled: req.MonitoringEnabled,
	})
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Failed to apply runtime config", err)
		return
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](nil))
}

func (h *Handler) GetRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	status := h.Service.GetRuntimeConfigStatus(ctx)
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(&RuntimeConfigStatusResponse{
		ConfigVersion: status.ConfigVersion,
		Applied:       status.Applied,
		AppliedAt:     status.AppliedAt,
		RestartCount:  status.RestartCount,
		LastError:     status.LastError,
	}))
}
