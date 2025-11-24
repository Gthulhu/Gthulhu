package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"time"
)

// Client handles JWT authentication with Gthulhu API
type Client struct {
	apiEndpoint   string
	publicKeyPath string
	token         string
	tokenExpiry   time.Time
	httpClient    *http.Client
	mu            sync.RWMutex
}

// TokenRequest represents the request structure for JWT token generation
type TokenRequest struct {
	PublicKey string `json:"public_key"` // PEM encoded public key
}

// TokenResponse represents the response structure for JWT token generation
type TokenResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
	Token     string `json:"token,omitempty"`
}

// NewClient creates a new authentication client
func NewClient(apiEndpoint, publicKeyPath string) *Client {
	return &Client{
		apiEndpoint:   apiEndpoint,
		publicKeyPath: publicKeyPath,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetToken returns a valid JWT token, refreshing if necessary
func (c *Client) GetToken() (string, error) {
	c.mu.RLock()
	// Check if token is still valid (with 5 minute buffer)
	if c.token != "" && time.Now().Before(c.tokenExpiry.Add(-5*time.Minute)) {
		token := c.token
		c.mu.RUnlock()
		return token, nil
	}
	c.mu.RUnlock()

	// Need to refresh token
	return c.refreshToken()
}

// refreshToken obtains a new JWT token from the API
func (c *Client) refreshToken() (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Double-check after acquiring lock
	if c.token != "" && time.Now().Before(c.tokenExpiry.Add(-5*time.Minute)) {
		return c.token, nil
	}

	// Read public key
	publicKeyBytes, err := os.ReadFile(c.publicKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key: %w", err)
	}

	// Prepare request
	reqBody := TokenRequest{
		PublicKey: string(publicKeyBytes),
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send token request
	tokenURL := c.apiEndpoint + "/api/v1/auth/token"
	req, err := http.NewRequest("POST", tokenURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if !tokenResp.Success || tokenResp.Token == "" {
		return "", fmt.Errorf("token generation failed: %s", tokenResp.Message)
	}

	// Store token (assume 24 hour expiry, will be refreshed 5 min before)
	c.token = tokenResp.Token
	c.tokenExpiry = time.Now().Add(24 * time.Hour)

	return c.token, nil
}
