package rest

import (
	"net/http"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

// UpdateMetricsRequest represents the payload for updating metrics.
type UpdateMetricsRequest struct {
	Usersched_last_run_at uint64 `json:"usersched_last_run_at"` // The PID of the userspace scheduler
	Nr_queued             uint64 `json:"nr_queued"`             // Number of tasks queued in the userspace scheduler
	Nr_scheduled          uint64 `json:"nr_scheduled"`          // Number of tasks scheduled by the userspace scheduler
	Nr_running            uint64 `json:"nr_running"`            // Number of tasks currently running in the userspace scheduler
	Nr_online_cpus        uint64 `json:"nr_online_cpus"`        // Number of online CPUs in the system
	Nr_user_dispatches    uint64 `json:"nr_user_dispatches"`    // Number of user-space dispatches
	Nr_kernel_dispatches  uint64 `json:"nr_kernel_dispatches"`  // Number of kernel-space dispatches
	Nr_cancel_dispatches  uint64 `json:"nr_cancel_dispatches"`  // Number of cancelled dispatches
	Nr_bounce_dispatches  uint64 `json:"nr_bounce_dispatches"`  // Number of bounce dispatches
	Nr_failed_dispatches  uint64 `json:"nr_failed_dispatches"`  // Number of failed dispatches
	Nr_sched_congested    uint64 `json:"nr_sched_congested"`    // Number of times the scheduler was congested
}

// UpdateMetrics handles the updating of metrics via a REST endpoint.
func (h *Handler) UpdateMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req UpdateMetricsRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	newMetricSet := &domain.MetricSet{
		UserSchedLastRunAt: req.Usersched_last_run_at,
		NrQueued:           req.Nr_queued,
		NrScheduled:        req.Nr_scheduled,
		NrRunning:          req.Nr_running,
		NrOnlineCPUs:       req.Nr_online_cpus,
		NrUserDispatches:   req.Nr_user_dispatches,
		NrKernelDispatches: req.Nr_kernel_dispatches,
		NrCancelDispatches: req.Nr_cancel_dispatches,
		NrBounceDispatches: req.Nr_bounce_dispatches,
		NrFailedDispatches: req.Nr_failed_dispatches,
		NrSchedCongested:   req.Nr_sched_congested,
	}
	h.Service.UpdateMetrics(r.Context(), newMetricSet)
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](nil))
}
