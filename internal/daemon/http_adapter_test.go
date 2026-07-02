package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockRuntimeConfigStore struct {
	applyChanged bool
	applyErr     error
	applyCalls   int
	lastReq      runtimeConfigRequest
}

func (m *mockRuntimeConfigStore) InitializeRuntimeConfig(_, _ string) error {
	return nil
}

func (m *mockRuntimeConfigStore) ApplyRuntimeConfig(_, _ string, req runtimeConfigRequest) (bool, error) {
	m.applyCalls++
	m.lastReq = req
	if m.applyErr != nil {
		return false, m.applyErr
	}
	return m.applyChanged, nil
}

func newTestHandler(state *controlState, store RuntimeConfigStore, restartReqCh chan<- struct{}) *controlAPIHandler {
	return &controlAPIHandler{
		state:             state,
		runtimeConfigPath: "/tmp/runtime.yaml",
		schedulerBinPath:  "/tmp/gthulhu",
		runtimeStore:      store,
		restartReqCh:      restartReqCh,
	}
}

func TestControlAPI_StatusMethodNotAllowed(t *testing.T) {
	state := &controlState{}
	store := &mockRuntimeConfigStore{}
	restartReqCh := make(chan struct{}, 1)
	h := newTestHandler(state, store, restartReqCh)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/status", nil)
	rr := httptest.NewRecorder()
	h.routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusMethodNotAllowed)
	}
}

func TestControlAPI_RuntimeConfigSameVersionNoop(t *testing.T) {
	state := &controlState{}
	state.set("v1", true)
	store := &mockRuntimeConfigStore{}
	restartReqCh := make(chan struct{}, 1)
	h := newTestHandler(state, store, restartReqCh)

	body := runtimeConfigRequest{ConfigVersion: "v1"}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime-config", bytes.NewReader(buf))
	rr := httptest.NewRecorder()
	h.routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusOK)
	}
	if store.applyCalls != 0 {
		t.Fatalf("applyCalls=%d, want 0", store.applyCalls)
	}
	select {
	case <-restartReqCh:
		t.Fatalf("unexpected restart signal on noop request")
	default:
	}
}

func TestControlAPI_RuntimeConfigApplyChangedTriggersRestart(t *testing.T) {
	state := &controlState{}
	store := &mockRuntimeConfigStore{applyChanged: true}
	restartReqCh := make(chan struct{}, 1)
	h := newTestHandler(state, store, restartReqCh)

	body := runtimeConfigRequest{ConfigVersion: "v2", Mode: "gthulhu", SchedulerEnabled: true}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime-config", bytes.NewReader(buf))
	rr := httptest.NewRecorder()
	h.routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusOK)
	}
	if store.applyCalls != 1 {
		t.Fatalf("applyCalls=%d, want 1", store.applyCalls)
	}
	if store.lastReq.ConfigVersion != "v2" {
		t.Fatalf("lastReq.ConfigVersion=%q, want v2", store.lastReq.ConfigVersion)
	}
	select {
	case <-restartReqCh:
	default:
		t.Fatalf("expected restart signal but channel was empty")
	}
}

func TestControlAPI_RuntimeConfigApplyError(t *testing.T) {
	state := &controlState{}
	store := &mockRuntimeConfigStore{applyErr: errors.New("apply failed")}
	restartReqCh := make(chan struct{}, 1)
	h := newTestHandler(state, store, restartReqCh)

	body := runtimeConfigRequest{ConfigVersion: "v3"}
	buf, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/runtime-config", bytes.NewReader(buf))
	rr := httptest.NewRecorder()
	h.routes().ServeHTTP(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Fatalf("status=%d, want %d", rr.Code, http.StatusInternalServerError)
	}

	state.mu.RLock()
	lastErr := state.lastError
	state.mu.RUnlock()
	if lastErr == "" {
		t.Fatalf("expected lastError to be set")
	}

	select {
	case <-restartReqCh:
		t.Fatalf("unexpected restart signal when apply failed")
	default:
	}
}
