package rest

import (
	"net/http"
	"strings"
)

type ingestMetricsRequest struct {
	Timestamp int64          `json:"timestamp"`
	Namespace string         `json:"namespace"`
	Pod       string         `json:"pod"`
	Node      string         `json:"node"`
	Metrics   metricsPayload `json:"metrics"`
}

// IngestPodMetrics godoc
// @Summary Ingest pod metrics for adaptive classification
// @Description Accept periodic pod scheduling metrics and update adaptive clustering state.
// @Tags PodSchedulingMetrics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body ingestMetricsRequest true "Pod metrics payload"
// @Success 200 {object} SuccessResponse[classifyResponseItem]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/metrics [post]
func (h *Handler) IngestPodMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req ingestMetricsRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "invalid request body", err)
		return
	}
	if strings.TrimSpace(req.Namespace) == "" || strings.TrimSpace(req.Pod) == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "namespace and pod are required", nil)
		return
	}

	item := h.classifier.Ingest(classificationInput{
		Timestamp: req.Timestamp,
		Namespace: req.Namespace,
		Pod:       req.Pod,
		Node:      req.Node,
		Metrics:   req.Metrics,
	})
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(item))
}

// GetPodClassification godoc
// @Summary Get adaptive classification for a pod
// @Description Returns current classification, drift status and recommendation for a pod.
// @Tags PodSchedulingMetrics
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[classifyResponseItem]
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Router /api/v1/classify/{namespace}/{pod} [get]
func (h *Handler) GetPodClassification(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	namespace := h.GetPathParam(r, "namespace")
	pod := h.GetPathParam(r, "pod")
	if namespace == "" || pod == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "namespace and pod path parameters are required", nil)
		return
	}

	item, ok := h.classifier.Get(namespace, pod)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusNotFound, "classification not found", nil)
		return
	}
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(item))
}

// ListPodClassifications godoc
// @Summary List adaptive classifications
// @Description Lists all known pod classifications with optional filters.
// @Tags PodSchedulingMetrics
// @Produce json
// @Security BearerAuth
// @Param namespace query string false "namespace filter"
// @Param phase query string false "phase filter"
// @Param type query string false "classification type filter"
// @Success 200 {object} SuccessResponse[listClassifyResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Router /api/v1/classify [get]
func (h *Handler) ListPodClassifications(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	phase := PodPhase(strings.TrimSpace(r.URL.Query().Get("phase")))
	tag := strings.TrimSpace(r.URL.Query().Get("type"))
	namespace := strings.TrimSpace(r.URL.Query().Get("namespace"))

	resp := &listClassifyResponse{Items: h.classifier.List(namespace, phase, tag)}
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(resp))
}
