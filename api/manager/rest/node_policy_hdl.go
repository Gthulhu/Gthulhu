package rest

import (
	"net/http"

	"github.com/Gthulhu/api/manager/domain"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type DRASelectorPayload struct {
	DeviceClass string          `json:"deviceClass,omitempty"`
	Attributes  []LabelSelector `json:"attributes,omitempty"`
}

type CreateNodeSchedulingPolicyRequest struct {
	NodeSelectors []LabelSelector      `json:"nodeSelectors,omitempty"`
	NodeNames     []string             `json:"nodeNames,omitempty"`
	DRASelectors  []DRASelectorPayload `json:"draSelectors,omitempty"`
	CommandRegex  string               `json:"commandRegex,omitempty"`
	Priority      int                  `json:"priority,omitempty"`
	ExecutionTime int64                `json:"executionTime,omitempty"`
}

type UpdateNodeSchedulingPolicyRequest struct {
	PolicyID      string               `json:"policyId"`
	NodeSelectors []LabelSelector      `json:"nodeSelectors,omitempty"`
	NodeNames     []string             `json:"nodeNames,omitempty"`
	DRASelectors  []DRASelectorPayload `json:"draSelectors,omitempty"`
	CommandRegex  string               `json:"commandRegex,omitempty"`
	Priority      int                  `json:"priority,omitempty"`
	ExecutionTime int64                `json:"executionTime,omitempty"`
}

func convertRequestDRASelectorsToDomain(selectors []DRASelectorPayload) []domain.DRASelector {
	result := make([]domain.DRASelector, len(selectors))
	for i, s := range selectors {
		result[i] = domain.DRASelector{
			DeviceClass: s.DeviceClass,
			Attributes:  make([]domain.LabelSelector, len(s.Attributes)),
		}
		for j, a := range s.Attributes {
			result[i].Attributes[j] = domain.LabelSelector{Key: a.Key, Value: a.Value}
		}
	}
	return result
}

func convertRequestLabelSelectorsToDomain(selectors []LabelSelector) []domain.LabelSelector {
	result := make([]domain.LabelSelector, len(selectors))
	for i, s := range selectors {
		result[i] = domain.LabelSelector{Key: s.Key, Value: s.Value}
	}
	return result
}

// CreateNodeSchedulingPolicy godoc
// @Summary Create node scheduling policy
// @Description Create a new node-level scheduling policy.
// @Tags NodePolicies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body CreateNodeSchedulingPolicyRequest true "Node scheduling policy payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/node-scheduling-policies [post]
func (h *Handler) CreateNodeSchedulingPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req CreateNodeSchedulingPolicyRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}

	policy := &domain.NodeSchedulingPolicy{
		NodeSelectors: convertRequestLabelSelectorsToDomain(req.NodeSelectors),
		NodeNames:     req.NodeNames,
		DRASelectors:  convertRequestDRASelectorsToDomain(req.DRASelectors),
		CommandRegex:  req.CommandRegex,
		Priority:      req.Priority,
		ExecutionTime: req.ExecutionTime,
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.Svc.CreateNodeSchedulingPolicy(ctx, &claims, policy); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

// UpdateNodeSchedulingPolicy godoc
// @Summary Update node scheduling policy
// @Description Update an existing node-level scheduling policy.
// @Tags NodePolicies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body UpdateNodeSchedulingPolicyRequest true "Node scheduling policy payload"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/node-scheduling-policies [put]
func (h *Handler) UpdateNodeSchedulingPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req UpdateNodeSchedulingPolicyRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if req.PolicyID == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Policy ID is required", nil)
		return
	}

	policy := &domain.NodeSchedulingPolicy{
		NodeSelectors: convertRequestLabelSelectorsToDomain(req.NodeSelectors),
		NodeNames:     req.NodeNames,
		DRASelectors:  convertRequestDRASelectorsToDomain(req.DRASelectors),
		CommandRegex:  req.CommandRegex,
		Priority:      req.Priority,
		ExecutionTime: req.ExecutionTime,
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.Svc.UpdateNodeSchedulingPolicy(ctx, &claims, req.PolicyID, policy); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[string](nil)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type NodeSchedulingPolicyResponse struct {
	ID            bson.ObjectID        `json:"id"`
	NodeSelectors []LabelSelector      `json:"nodeSelectors,omitempty"`
	NodeNames     []string             `json:"nodeNames,omitempty"`
	DRASelectors  []DRASelectorPayload `json:"draSelectors,omitempty"`
	CommandRegex  string               `json:"commandRegex,omitempty"`
	Priority      int                  `json:"priority,omitempty"`
	ExecutionTime int64                `json:"executionTime,omitempty"`
}

func convertDomainNodePolicyToResponse(p *domain.NodeSchedulingPolicy) *NodeSchedulingPolicyResponse {
	attrs := make([]DRASelectorPayload, len(p.DRASelectors))
	for i, s := range p.DRASelectors {
		attrs[i] = DRASelectorPayload{
			DeviceClass: s.DeviceClass,
			Attributes:  convertDomainLabelSelectorsToResponseLabelSelectors(s.Attributes),
		}
	}
	return &NodeSchedulingPolicyResponse{
		ID:            p.ID,
		NodeSelectors: convertDomainLabelSelectorsToResponseLabelSelectors(p.NodeSelectors),
		NodeNames:     p.NodeNames,
		DRASelectors:  attrs,
		CommandRegex:  p.CommandRegex,
		Priority:      p.Priority,
		ExecutionTime: p.ExecutionTime,
	}
}

type ListNodeSchedulingPoliciesResponse struct {
	Policies []*NodeSchedulingPolicyResponse `json:"policies"`
}

// ListSelfNodeSchedulingPolicies godoc
// @Summary List self node scheduling policies
// @Description List node scheduling policies created by the authenticated user.
// @Tags NodePolicies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListNodeSchedulingPoliciesResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/node-scheduling-policies/self [get]
func (h *Handler) ListSelfNodeSchedulingPolicies(w http.ResponseWriter, r *http.Request) {
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
	queryOpt := &domain.QueryNodePolicyOptions{
		CreatorIDs: []bson.ObjectID{uid},
	}

	if err := h.Svc.ListNodeSchedulingPolicies(ctx, queryOpt); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := ListNodeSchedulingPoliciesResponse{
		Policies: make([]*NodeSchedulingPolicyResponse, len(queryOpt.Result)),
	}
	for i, p := range queryOpt.Result {
		resp.Policies[i] = convertDomainNodePolicyToResponse(p)
	}
	response := NewSuccessResponse[ListNodeSchedulingPoliciesResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type NodeSchedulingIntentResponse struct {
	ID            bson.ObjectID      `json:"id"`
	PolicyID      bson.ObjectID      `json:"policyId"`
	NodeID        string             `json:"nodeId"`
	CommandRegex  string             `json:"commandRegex,omitempty"`
	Priority      int                `json:"priority,omitempty"`
	ExecutionTime int64              `json:"executionTime,omitempty"`
	State         domain.IntentState `json:"state,omitempty"`
}

type ListNodeSchedulingIntentsResponse struct {
	Intents []*NodeSchedulingIntentResponse `json:"intents"`
}

// ListSelfNodeSchedulingIntents godoc
// @Summary List self node scheduling intents
// @Description List node scheduling intents created by the authenticated user.
// @Tags NodePolicies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[ListNodeSchedulingIntentsResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/node-scheduling-intents/self [get]
func (h *Handler) ListSelfNodeSchedulingIntents(w http.ResponseWriter, r *http.Request) {
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
	queryOpt := &domain.QueryNodeIntentOptions{
		CreatorIDs: []bson.ObjectID{uid},
	}

	if err := h.Svc.ListNodeSchedulingIntents(ctx, queryOpt); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	resp := ListNodeSchedulingIntentsResponse{
		Intents: make([]*NodeSchedulingIntentResponse, len(queryOpt.Result)),
	}
	for i, in := range queryOpt.Result {
		resp.Intents[i] = &NodeSchedulingIntentResponse{
			ID:            in.ID,
			PolicyID:      in.PolicyID,
			NodeID:        in.NodeID,
			CommandRegex:  in.CommandRegex,
			Priority:      in.Priority,
			ExecutionTime: in.ExecutionTime,
			State:         in.State,
		}
	}
	response := NewSuccessResponse[ListNodeSchedulingIntentsResponse](&resp)
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type DeleteNodeSchedulingPolicyRequest struct {
	PolicyID string `json:"policyId"`
}

// DeleteNodeSchedulingPolicy godoc
// @Summary Delete node scheduling policy
// @Description Delete a node scheduling policy and its associated intents.
// @Tags NodePolicies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeleteNodeSchedulingPolicyRequest true "Policy ID to delete"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/node-scheduling-policies [delete]
func (h *Handler) DeleteNodeSchedulingPolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req DeleteNodeSchedulingPolicyRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request body", err)
		return
	}
	if req.PolicyID == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Policy ID is required", nil)
		return
	}

	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		h.ErrorResponse(ctx, w, http.StatusUnauthorized, "Unauthorized", nil)
		return
	}

	if err := h.Svc.DeleteNodeSchedulingPolicy(ctx, &claims, req.PolicyID); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[EmptyResponse](&EmptyResponse{})
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type DeleteNodeSchedulingIntentsRequest struct {
	IntentIDs []string `json:"intentIds"`
}

// DeleteNodeSchedulingIntents godoc
// @Summary Delete node scheduling intents
// @Description Delete one or more node scheduling intents.
// @Tags NodePolicies
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param request body DeleteNodeSchedulingIntentsRequest true "Intent IDs to delete"
// @Success 200 {object} SuccessResponse[EmptyResponse]
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 403 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/node-scheduling-intents [delete]
func (h *Handler) DeleteNodeSchedulingIntents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req DeleteNodeSchedulingIntentsRequest
	if err := h.JSONBind(r, &req); err != nil {
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

	if err := h.Svc.DeleteNodeSchedulingIntents(ctx, &claims, req.IntentIDs); err != nil {
		h.HandleError(ctx, w, err)
		return
	}

	response := NewSuccessResponse[EmptyResponse](&EmptyResponse{})
	h.JSONResponse(ctx, w, http.StatusOK, response)
}
