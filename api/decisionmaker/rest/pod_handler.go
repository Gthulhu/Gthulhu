package rest

import (
	"net/http"
	"os"
	"time"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

// PodProcess represents a process information within a pod (for API response)
type PodProcess struct {
	PID         int    `json:"pid"`
	Command     string `json:"command"`
	PPID        int    `json:"ppid,omitempty"`
	ContainerID string `json:"container_id,omitempty"`
}

// PodInfo represents pod information with associated processes (for API response)
type PodInfo struct {
	PodUID    string       `json:"pod_uid"`
	PodID     string       `json:"pod_id,omitempty"`
	Processes []PodProcess `json:"processes"`
}

// GetPodsPIDsResponse is the response structure for the GET /api/v1/pods/pids endpoint
type GetPodsPIDsResponse struct {
	Pods      []PodInfo `json:"pods"`
	Timestamp string    `json:"timestamp"`
	NodeName  string    `json:"node_name"`
	NodeID    string    `json:"node_id,omitempty"`
}

// GetPodsPIDs godoc
// @Summary Get Pod to PID mappings
// @Description Returns all pods running on this node with their associated process IDs
// @Tags Pods
// @Produce json
// @Security BearerAuth
// @Success 200 {object} SuccessResponse[GetPodsPIDsResponse]
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/v1/pods/pids [get]
func (h *Handler) GetPodsPIDs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get all pod information from /proc filesystem
	podInfoMap, err := h.Service.GetAllPodInfos(ctx)
	if err != nil {
		h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Failed to retrieve pod information", err)
		return
	}

	// Convert map to slice for response
	pods := make([]PodInfo, 0, len(podInfoMap))
	for _, podInfo := range podInfoMap {
		processes := make([]PodProcess, 0, len(podInfo.Processes))
		for _, proc := range podInfo.Processes {
			processes = append(processes, PodProcess{
				PID:         proc.PID,
				Command:     proc.Command,
				PPID:        proc.PPID,
				ContainerID: proc.ContainerID,
			})
		}
		pods = append(pods, PodInfo{
			PodUID:    podInfo.PodUID,
			PodID:     podInfo.PodID,
			Processes: processes,
		})
	}

	// Get node name from hostname or environment variable
	nodeName, _ := os.Hostname()
	if envNodeName := os.Getenv("NODE_NAME"); envNodeName != "" {
		nodeName = envNodeName
	}

	response := GetPodsPIDsResponse{
		Pods:      pods,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		NodeName:  nodeName,
	}

	h.JSONResponse(ctx, w, http.StatusOK, NewSuccessResponse(&response))
}

// convertDomainPodProcess converts domain.PodProcess to rest.PodProcess
func convertDomainPodProcess(proc domain.PodProcess) PodProcess {
	return PodProcess{
		PID:         proc.PID,
		Command:     proc.Command,
		PPID:        proc.PPID,
		ContainerID: proc.ContainerID,
	}
}
