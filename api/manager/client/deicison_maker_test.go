package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIntentMerkleRootSuccess(t *testing.T) {
	const cachedToken = "cached-token"
	const rootHash = "root-hash-123"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/api/v1/intents/merkle", r.URL.Path)
		assert.Equal(t, "Bearer "+cachedToken, r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"rootHash":"` + rootHash + `"},"timestamp":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	dm := newDecisionMakerPodFromServerURL(t, server.URL)
	client := newDecisionMakerClientWithCachedToken(dm.NodeID, cachedToken, server.Client())

	got, err := client.GetIntentMerkleRoot(context.Background(), dm)
	require.NoError(t, err)
	assert.Equal(t, rootHash, got)
}

func TestGetIntentMerkleRootNonOKStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dm := newDecisionMakerPodFromServerURL(t, server.URL)
	client := newDecisionMakerClientWithCachedToken(dm.NodeID, "cached-token", server.Client())

	_, err := client.GetIntentMerkleRoot(context.Background(), dm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned non-OK status")
}

func TestGetIntentMerkleRootEmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":null,"timestamp":"2026-01-01T00:00:00Z"}`))
	}))
	defer server.Close()

	dm := newDecisionMakerPodFromServerURL(t, server.URL)
	client := newDecisionMakerClientWithCachedToken(dm.NodeID, "cached-token", server.Client())

	_, err := client.GetIntentMerkleRoot(context.Background(), dm)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "returned empty merkle root")
}

func newDecisionMakerClientWithCachedToken(nodeID, token string, httpClient *http.Client) *DecisionMakerClient {
	tokenCache := cache.New[string, string]()
	tokenCache.Set(nodeID, token)
	return &DecisionMakerClient{
		Client:     httpClient,
		tokenCache: tokenCache,
	}
}

func newDecisionMakerPodFromServerURL(t *testing.T, rawURL string) *domain.DecisionMakerPod {
	t.Helper()
	parsedURL, err := url.Parse(rawURL)
	require.NoError(t, err)
	host, portStr, err := net.SplitHostPort(parsedURL.Host)
	require.NoError(t, err)
	port, err := strconv.Atoi(portStr)
	require.NoError(t, err)
	return &domain.DecisionMakerPod{
		NodeID: "node-1",
		Host:   host,
		Port:   port,
		State:  domain.NodeStateOnline,
	}
}

func TestNewDecisionMakerClientMTLSDisabled(t *testing.T) {
	keyConfig := config.KeyConfig{}
	mtlsCfg := config.MTLSConfig{Enable: false}

	c, err := NewDecisionMakerClient(keyConfig, mtlsCfg)
	require.NoError(t, err)
	require.NotNil(t, c)

	dc := c.(*DecisionMakerClient)
	assert.False(t, dc.mtlsEnabled)
	assert.Equal(t, "http", dc.scheme())
}

func TestNewDecisionMakerClientMTLSBadCert(t *testing.T) {
	mtlsCfg := config.MTLSConfig{
		Enable:  true,
		CertPem: "not-valid-pem",
		KeyPem:  "not-valid-pem",
		CAPem:   "not-valid-pem",
	}
	_, err := NewDecisionMakerClient(config.KeyConfig{}, mtlsCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "load mTLS client certificate")
}

func TestNewDecisionMakerClientMTLSBadCA(t *testing.T) {
	certs := generateTestCerts(t)

	mtlsCfg := config.MTLSConfig{
		Enable:  true,
		CertPem: config.SecretValue(certs.certPEM),
		KeyPem:  config.SecretValue(certs.keyPEM),
		CAPem:   config.SecretValue("not-a-valid-ca-pem"),
	}
	_, err := NewDecisionMakerClient(config.KeyConfig{}, mtlsCfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse mTLS CA certificate")
}

func TestDecisionMakerClientMTLSEnabled(t *testing.T) {
	certs := generateTestCerts(t)

	mtlsCfg := config.MTLSConfig{
		Enable:  true,
		CertPem: config.SecretValue(certs.certPEM),
		KeyPem:  config.SecretValue(certs.keyPEM),
		CAPem:   config.SecretValue(certs.caPEM),
	}
	c, err := NewDecisionMakerClient(config.KeyConfig{}, mtlsCfg)
	require.NoError(t, err)
	require.NotNil(t, c)

	dc := c.(*DecisionMakerClient)
	assert.True(t, dc.mtlsEnabled)
	assert.Equal(t, "https", dc.scheme())
}

func TestDecisionMakerClientMTLSEndToEnd(t *testing.T) {
	const cachedToken = "cached-token"
	const rootHash = "mtls-root-hash"

	certs := generateTestCerts(t)

	// Build mTLS server (requires client cert signed by the CA)
	serverCert, err := tls.X509KeyPair([]byte(certs.certPEM), []byte(certs.keyPEM))
	require.NoError(t, err)
	caPool := x509.NewCertPool()
	require.True(t, caPool.AppendCertsFromPEM([]byte(certs.caPEM)))

	serverTLSCfg := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caPool,
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"success":true,"data":{"rootHash":"` + rootHash + `"},"timestamp":"2026-01-01T00:00:00Z"}`))
	}))
	server.TLS = serverTLSCfg
	server.StartTLS()
	defer server.Close()

	// Build mTLS client
	mtlsCfg := config.MTLSConfig{
		Enable:  true,
		CertPem: config.SecretValue(certs.certPEM),
		KeyPem:  config.SecretValue(certs.keyPEM),
		CAPem:   config.SecretValue(certs.caPEM),
	}
	c, err := NewDecisionMakerClient(config.KeyConfig{}, mtlsCfg)
	require.NoError(t, err)

	dm := newDecisionMakerPodFromServerURL(t, server.URL)

	// Inject cached token so GetIntentMerkleRoot skips GetToken
	dc := c.(*DecisionMakerClient)
	dc.tokenCache.Set(dm.NodeID, cachedToken)

	got, err := dc.GetIntentMerkleRoot(context.Background(), dm)
	require.NoError(t, err)
	assert.Equal(t, rootHash, got)
}

// testCerts holds PEM-encoded self-signed CA + leaf cert for unit testing.
type testCerts struct {
	caPEM   string
	certPEM string
	keyPEM  string
}

// generateTestCerts creates a minimal self-signed CA and a leaf cert/key signed by it.
func generateTestCerts(t *testing.T) testCerts {
	t.Helper()

	// Use a fixed time window so tests remain deterministic regardless of when they run.
	notBefore := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2100, 1, 1, 0, 0, 0, 0, time.UTC)

	// Generate CA key + cert
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "test-ca"},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		IsCA:                  true,
		BasicConstraintsValid: true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caKey.PublicKey, caKey)
	require.NoError(t, err)
	caCert, err := x509.ParseCertificate(caDER)
	require.NoError(t, err)
	caPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))

	// Generate leaf key + cert signed by CA
	leafKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	leafTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-leaf"},
		NotBefore:    notBefore,
		NotAfter:     notAfter,
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTemplate, caCert, &leafKey.PublicKey, caKey)
	require.NoError(t, err)
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: leafDER}))

	leafKeyDER, err := x509.MarshalECPrivateKey(leafKey)
	require.NoError(t, err)
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: leafKeyDER}))

	return testCerts{caPEM: caPEM, certPEM: certPEM, keyPEM: keyPEM}
}
