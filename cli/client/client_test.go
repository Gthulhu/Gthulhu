// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestGetStrategies(t *testing.T) {
	expected := SchedulingStrategiesResponse{
		Success: true,
		Scheduling: []SchedulingStrategy{
			{Priority: 1, ExecutionTime: 20000000, PID: 1234},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scheduling/strategies" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodGet {
			t.Errorf("unexpected method: %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", false)
	resp, err := c.GetStrategies()
	if err != nil {
		t.Fatalf("GetStrategies failed: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if len(resp.Scheduling) != 1 {
		t.Fatalf("expected 1 strategy, got %d", len(resp.Scheduling))
	}
	if resp.Scheduling[0].PID != 1234 {
		t.Errorf("expected PID=1234, got %d", resp.Scheduling[0].PID)
	}
}

func TestSetStrategies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/scheduling/strategies" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.Method != http.MethodPost {
			t.Errorf("unexpected method: %s", r.Method)
		}
		var req SchedulingStrategiesRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Strategies) != 1 {
			t.Fatalf("expected 1 strategy in request, got %d", len(req.Strategies))
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(SchedulingStrategiesResponse{Success: true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", false)
	resp, err := c.SetStrategies(&SchedulingStrategiesRequest{
		Strategies: []StrategyInput{{Priority: true, ExecutionTime: 5000000}},
	})
	if err != nil {
		t.Fatalf("SetStrategies failed: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
}

func TestGetMetrics(t *testing.T) {
	expected := MetricsResponse{
		Success: true,
		Data: &BssData{
			NrQueued:     10,
			NrOnlineCpus: 8,
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/metrics" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", false)
	resp, err := c.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics failed: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if resp.Data == nil {
		t.Fatal("expected non-nil Data")
	}
	if resp.Data.NrQueued != 10 {
		t.Errorf("expected NrQueued=10, got %d", resp.Data.NrQueued)
	}
}

func TestGetPodPIDs(t *testing.T) {
	expected := PodPIDsResponse{
		Success: true,
		Data: []PodPIDEntry{
			{PodName: "pod-1", Namespace: "default", PID: 42},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/pods/pids" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(expected)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "", false)
	resp, err := c.GetPodPIDs()
	if err != nil {
		t.Fatalf("GetPodPIDs failed: %v", err)
	}
	if !resp.Success {
		t.Error("expected success=true")
	}
	if len(resp.Data) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resp.Data))
	}
	if resp.Data[0].PID != 42 {
		t.Errorf("expected PID=42, got %d", resp.Data[0].PID)
	}
}

func TestAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/auth/token" {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(TokenResponse{
				Success: true,
				Data:    TokenData{Token: "test-token", ExpiredAt: 9999999999},
			})
			return
		}
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(MetricsResponse{Success: true})
	}))
	defer srv.Close()

	// Create a client with auth enabled but skip public key validation
	// by injecting a token directly.
	c := NewClient(srv.URL, "", true)
	c.token = "manual-token"
	c.tokenExpiresAt = time.Now().Add(1 * time.Hour)

	_, err := c.GetMetrics()
	if err != nil {
		t.Fatalf("GetMetrics with auth failed: %v", err)
	}
	if gotAuth != "Bearer manual-token" {
		t.Errorf("expected Bearer auth header, got %q", gotAuth)
	}
}

func TestNewClient(t *testing.T) {
	c := NewClient("http://example.com/", "key.pem", true)
	if c.baseURL != "http://example.com" {
		t.Errorf("expected trailing slash trimmed, got %q", c.baseURL)
	}
	if c.publicKeyPath != "key.pem" {
		t.Errorf("expected publicKeyPath=key.pem, got %q", c.publicKeyPath)
	}
	if !c.authEnabled {
		t.Error("expected authEnabled=true")
	}
}
