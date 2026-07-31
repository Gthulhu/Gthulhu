package rest

import (
	"net/http"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

// HandleNodePoliciesRequest is the payload the Manager sends when pushing
// node-level scheduling policies to this Decision Maker.
type HandleNodePoliciesRequest struct {
	Policies []NodePolicy `json:"policies"`
}

// NodePolicy mirrors domain.NodePolicy over the wire.
type NodePolicy struct {
	PolicyID      string `json:"policyID,omitempty"`
	NodeID        string `json:"nodeID,omitempty"`
	CommandRegex  string `json:"commandRegex,omitempty"`
	Priority      int    `json:"priority,omitempty"`
	ExecutionTime int64  `json:"executionTime,omitempty"`
}

// HandleNodePolicies receives node-level scheduling policies from the
// Manager and stores them so that resolveNodeSchedulingIntents can match
// them against the node's process table.
func (h *Handler) HandleNodePolicies(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req HandleNodePoliciesRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	policies := make([]*domain.NodePolicy, 0, len(req.Policies))
	for _, policy := range req.Policies {
		policies = append(policies, &domain.NodePolicy{
			PolicyID:      policy.PolicyID,
			NodeID:        policy.NodeID,
			CommandRegex:  policy.CommandRegex,
			Priority:      policy.Priority,
			ExecutionTime: policy.ExecutionTime,
		})
	}

	if err := h.Service.ProcessNodePolicies(ctx, policies); err != nil {
		h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Failed to process node policies", err)
		return
	}
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](nil))
}

// GetNodePolicyMerkleRoot returns the merkle root hash of the currently
// cached node policies, used by the Manager to detect drift after a
// Decision Maker restart.
func (h *Handler) GetNodePolicyMerkleRoot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rootHash := h.Service.GetNodePolicyMerkleRootHash()
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(&MerkleRootResponse{RootHash: rootHash}))
}

// DeleteNodePolicyRequest describes a request to remove node policies from
// the Decision Maker's in-memory cache.
type DeleteNodePolicyRequest struct {
	PolicyID string `json:"policyId,omitempty"` // If provided, deletes only this policy.
	All      bool   `json:"all,omitempty"`      // If true, deletes all node policies.
}

// DeleteNodePolicy removes one or all node policies from the cache.
func (h *Handler) DeleteNodePolicy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req DeleteNodePolicyRequest
	if err := h.JSONBind(r, &req); err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	var err error
	if req.All {
		err = h.Service.DeleteAllNodePolicies(ctx)
	} else if req.PolicyID != "" {
		err = h.Service.DeleteNodePolicyByID(ctx, req.PolicyID)
	} else {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "policyId is required when 'all' is false", nil)
		return
	}
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Failed to delete node policy", err)
		return
	}
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](nil))
}
