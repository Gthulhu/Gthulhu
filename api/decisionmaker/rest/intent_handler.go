package rest

import (
	"net/http"
	"time"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/decisionmaker/service"
)

type HandleIntentsRequest struct {
	Intents []Intent `json:"intents"`
}

type Intent struct {
	PodName       string            `json:"podName,omitempty"`
	PodID         string            `json:"podID,omitempty"`
	NodeID        string            `json:"nodeID,omitempty"`
	K8sNamespace  string            `json:"k8sNamespace,omitempty"`
	CommandRegex  string            `json:"commandRegex,omitempty"`
	Priority      int               `json:"priority,omitempty"`
	ExecutionTime int64             `json:"executionTime,omitempty"`
	PodLabels     map[string]string `json:"podLabels,omitempty"`
}

func (h *Handler) HandleIntents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req HandleIntentsRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}
	intents := make([]*domain.Intent, 0, len(req.Intents))
	for _, intent := range req.Intents {
		intents = append(intents, &domain.Intent{
			PodName:       intent.PodName,
			PodID:         intent.PodID,
			NodeID:        intent.NodeID,
			K8sNamespace:  intent.K8sNamespace,
			CommandRegex:  intent.CommandRegex,
			Priority:      intent.Priority,
			ExecutionTime: intent.ExecutionTime,
			PodLabels:     intent.PodLabels,
		})
	}
	err = h.Service.ProcessIntents(r.Context(), intents)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Failed to process intents", err)
		return
	}
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](nil))
}

// SchedulingStrategy represents a strategy for process scheduling
type SchedulingIntents struct {
	Priority      int             `json:"priority"`                // Priority value; higher value means higher priority
	ExecutionTime uint64          `json:"execution_time"`          // Time slice for this process in nanoseconds
	PID           int             `json:"pid,omitempty"`           // Process ID to apply this strategy to
	Selectors     []LabelSelector `json:"selectors,omitempty"`     // Label selectors to match pods
	CommandRegex  string          `json:"command_regex,omitempty"` // Regex to match process command
}

// LabelSelector represents a key-value pair for pod label selection
type LabelSelector struct {
	Key   string `json:"key"`   // Label key
	Value string `json:"value"` // Label value
}

type ListIntentsResponse struct {
	Success    bool                 `json:"success"`
	Message    string               `json:"message"`
	Timestamp  string               `json:"timestamp"`
	Scheduling []*SchedulingIntents `json:"scheduling"`
}

func (h *Handler) ListIntents(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	intents, err := h.Service.ListAllSchedulingIntents(ctx)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Failed to list scheduling intents", err)
		return
	}

	schedulingIntents := make([]*SchedulingIntents, 0, len(intents))
	for _, intent := range intents {
		schedulingIntents = append(schedulingIntents, &SchedulingIntents{
			Priority:      intent.Priority,
			ExecutionTime: intent.ExecutionTime,
			PID:           intent.PID,
			Selectors:     convertMapToLabelSelectors(intent.Selectors),
			CommandRegex:  intent.CommandRegex,
		})
	}

	response := ListIntentsResponse{
		Success:    true,
		Message:    "Scheduling intents retrieved successfully",
		Timestamp:  time.Now().UTC().Format(time.RFC3339), // You can set the current timestamp here if needed
		Scheduling: schedulingIntents,
	}
	h.JSONResponse(ctx, w, http.StatusOK, response)
}

type MerkleRootResponse struct {
	RootHash string `json:"rootHash"`
}

func (h *Handler) GetIntentMerkleRoot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	resp, err := h.Service.TraverseIntentMerkleTree(ctx, &service.TraverseIntentMerkleTreeOptions{
		Depth: 0,
	})
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Failed to get intent merkle root", err)
		return
	}
	rootHash := ""
	if resp != nil && resp.RootNode != nil {
		rootHash = resp.RootNode.Hash
	}
	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(&MerkleRootResponse{RootHash: rootHash}))
}

func convertMapToLabelSelectors(selectorMap []domain.LabelSelector) []LabelSelector {
	labelSelectors := make([]LabelSelector, 0, len(selectorMap))
	for _, sel := range selectorMap {
		labelSelectors = append(labelSelectors, LabelSelector{
			Key:   sel.Key,
			Value: sel.Value,
		})
	}
	return labelSelectors
}

type DeleteIntentRequest struct {
	PodID string `json:"podId,omitempty"` // If provided, deletes all intents for this pod
	PID   *int   `json:"pid,omitempty"`   // If provided with PodID, deletes specific intent
	All   bool   `json:"all,omitempty"`   // If true, deletes all intents
}

func (h *Handler) DeleteIntent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req DeleteIntentRequest
	err := h.JSONBind(r, &req)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "Invalid request payload", err)
		return
	}

	if req.All {
		err = h.Service.DeleteAllIntents(ctx)
		if err != nil {
			h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Failed to delete all intents", err)
			return
		}
		h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](nil))
		return
	}

	if req.PodID == "" {
		h.ErrorResponse(ctx, w, http.StatusBadRequest, "PodID is required when 'all' is false", nil)
		return
	}

	if req.PID != nil {
		err = h.Service.DeleteIntentByPID(ctx, req.PodID, *req.PID)
	} else {
		err = h.Service.DeleteIntentByPodID(ctx, req.PodID)
	}

	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Failed to delete intent", err)
		return
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse[EmptyResponse](nil))
}
