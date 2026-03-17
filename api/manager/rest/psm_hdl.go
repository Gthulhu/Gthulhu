package rest

import (
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
)

// ---------------------------------------------------------------------------
// Request / Response DTOs
// ---------------------------------------------------------------------------

type PSMLabelSelector struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type PSMMetricsDTO struct {
	VoluntaryCtxSwitches   bool `json:"voluntaryCtxSwitches"`
	InvoluntaryCtxSwitches bool `json:"involuntaryCtxSwitches"`
	CPUTimeNs              bool `json:"cpuTimeNs"`
	WaitTimeNs             bool `json:"waitTimeNs"`
	RunCount               bool `json:"runCount"`
	CPUMigrations          bool `json:"cpuMigrations"`
}

type PSMScaleTargetRefDTO struct {
	APIVersion string `json:"apiVersion,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
}

type PSMScalingDTO struct {
	Enabled         bool                  `json:"enabled"`
	MetricName      string                `json:"metricName,omitempty"`
	TargetValue     string                `json:"targetValue,omitempty"`
	ScaleTargetRef  *PSMScaleTargetRefDTO `json:"scaleTargetRef,omitempty"`
	MinReplicaCount int32                 `json:"minReplicaCount,omitempty"`
	MaxReplicaCount int32                 `json:"maxReplicaCount,omitempty"`
	CooldownPeriod  int32                 `json:"cooldownPeriod,omitempty"`
}

type CreatePSMRequest struct {
	LabelSelectors            []PSMLabelSelector `json:"labelSelectors"`
	K8sNamespaces             []string           `json:"k8sNamespaces,omitempty"`
	CommandRegex              string             `json:"commandRegex,omitempty"`
	CollectionIntervalSeconds int32              `json:"collectionIntervalSeconds,omitempty"`
	Enabled                   *bool              `json:"enabled,omitempty"`
	Metrics                   *PSMMetricsDTO     `json:"metrics,omitempty"`
	Scaling                   *PSMScalingDTO     `json:"scaling,omitempty"`
}

type UpdatePSMRequest struct {
	ID                        string             `json:"id"`
	LabelSelectors            []PSMLabelSelector `json:"labelSelectors"`
	K8sNamespaces             []string           `json:"k8sNamespaces,omitempty"`
	CommandRegex              string             `json:"commandRegex,omitempty"`
	CollectionIntervalSeconds int32              `json:"collectionIntervalSeconds,omitempty"`
	Enabled                   *bool              `json:"enabled,omitempty"`
	Metrics                   *PSMMetricsDTO     `json:"metrics,omitempty"`
	Scaling                   *PSMScalingDTO     `json:"scaling,omitempty"`
}

type DeletePSMRequest struct {
	ID string `json:"id"`
}

type PSMResponseItem struct {
	ID                        string             `json:"id"`
	LabelSelectors            []PSMLabelSelector `json:"labelSelectors"`
	K8sNamespaces             []string           `json:"k8sNamespaces,omitempty"`
	CommandRegex              string             `json:"commandRegex,omitempty"`
	CollectionIntervalSeconds int32              `json:"collectionIntervalSeconds"`
	Enabled                   bool               `json:"enabled"`
	Metrics                   *PSMMetricsDTO     `json:"metrics,omitempty"`
	Scaling                   *PSMScalingDTO     `json:"scaling,omitempty"`
	CreatedTime               int64              `json:"createdTime,omitempty"`
	UpdatedTime               int64              `json:"updatedTime,omitempty"`
}

type ListPSMResponse struct {
	Items []*PSMResponseItem `json:"items"`
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

// CreatePodSchedulingMetrics godoc
// @Summary Create PodSchedulingMetrics
// @Description Create a new PodSchedulingMetrics resource.
// @Tags PodSchedulingMetrics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreatePSMRequest true "PSM payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/pod-scheduling-metrics [post]
func (h *Handler) CreatePodSchedulingMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreatePSMRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if len(req.LabelSelectors) == 0 {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "At least one label selector is required", nil)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	psm := psmRequestToDomain(&req)

	if err := h.Svc.CreatePodSchedulingMetrics(ctx, &claims, psm); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[string](nil))
}

// ListPodSchedulingMetrics godoc
// @Summary List PodSchedulingMetrics
// @Description List all PodSchedulingMetrics resources.
// @Tags PodSchedulingMetrics
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListPSMResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/pod-scheduling-metrics [get]
func (h *Handler) ListPodSchedulingMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	queryOpt := &domain.QueryPSMOptions{}

	if err := h.Svc.ListPodSchedulingMetrics(ctx, queryOpt); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := ListPSMResponse{
		Items: make([]*PSMResponseItem, len(queryOpt.Result)),
	}
	for i, d := range queryOpt.Result {
		resp.Items[i] = domainPSMToResponse(d)
	}
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(&resp))
}

// UpdatePodSchedulingMetrics godoc
// @Summary Update PodSchedulingMetrics
// @Description Update an existing PodSchedulingMetrics resource.
// @Tags PodSchedulingMetrics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdatePSMRequest true "PSM payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/pod-scheduling-metrics [put]
func (h *Handler) UpdatePodSchedulingMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req UpdatePSMRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if req.ID == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "ID is required", nil)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	psm := updatePSMRequestToDomain(&req)

	if err := h.Svc.UpdatePodSchedulingMetrics(ctx, &claims, req.ID, psm); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](&EmptyResponse{}))
}

// DeletePodSchedulingMetrics godoc
// @Summary Delete PodSchedulingMetrics
// @Description Delete a PodSchedulingMetrics resource.
// @Tags PodSchedulingMetrics
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeletePSMRequest true "PSM ID to delete"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/pod-scheduling-metrics [delete]
func (h *Handler) DeletePodSchedulingMetrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req DeletePSMRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if req.ID == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "ID is required", nil)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.Svc.DeletePodSchedulingMetrics(ctx, &claims, req.ID); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](&EmptyResponse{}))
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

func psmRequestToDomain(req *CreatePSMRequest) *domain.PodSchedulingMetrics {
	psm := &domain.PodSchedulingMetrics{
		K8sNamespaces:             req.K8sNamespaces,
		CommandRegex:              req.CommandRegex,
		CollectionIntervalSeconds: req.CollectionIntervalSeconds,
		Enabled:                   true,
	}
	if req.Enabled != nil {
		psm.Enabled = *req.Enabled
	}
	for _, ls := range req.LabelSelectors {
		psm.LabelSelectors = append(psm.LabelSelectors, domain.LabelSelector{Key: ls.Key, Value: ls.Value})
	}
	if req.Metrics != nil {
		psm.Metrics = metricsDTOToDomain(req.Metrics)
	}
	if req.Scaling != nil {
		psm.Scaling = scalingDTOToDomain(req.Scaling)
	}
	return psm
}

func updatePSMRequestToDomain(req *UpdatePSMRequest) *domain.PodSchedulingMetrics {
	psm := &domain.PodSchedulingMetrics{
		K8sNamespaces:             req.K8sNamespaces,
		CommandRegex:              req.CommandRegex,
		CollectionIntervalSeconds: req.CollectionIntervalSeconds,
		Enabled:                   true,
	}
	if req.Enabled != nil {
		psm.Enabled = *req.Enabled
	}
	for _, ls := range req.LabelSelectors {
		psm.LabelSelectors = append(psm.LabelSelectors, domain.LabelSelector{Key: ls.Key, Value: ls.Value})
	}
	if req.Metrics != nil {
		psm.Metrics = metricsDTOToDomain(req.Metrics)
	}
	if req.Scaling != nil {
		psm.Scaling = scalingDTOToDomain(req.Scaling)
	}
	return psm
}

func metricsDTOToDomain(dto *PSMMetricsDTO) *domain.PSMMetrics {
	return &domain.PSMMetrics{
		VoluntaryCtxSwitches:   dto.VoluntaryCtxSwitches,
		InvoluntaryCtxSwitches: dto.InvoluntaryCtxSwitches,
		CPUTimeNs:              dto.CPUTimeNs,
		WaitTimeNs:             dto.WaitTimeNs,
		RunCount:               dto.RunCount,
		CPUMigrations:          dto.CPUMigrations,
	}
}

func scalingDTOToDomain(dto *PSMScalingDTO) *domain.PSMScaling {
	s := &domain.PSMScaling{
		Enabled:         dto.Enabled,
		MetricName:      dto.MetricName,
		TargetValue:     dto.TargetValue,
		MinReplicaCount: dto.MinReplicaCount,
		MaxReplicaCount: dto.MaxReplicaCount,
		CooldownPeriod:  dto.CooldownPeriod,
	}
	if dto.ScaleTargetRef != nil {
		s.ScaleTargetRef = &domain.PSMScaleTargetRef{
			APIVersion: dto.ScaleTargetRef.APIVersion,
			Kind:       dto.ScaleTargetRef.Kind,
			Name:       dto.ScaleTargetRef.Name,
		}
	}
	return s
}

func domainPSMToResponse(d *domain.PodSchedulingMetrics) *PSMResponseItem {
	item := &PSMResponseItem{
		ID:                        d.ID.Hex(),
		K8sNamespaces:             d.K8sNamespaces,
		CommandRegex:              d.CommandRegex,
		CollectionIntervalSeconds: d.CollectionIntervalSeconds,
		Enabled:                   d.Enabled,
		CreatedTime:               d.CreatedTime,
		UpdatedTime:               d.UpdatedTime,
	}
	for _, ls := range d.LabelSelectors {
		item.LabelSelectors = append(item.LabelSelectors, PSMLabelSelector{Key: ls.Key, Value: ls.Value})
	}
	if d.Metrics != nil {
		item.Metrics = &PSMMetricsDTO{
			VoluntaryCtxSwitches:   d.Metrics.VoluntaryCtxSwitches,
			InvoluntaryCtxSwitches: d.Metrics.InvoluntaryCtxSwitches,
			CPUTimeNs:              d.Metrics.CPUTimeNs,
			WaitTimeNs:             d.Metrics.WaitTimeNs,
			RunCount:               d.Metrics.RunCount,
			CPUMigrations:          d.Metrics.CPUMigrations,
		}
	}
	if d.Scaling != nil {
		item.Scaling = &PSMScalingDTO{
			Enabled:         d.Scaling.Enabled,
			MetricName:      d.Scaling.MetricName,
			TargetValue:     d.Scaling.TargetValue,
			MinReplicaCount: d.Scaling.MinReplicaCount,
			MaxReplicaCount: d.Scaling.MaxReplicaCount,
			CooldownPeriod:  d.Scaling.CooldownPeriod,
		}
		if d.Scaling.ScaleTargetRef != nil {
			item.Scaling.ScaleTargetRef = &PSMScaleTargetRefDTO{
				APIVersion: d.Scaling.ScaleTargetRef.APIVersion,
				Kind:       d.Scaling.ScaleTargetRef.Kind,
				Name:       d.Scaling.ScaleTargetRef.Name,
			}
		}
	}
	return item
}
