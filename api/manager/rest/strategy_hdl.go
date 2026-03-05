package rest

import (
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type LabelSelector struct {
	Key   string `json:"key,omitempty"`
	Value string `json:"value,omitempty"`
}

type CreateScheduleStrategyRequest struct {
	StrategyNamespace string          `json:"strategyNamespace,omitempty"`
	LabelSelectors    []LabelSelector `json:"labelSelectors,omitempty"`
	K8sNamespace      []string        `json:"k8sNamespace,omitempty"`
	CommandRegex      string          `json:"commandRegex,omitempty"`
	Priority          int             `json:"priority,omitempty"`
	ExecutionTime     int64           `json:"executionTime,omitempty"`
}

type UpdateScheduleStrategyRequest struct {
	StrategyID        string          `json:"strategyId"`
	StrategyNamespace string          `json:"strategyNamespace,omitempty"`
	LabelSelectors    []LabelSelector `json:"labelSelectors,omitempty"`
	K8sNamespace      []string        `json:"k8sNamespace,omitempty"`
	CommandRegex      string          `json:"commandRegex,omitempty"`
	Priority          int             `json:"priority,omitempty"`
	ExecutionTime     int64           `json:"executionTime,omitempty"`
}

// CreateScheduleStrategy godoc
// @Summary Create schedule strategy
// @Description Create a new schedule strategy.
// @Tags Strategies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateScheduleStrategyRequest true "Schedule strategy payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/strategies [post]
func (h *Handler) CreateScheduleStrategy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateScheduleStrategyRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	strategy := &domain.ScheduleStrategy{
		StrategyNamespace: req.StrategyNamespace,
		LabelSelectors:    make([]domain.LabelSelector, len(req.LabelSelectors)),
		K8sNamespace:      req.K8sNamespace,
		CommandRegex:      req.CommandRegex,
		Priority:          req.Priority,
		ExecutionTime:     req.ExecutionTime,
	}
	for i, ls := range req.LabelSelectors {
		strategy.LabelSelectors[i] = domain.LabelSelector{
			Key:   ls.Key,
			Value: ls.Value,
		}
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	err = h.Svc.CreateScheduleStrategy(ctx, &claims, strategy)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

// UpdateScheduleStrategy godoc
// @Summary Update schedule strategy
// @Description Update an existing schedule strategy.
// @Tags Strategies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateScheduleStrategyRequest true "Schedule strategy payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/strategies [put]
func (h *Handler) UpdateScheduleStrategy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req UpdateScheduleStrategyRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if req.StrategyID == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Strategy ID is required", nil)
		return
	}

	strategy := &domain.ScheduleStrategy{
		StrategyNamespace: req.StrategyNamespace,
		LabelSelectors:    make([]domain.LabelSelector, len(req.LabelSelectors)),
		K8sNamespace:      req.K8sNamespace,
		CommandRegex:      req.CommandRegex,
		Priority:          req.Priority,
		ExecutionTime:     req.ExecutionTime,
	}
	for i, ls := range req.LabelSelectors {
		strategy.LabelSelectors[i] = domain.LabelSelector{
			Key:   ls.Key,
			Value: ls.Value,
		}
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.Svc.UpdateScheduleStrategy(ctx, &claims, req.StrategyID, strategy); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type ListSchedulerStrategiesResponse struct {
	Strategies []*ScheduleStrategy `json:"strategies"`
}

type ScheduleStrategy struct {
	ID                bson.ObjectID   `bson:"_id,omitempty"`
	StrategyNamespace string          `bson:"strategyNamespace,omitempty"`
	LabelSelectors    []LabelSelector `bson:"labelSelectors,omitempty"`
	K8sNamespace      []string        `bson:"k8sNamespace,omitempty"`
	CommandRegex      string          `bson:"commandRegex,omitempty"`
	Priority          int             `bson:"priority,omitempty"`
	ExecutionTime     int64           `bson:"executionTime,omitempty"`
}

// ListSelfScheduleStrategies godoc
// @Summary List self schedule strategies
// @Description List schedule strategies created by the authenticated user.
// @Tags Strategies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListSchedulerStrategiesResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/strategies/self [get]
func (h *Handler) ListSelfScheduleStrategies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	uid, err := claims.GetBsonObjectUID()
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid user ID in token", err)
		return
	}
	queryOpt := &domain.QueryStrategyOptions{
		CreatorIDs: []bson.ObjectID{uid},
	}

	err = h.Svc.ListScheduleStrategies(ctx, queryOpt)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := ListSchedulerStrategiesResponse{
		Strategies: make([]*ScheduleStrategy, len(queryOpt.Result)),
	}
	for i, ds := range queryOpt.Result {
		resp.Strategies[i] = h.convertDomainStrategyToResponseStrategy(ds)
	}
	response := NewSuccessResponse[ListSchedulerStrategiesResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

func (h *Handler) convertDomainStrategyToResponseStrategy(domainStrategy *domain.ScheduleStrategy) *ScheduleStrategy {
	return &ScheduleStrategy{
		ID:                domainStrategy.ID,
		StrategyNamespace: domainStrategy.StrategyNamespace,
		LabelSelectors:    convertDomainLabelSelectorsToResponseLabelSelectors(domainStrategy.LabelSelectors),
		K8sNamespace:      domainStrategy.K8sNamespace,
		CommandRegex:      domainStrategy.CommandRegex,
		Priority:          domainStrategy.Priority,
		ExecutionTime:     domainStrategy.ExecutionTime,
	}
}

func convertDomainLabelSelectorsToResponseLabelSelectors(domainLabelSelectors []domain.LabelSelector) []LabelSelector {
	responseLabelSelectors := make([]LabelSelector, len(domainLabelSelectors))
	for i, dls := range domainLabelSelectors {
		responseLabelSelectors[i] = LabelSelector{
			Key:   dls.Key,
			Value: dls.Value,
		}
	}
	return responseLabelSelectors
}

type ListScheduleIntentsResponse struct {
	Intents []*ScheduleIntent `json:"intents"`
}

type ScheduleIntent struct {
	ID            bson.ObjectID      `bson:"_id,omitempty"`
	StrategyID    bson.ObjectID      `bson:"strategyID,omitempty"`
	PodID         string             `bson:"podID,omitempty"`
	NodeID        string             `bson:"nodeID,omitempty"`
	K8sNamespace  string             `bson:"k8sNamespace,omitempty"`
	CommandRegex  string             `bson:"commandRegex,omitempty"`
	Priority      int                `bson:"priority,omitempty"`
	ExecutionTime int64              `bson:"executionTime,omitempty"`
	PodLabels     map[string]string  `bson:"podLabels,omitempty"`
	State         domain.IntentState `bson:"state,omitempty"`
}

// ListSelfScheduleIntents godoc
// @Summary List self schedule intents
// @Description List schedule intents created by the authenticated user.
// @Tags Strategies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListScheduleIntentsResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/intents/self [get]
func (h *Handler) ListSelfScheduleIntents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	uid, err := claims.GetBsonObjectUID()
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid user ID in token", err)
		return
	}
	queryOpt := &domain.QueryIntentOptions{
		CreatorIDs: []bson.ObjectID{uid},
	}

	err = h.Svc.ListScheduleIntents(ctx, queryOpt)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := ListScheduleIntentsResponse{
		Intents: make([]*ScheduleIntent, len(queryOpt.Result)),
	}
	for i, di := range queryOpt.Result {
		resp.Intents[i] = h.convertDomainIntentToResponseIntent(di)
	}
	response := NewSuccessResponse[ListScheduleIntentsResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

func (h *Handler) convertDomainIntentToResponseIntent(domainIntent *domain.ScheduleIntent) *ScheduleIntent {
	return &ScheduleIntent{
		ID:            domainIntent.ID,
		StrategyID:    domainIntent.StrategyID,
		PodID:         domainIntent.PodID,
		NodeID:        domainIntent.NodeID,
		K8sNamespace:  domainIntent.K8sNamespace,
		CommandRegex:  domainIntent.CommandRegex,
		Priority:      domainIntent.Priority,
		ExecutionTime: domainIntent.ExecutionTime,
		PodLabels:     domainIntent.PodLabels,
		State:         domainIntent.State,
	}
}

type DeleteScheduleStrategyRequest struct {
	StrategyID string `json:"strategyId"`
}

// DeleteScheduleStrategy godoc
// @Summary Delete schedule strategy
// @Description Delete a schedule strategy and its associated intents.
// @Tags Strategies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeleteScheduleStrategyRequest true "Strategy ID to delete"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/strategies [delete]
func (h *Handler) DeleteScheduleStrategy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req DeleteScheduleStrategyRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if req.StrategyID == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Strategy ID is required", nil)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	err = h.Svc.DeleteScheduleStrategy(ctx, &claims, req.StrategyID)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[EmptyResponse](&EmptyResponse{})
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type DeleteScheduleIntentsRequest struct {
	IntentIDs []string `json:"intentIds"`
}

// DeleteScheduleIntents godoc
// @Summary Delete schedule intents
// @Description Delete one or more schedule intents.
// @Tags Strategies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeleteScheduleIntentsRequest true "Intent IDs to delete"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/intents [delete]
func (h *Handler) DeleteScheduleIntents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req DeleteScheduleIntentsRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	if len(req.IntentIDs) == 0 {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "At least one intent ID is required", nil)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	err = h.Svc.DeleteScheduleIntents(ctx, &claims, req.IntentIDs)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[EmptyResponse](&EmptyResponse{})
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

// GetNodePodPIDMappingResponse is the response structure for the GET /api/v1/nodes/:nodeID/pods/pids endpoint
type GetNodePodPIDMappingResponse struct {
	Pods      []PodPIDInfo `json:"pods"`
	Timestamp string       `json:"timestamp"`
	NodeName  string       `json:"node_name"`
	NodeID    string       `json:"node_id,omitempty"`
}

// PodPIDInfo represents pod information with associated processes (for API response)
type PodPIDInfo struct {
	PodUID    string          `json:"pod_uid"`
	PodID     string          `json:"pod_id,omitempty"`
	Processes []PodPIDProcess `json:"processes"`
}

// PodPIDProcess represents a process information within a pod (for API response)
type PodPIDProcess struct {
	PID         int    `json:"pid"`
	Command     string `json:"command"`
	PPID        int    `json:"ppid,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
}

// GetNodePodPIDMapping godoc
// @Summary Get Pod-PID mapping for a specific node
// @Description Returns all pods running on the specified node with their associated process IDs
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Param nodeID path string true "Node ID"
// @Success 200 {object} SuccessResponse[GetNodePodPIDMappingResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/nodes/{nodeID}/pods/pids [get]
func (h *Handler) GetNodePodPIDMapping(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	nodeID := h.GetPathParam(r, "nodeID")
	if nodeID == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Node ID is required", nil)
		return
	}

	result, err := h.Svc.GetPodPIDMapping(ctx, nodeID)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	// Convert domain response to REST response
	resp := GetNodePodPIDMappingResponse{
		Pods:      make([]PodPIDInfo, len(result.Pods)),
		Timestamp: result.Timestamp,
		NodeName:  result.NodeName,
		NodeID:    result.NodeID,
	}
	for i, pod := range result.Pods {
		processes := make([]PodPIDProcess, len(pod.Processes))
		for j, proc := range pod.Processes {
			processes[j] = PodPIDProcess{
				PID:         proc.PID,
				Command:     proc.Command,
				PPID:        proc.PPID,
				ContainerID: proc.ContainerID,
			}
		}
		resp.Pods[i] = PodPIDInfo{
			PodUID:    pod.PodUID,
			PodID:     pod.PodID,
			Processes: processes,
		}
	}

	response := NewSuccessResponse[GetNodePodPIDMappingResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

// NodeInfo represents node information for API response
type NodeInfo struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

// ListNodesResponse is the response structure for the GET /api/v1/nodes endpoint
type ListNodesResponse struct {
	Nodes []NodeInfo `json:"nodes"`
}

// ListNodes godoc
// @Summary List all Kubernetes nodes
// @Description Returns all nodes in the Kubernetes cluster
// @Tags Nodes
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListNodesResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/nodes [get]
func (h *Handler) ListNodes(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	nodes, err := h.Svc.ListNodes(ctx)
	if err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := ListNodesResponse{
		Nodes: make([]NodeInfo, len(nodes)),
	}
	for i, node := range nodes {
		resp.Nodes[i] = NodeInfo{
			Name:   node.Name,
			Status: node.Status,
		}
	}

	response := NewSuccessResponse[ListNodesResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}
