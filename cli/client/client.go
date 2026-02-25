// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

// Package client provides an HTTP client for the Gthulhu API server.
// It handles JWT-based authentication and exposes methods that correspond
// to each API endpoint (strategies, metrics, pods, auth).
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// Client is the Gthulhu API client.
type Client struct {
	baseURL    string
	httpClient *http.Client

	// JWT authentication state
	token          string
	tokenExpiresAt time.Time
	authEnabled    bool
}

// NewClient creates a new API client targeting the given base URL.
func NewClient(baseURL string, authEnabled bool) *Client {
	c := &Client{
		baseURL:     strings.TrimSuffix(baseURL, "/"),
		authEnabled: authEnabled,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	// Try to load existing token from storage
	if authEnabled {
		token, expiresAt, err := LoadToken()
		if err == nil && token != "" {
			c.token = token
			c.tokenExpiresAt = expiresAt
		}
	}

	return c
}

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

// Login authenticates using username and password and returns the JWT token.
func (c *Client) Login(username, password string) (*LoginResponse, error) {
	body, err := json.Marshal(LoginRequest{
		Username: username,
		Password: password,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal login request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/auth/login",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("send login request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("login failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("login failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var loginResp LoginResponse
	if err := json.Unmarshal(respBody, &loginResp); err != nil {
		return nil, fmt.Errorf("unmarshal login response: %w", err)
	}
	if !loginResp.Success || loginResp.Data.Token == "" {
		return nil, fmt.Errorf("login unsuccessful")
	}

	c.token = loginResp.Data.Token
	// For login, we don't have an expiration time in the response,
	// so we'll set a reasonable default (e.g., 24 hours from now)
	c.tokenExpiresAt = time.Now().Add(24 * time.Hour)

	// Save token to disk
	if err := SaveToken(c.token, c.tokenExpiresAt); err != nil {
		// Log warning but don't fail the login
		fmt.Fprintf(os.Stderr, "Warning: failed to save token: %v\n", err)
	}

	return &loginResp, nil
}

// Deprecated: RequestToken uses public key authentication which is not supported in Manager Mode.
// Use Login() with username/password instead.
func (c *Client) RequestToken() (*TokenResponse, error) {
	return nil, fmt.Errorf("public key authentication is not supported in Manager Mode, please use 'auth login' with username/password")
}

// ensureValidToken checks if the JWT token is present and not expired.
// If token is missing or expired, returns an error asking user to login.
func (c *Client) ensureValidToken() error {
	if c.token == "" || time.Now().After(c.tokenExpiresAt) {
		return fmt.Errorf("authentication required: please run 'gthulhu-cli auth login' to obtain a token")
	}
	return nil
}

// doRequest performs an authenticated HTTP request.
func (c *Client) doRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	if c.authEnabled {
		if err := c.ensureValidToken(); err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	return c.httpClient.Do(req)
}

// ---------------------------------------------------------------------------
// Strategies
// ---------------------------------------------------------------------------

// GetStrategies fetches the current scheduling strategies created by the authenticated user.
func (c *Client) GetStrategies() (*ListSchedulerStrategiesResponse, error) {
	resp, err := c.doRequest("GET", c.baseURL+"/api/v1/strategies/self", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("get strategies failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("get strategies failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result ListSchedulerStrategiesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// CreateStrategy creates a new scheduling strategy.
func (c *Client) CreateStrategy(req *CreateScheduleStrategyRequest) (*EmptyDataResponse, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doRequest("POST", c.baseURL+"/api/v1/strategies", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("create strategy failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("create strategy failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result EmptyDataResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// DeleteStrategy deletes a scheduling strategy.
func (c *Client) DeleteStrategy(strategyID string) (*EmptyDataResponse, error) {
	req := DeleteScheduleStrategyRequest{
		StrategyID: strategyID,
	}

	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	resp, err := c.doRequest("DELETE", c.baseURL+"/api/v1/strategies", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("delete strategy failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("delete strategy failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result EmptyDataResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Metrics
// ---------------------------------------------------------------------------

// GetMetrics retrieves current scheduler metrics.
func (c *Client) GetMetrics() (*MetricsResponse, error) {
	resp, err := c.doRequest("GET", c.baseURL+"/api/v1/metrics", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result MetricsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Nodes
// ---------------------------------------------------------------------------

// ListNodes retrieves all Kubernetes nodes.
func (c *Client) ListNodes() (*ListNodesResponse, error) {
	resp, err := c.doRequest("GET", c.baseURL+"/api/v1/nodes", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("list nodes failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("list nodes failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result ListNodesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// GetNodePodPIDMapping retrieves pod-PID mappings for a specific node.
func (c *Client) GetNodePodPIDMapping(nodeID string) (*GetNodePodPIDMappingResponse, error) {
	url := fmt.Sprintf("%s/api/v1/nodes/%s/pods/pids", c.baseURL, nodeID)
	resp, err := c.doRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("get node pod-PID mapping failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("get node pod-PID mapping failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result GetNodePodPIDMappingResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Pods
// ---------------------------------------------------------------------------

// GetPodPIDs retrieves the pod-to-PID mapping from the API.
// Note: This endpoint (/api/v1/pods/pids) is from the decisionmaker service.
// In Manager Mode, consider using GetNodePodPIDMapping() instead.
func (c *Client) GetPodPIDs() (*GetPodsPIDsResponse, error) {
	resp, err := c.doRequest("GET", c.baseURL+"/api/v1/pods/pids", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("get pod PIDs failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("get pod PIDs failed (HTTP %d): %s", resp.StatusCode, string(body))
	}

	var result GetPodsPIDsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}
