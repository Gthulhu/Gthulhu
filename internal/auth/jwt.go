package auth

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

// ErrorResponse represents error response structure
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// JWTClient handles JWT authentication for API calls
type JWTClient struct {
	publicKeyPath  string
	apiBaseURL     string
	token          string
	tokenExpiresAt time.Time
	httpClient     *http.Client
}

// NewJWTClient creates a new JWT client
func NewJWTClient(publicKeyPath, apiBaseURL string) *JWTClient {
	return &JWTClient{
		publicKeyPath: publicKeyPath,
		apiBaseURL:    strings.TrimSuffix(apiBaseURL, "/"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// loadPublicKey loads the RSA public key from PEM file
func (c *JWTClient) loadPublicKey() (string, error) {
	keyData, err := os.ReadFile(c.publicKeyPath)
	if err != nil {
		return "", fmt.Errorf("failed to read public key file: %v", err)
	}

	// Verify it's a valid PEM format
	block, _ := pem.Decode(keyData)
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block containing public key")
	}

	// Verify it's a valid RSA public key
	_, err = x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse public key: %v", err)
	}

	return string(keyData), nil
}

// requestToken requests a JWT token from the API server
func (c *JWTClient) requestToken() error {
	publicKeyPEM, err := c.loadPublicKey()
	if err != nil {
		return fmt.Errorf("failed to load public key: %v", err)
	}

	// Prepare request
	request := TokenRequest{
		PublicKey: publicKeyPEM,
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return fmt.Errorf("failed to marshal token request: %v", err)
	}

	// Send request to token endpoint
	tokenURL := c.apiBaseURL + "/api/v1/auth/token"
	resp, err := c.httpClient.Post(tokenURL, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		return fmt.Errorf("failed to send token request: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response body: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errorResp ErrorResponse
		if err := json.Unmarshal(body, &errorResp); err != nil {
			return fmt.Errorf("token request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return fmt.Errorf("token request failed: %s", errorResp.Error)
	}

	var tokenResp TokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return fmt.Errorf("failed to unmarshal token response: %v", err)
	}

	if !tokenResp.Success || tokenResp.Token == "" {
		return fmt.Errorf("token request unsuccessful: %s", tokenResp.Message)
	}

	c.token = tokenResp.Token
	// Set token expiration to 23 hours from now (giving 1 hour buffer before actual expiration)
	c.tokenExpiresAt = time.Now().Add(23 * time.Hour)

	return nil
}

// ensureValidToken ensures we have a valid JWT token
func (c *JWTClient) ensureValidToken() error {
	// Check if we need to get a new token
	if c.token == "" || time.Now().After(c.tokenExpiresAt) {
		if err := c.requestToken(); err != nil {
			return fmt.Errorf("failed to obtain JWT token: %v", err)
		}
	}
	return nil
}

// GetAuthenticatedClient returns an HTTP client with JWT authentication
func (c *JWTClient) GetAuthenticatedClient() (*http.Client, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	// Create a custom transport that adds the Authorization header
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &authenticatedTransport{
			token:     c.token,
			transport: http.DefaultTransport,
		},
	}

	return client, nil
}

// MakeAuthenticatedRequest makes an HTTP request with JWT authentication
func (c *JWTClient) MakeAuthenticatedRequest(method, url string, body io.Reader) (*http.Response, error) {
	if err := c.ensureValidToken(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}

	// Add Authorization header
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	return c.httpClient.Do(req)
}

// authenticatedTransport is a custom transport that adds JWT authentication
type authenticatedTransport struct {
	token     string
	transport http.RoundTripper
}

func (t *authenticatedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// Clone the request to avoid modifying the original
	clonedReq := req.Clone(req.Context())
	clonedReq.Header.Set("Authorization", "Bearer "+t.token)
	clonedReq.Header.Set("Content-Type", "application/json")

	return t.transport.RoundTrip(clonedReq)
}
