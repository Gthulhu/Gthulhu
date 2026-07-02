package daemon

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
)

type controlAPIHandler struct {
	state             *controlState
	runtimeConfigPath string
	schedulerBinPath  string
	runtimeStore      RuntimeConfigStore
	restartReqCh      chan<- struct{}
}

func startControlServer(addr, runtimeConfigPath string, schedulerBinPath string, state *controlState, runtimeStore RuntimeConfigStore, restartReqCh chan<- struct{}) error {
	h := &controlAPIHandler{
		state:             state,
		runtimeConfigPath: runtimeConfigPath,
		schedulerBinPath:  schedulerBinPath,
		runtimeStore:      runtimeStore,
		restartReqCh:      restartReqCh,
	}
	server := &http.Server{Addr: addr, Handler: h.routes()}
	go func() {
		slog.Info("daemon control server started", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("daemon control server exited", "error", err)
		}
	}()
	return nil
}

func (h *controlAPIHandler) routes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.handleHealth)
	mux.HandleFunc("/api/v1/status", h.handleStatus)
	mux.HandleFunc("/api/v1/runtime-config", h.handleRuntimeConfig)
	return mux
}

func (h *controlAPIHandler) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (h *controlAPIHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"success": false, "error": "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": h.state.detailedSnapshot()})
}

func (h *controlAPIHandler) handleRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": h.state.snapshot()})
		return
	case http.MethodPost:
		body, err := io.ReadAll(r.Body)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request body"})
			return
		}
		var req runtimeConfigRequest
		if err := json.Unmarshal(body, &req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "invalid request payload"})
			return
		}
		if req.ConfigVersion == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"success": false, "error": "configVersion is required"})
			return
		}

		h.state.mu.RLock()
		sameVersion := h.state.applied && h.state.configVersion == req.ConfigVersion
		h.state.mu.RUnlock()
		if sameVersion {
			writeJSON(w, http.StatusOK, map[string]any{"success": true, "noop": true})
			return
		}

		changed, err := h.runtimeStore.ApplyRuntimeConfig(h.runtimeConfigPath, h.schedulerBinPath, req)
		if err != nil {
			slog.ErrorContext(ctx, "failed to apply runtime config", "error", err)
			errMsg := err.Error()
			h.state.recordError(errMsg)
			writeJSON(w, http.StatusInternalServerError, map[string]any{"success": false, "error": errMsg})
			return
		}
		h.state.set(req.ConfigVersion, true)
		if changed {
			select {
			case h.restartReqCh <- struct{}{}:
			default:
			}
		} else {
			slog.Info("runtime config request matched current state, skipping restart", "configVersion", req.ConfigVersion)
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "noop": !changed})
		return
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"success": false, "error": "method not allowed"})
		return
	}
}

func writeJSON(w http.ResponseWriter, status int, payload map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
