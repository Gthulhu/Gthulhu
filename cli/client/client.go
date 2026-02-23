// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

// Package client provides an HTTP client for the Gthulhu API server.
// It handles JWT-based authentication and exposes methods that correspond
// to each API endpoint (strategies, metrics, pods, auth).
package client

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
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
	publicKeyPath  string
	authEnabled    bool
}

// NewClient creates a new API client targeting the given base URL.
func NewClient(baseURL, publicKeyPath string, authEnabled bool) *Client {
	return &Client{
		baseURL:       strings.TrimSuffix(baseURL, "/"),
		publicKeyPath: publicKeyPath,
		authEnabled:   authEnabled,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ---------------------------------------------------------------------------
// Authentication
// ---------------------------------------------------------------------------

// RequestToken obtains a JWT token from the API server and returns the
// full TokenResponse so callers can inspect the token and expiry.
func (c *Client) RequestToken() (*TokenResponse, error) {
	publicKeyPEM, err := loadPublicKey(c.publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load public key: %w", err)
	}

	body, err := json.Marshal(TokenRequest{PublicKey: publicKeyPEM})
	if err != nil {
		return nil, fmt.Errorf("marshal token request: %w", err)
	}

	resp, err := c.httpClient.Post(
		c.baseURL+"/api/v1/auth/token",
		"application/json",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("send token request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if json.Unmarshal(respBody, &errResp) == nil && errResp.Error != "" {
			return nil, fmt.Errorf("token request failed: %s", errResp.Error)
		}
		return nil, fmt.Errorf("token request failed (HTTP %d): %s", resp.StatusCode, string(respBody))
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(respBody, &tokenResp); err != nil {
		return nil, fmt.Errorf("unmarshal token response: %w", err)
	}
	if !tokenResp.Success || tokenResp.Data.Token == "" {
		return nil, fmt.Errorf("token request unsuccessful")
	}

	c.token = tokenResp.Data.Token
	c.tokenExpiresAt = time.Unix(tokenResp.Data.ExpiredAt, 0)
	return &tokenResp, nil
}

// ensureValidToken refreshes the JWT token when it is missing or expired.
func (c *Client) ensureValidToken() error {
	if c.token == "" || time.Now().After(c.tokenExpiresAt) {
		if _, err := c.RequestToken(); err != nil {
			return fmt.Errorf("obtain JWT token: %w", err)
		}
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

// GetStrategies fetches the current scheduling strategies from the API.
func (c *Client) GetStrategies() (*SchedulingStrategiesResponse, error) {
	resp, err := c.doRequest("GET", c.baseURL+"/api/v1/scheduling/strategies", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result SchedulingStrategiesResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// SetStrategies posts new scheduling strategies to the API.
func (c *Client) SetStrategies(strategies *SchedulingStrategiesRequest) (*SchedulingStrategiesResponse, error) {
	data, err := json.Marshal(strategies)
	if err != nil {
		return nil, fmt.Errorf("marshal strategies: %w", err)
	}

	resp, err := c.doRequest("POST", c.baseURL+"/api/v1/scheduling/strategies", bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result SchedulingStrategiesResponse
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
// Pods
// ---------------------------------------------------------------------------

// GetPodPIDs retrieves the pod-to-PID mapping from the API.
func (c *Client) GetPodPIDs() (*PodPIDsResponse, error) {
	resp, err := c.doRequest("GET", c.baseURL+"/api/v1/pods/pids", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result PodPIDsResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &result, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// loadPublicKey reads and validates a PEM-encoded RSA public key from disk.
func loadPublicKey(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read public key file: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}
	if _, err := x509.ParsePKIXPublicKey(block.Bytes); err != nil {
		return "", fmt.Errorf("parse public key: %w", err)
	}
	return string(data), nil
}
