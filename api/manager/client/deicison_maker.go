package client

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	cache "github.com/Code-Hex/go-generics-cache"
	"github.com/Gthulhu/api/config"
	dmrest "github.com/Gthulhu/api/decisionmaker/rest"
	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/logger"
)

func NewDecisionMakerClient(keyConfig config.KeyConfig, mtlsCfg config.MTLSConfig) (domain.DecisionMakerAdapter, error) {
	httpClient := http.DefaultClient

	if mtlsCfg.Enable {
		cert, err := tls.X509KeyPair([]byte(mtlsCfg.CertPem.Value()), []byte(mtlsCfg.KeyPem.Value()))
		if err != nil {
			return nil, fmt.Errorf("load mTLS client certificate: %w", err)
		}

		caPool := x509.NewCertPool()
		if !caPool.AppendCertsFromPEM([]byte(mtlsCfg.CAPem.Value())) {
			return nil, fmt.Errorf("parse mTLS CA certificate")
		}

		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			RootCAs:      caPool,
			MinVersion:   tls.VersionTLS12,
			ServerName:   mtlsCfg.ServerName,
		}

		defaultTransport, ok := http.DefaultTransport.(*http.Transport)
		if !ok {
			return nil, fmt.Errorf("unexpected default transport type %T", http.DefaultTransport)
		}
		mtlsTransport := defaultTransport.Clone()
		mtlsTransport.TLSClientConfig = tlsCfg

		httpClient = &http.Client{
			Transport: mtlsTransport,
		}
	}

	return &DecisionMakerClient{
		Client:         httpClient,
		mtlsEnabled:    mtlsCfg.Enable,
		tokenPublicKey: keyConfig.DMPublicKeyPem.Value(),
		clientID:       keyConfig.ClientID,
		tokenCache:     cache.New[string, string](),
	}, nil
}

type DecisionMakerClient struct {
	*http.Client

	mtlsEnabled    bool
	tokenPublicKey string
	clientID       string
	tokenCache     *cache.Cache[string, string]
}

// scheme returns "https" when mTLS is enabled, "http" otherwise.
func (dm *DecisionMakerClient) scheme() string {
	if dm.mtlsEnabled {
		return "https"
	}
	return "http"
}

func (dm *DecisionMakerClient) SendSchedulingIntent(ctx context.Context, decisionMaker *domain.DecisionMakerPod, intents []*domain.ScheduleIntent) error {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return err
	}

	logger.Logger(ctx).Debug().Msgf("Sending %d scheduling intents to decision maker pod (host:%s nodeID:%s port:%d)", len(intents), decisionMaker.Host, decisionMaker.NodeID, decisionMaker.Port)

	reqPayload := dmrest.HandleIntentsRequest{
		Intents: make([]dmrest.Intent, 0, len(intents)),
	}
	for _, intent := range intents {
		reqPayload.Intents = append(reqPayload.Intents, dmrest.Intent{
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

	jsonBody, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}
	endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/intents"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := dm.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}
	return nil
}

func (dm *DecisionMakerClient) GetIntentMerkleRoot(ctx context.Context, decisionMaker *domain.DecisionMakerPod) (string, error) {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return "", err
	}

	endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/intents/merkle"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := dm.Client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}

	var merkleResp dmrest.SuccessResponse[dmrest.MerkleRootResponse]
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&merkleResp); err != nil {
		return "", err
	}
	if merkleResp.Data == nil {
		return "", fmt.Errorf("decision maker %s returned empty merkle root", decisionMaker)
	}
	return merkleResp.Data.RootHash, nil
}

func (dm *DecisionMakerClient) GetToken(ctx context.Context, decisionMaker *domain.DecisionMakerPod) (string, error) {
	if token, ok := dm.tokenCache.Get(decisionMaker.NodeID); ok {
		return token, nil
	}

	req := dmrest.TokenRequest{
		PublicKey: dm.tokenPublicKey,
		ClientID:  dm.clientID,
	}
	jsonBody, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/auth/token"
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := dm.Client.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}
	var tokenResp dmrest.SuccessResponse[dmrest.TokenResponse]
	decoder := json.NewDecoder(resp.Body)
	err = decoder.Decode(&tokenResp)
	if err != nil {
		return "", err
	}

	ttl := time.Now().Unix() - tokenResp.Data.ExpiredAt - 60

	dm.tokenCache.Set(decisionMaker.NodeID, tokenResp.Data.Token, cache.WithExpiration(time.Duration(ttl)*time.Second))
	return tokenResp.Data.Token, nil
}

func (dm *DecisionMakerClient) DeleteSchedulingIntents(ctx context.Context, decisionMaker *domain.DecisionMakerPod, req *domain.DeleteIntentsRequest) error {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return err
	}

	logger.Logger(ctx).Debug().Msgf("Deleting scheduling intents from decision maker pod (host:%s nodeID:%s port:%d)", decisionMaker.Host, decisionMaker.NodeID, decisionMaker.Port)

	// If All is true, delete all intents; otherwise delete by PodIDs one by one
	if req.All {
		deleteReq := dmrest.DeleteIntentRequest{
			All: true,
		}
		jsonBody, err := json.Marshal(deleteReq)
		if err != nil {
			return err
		}
		endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/intents"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+token)
		resp, err := dm.Client.Do(httpReq)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
		}
		return nil
	}

	// Delete intents by PodID
	for _, podID := range req.PodIDs {
		deleteReq := dmrest.DeleteIntentRequest{
			PodID: podID,
		}
		jsonBody, err := json.Marshal(deleteReq)
		if err != nil {
			return err
		}
		endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/intents"
		httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+token)
		resp, err := dm.Client.Do(httpReq)
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("decision maker %s returned non-OK status for podID %s: %s", decisionMaker, podID, resp.Status)
		}
	}

	return nil
}

func (dm *DecisionMakerClient) GetPodPIDMapping(ctx context.Context, decisionMaker *domain.DecisionMakerPod) (*domain.PodPIDMappingResponse, error) {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return nil, err
	}

	endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/pods/pids"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := dm.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}

	var podPIDResp dmrest.SuccessResponse[dmrest.GetPodsPIDsResponse]
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&podPIDResp); err != nil {
		return nil, err
	}
	if podPIDResp.Data == nil {
		return nil, fmt.Errorf("decision maker %s returned empty pod-pid mapping", decisionMaker)
	}

	// Convert dmrest types to domain types
	result := &domain.PodPIDMappingResponse{
		Timestamp: podPIDResp.Data.Timestamp,
		NodeName:  podPIDResp.Data.NodeName,
		NodeID:    podPIDResp.Data.NodeID,
		Pods:      make([]domain.PodPIDInfo, 0, len(podPIDResp.Data.Pods)),
	}
	for _, pod := range podPIDResp.Data.Pods {
		podInfo := domain.PodPIDInfo{
			PodUID:    pod.PodUID,
			PodID:     pod.PodID,
			Processes: make([]domain.PodProcess, 0, len(pod.Processes)),
		}
		for _, proc := range pod.Processes {
			podInfo.Processes = append(podInfo.Processes, domain.PodProcess{
				PID:         proc.PID,
				Command:     proc.Command,
				PPID:        proc.PPID,
				ContainerID: proc.ContainerID,
			})
		}
		result.Pods = append(result.Pods, podInfo)
	}

	return result, nil
}

func (dm *DecisionMakerClient) ApplyRuntimeConfig(ctx context.Context, decisionMaker *domain.DecisionMakerPod, config domain.RuntimeSchedulerConfig) error {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return err
	}

	type applyRuntimeConfigRequest struct {
		ConfigVersion     string `json:"configVersion,omitempty"`
		Mode              string `json:"mode,omitempty"`
		SliceNsDefault    uint64 `json:"sliceNsDefault,omitempty"`
		SliceNsMin        uint64 `json:"sliceNsMin,omitempty"`
		KernelMode        bool   `json:"kernelMode,omitempty"`
		MaxTimeWatchdog   bool   `json:"maxTimeWatchdog,omitempty"`
		EarlyProcessing   bool   `json:"earlyProcessing,omitempty"`
		BuiltinIdle       bool   `json:"builtinIdle,omitempty"`
		SchedulerEnabled  bool   `json:"schedulerEnabled"`
		MonitoringEnabled bool   `json:"monitoringEnabled"`
	}

	reqPayload := applyRuntimeConfigRequest{
		ConfigVersion:     config.ConfigVersion,
		Mode:              config.Mode,
		SliceNsDefault:    config.SliceNsDefault,
		SliceNsMin:        config.SliceNsMin,
		KernelMode:        config.KernelMode,
		MaxTimeWatchdog:   config.MaxTimeWatchdog,
		EarlyProcessing:   config.EarlyProcessing,
		BuiltinIdle:       config.BuiltinIdle,
		SchedulerEnabled:  config.SchedulerEnabled,
		MonitoringEnabled: config.MonitoringEnabled,
	}

	jsonBody, err := json.Marshal(reqPayload)
	if err != nil {
		return err
	}

	endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/runtime-config"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := dm.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}

	return nil
}

func (dm *DecisionMakerClient) GetRuntimeConfigStatus(ctx context.Context, decisionMaker *domain.DecisionMakerPod) (domain.RuntimeConfigApplyResult, error) {
	token, err := dm.GetToken(ctx, decisionMaker)
	if err != nil {
		return domain.RuntimeConfigApplyResult{}, err
	}

	endpoint := dm.scheme() + "://" + decisionMaker.Host + ":" + strconv.Itoa(decisionMaker.Port) + "/api/v1/runtime-config"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return domain.RuntimeConfigApplyResult{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := dm.Client.Do(req)
	if err != nil {
		return domain.RuntimeConfigApplyResult{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return domain.RuntimeConfigApplyResult{}, fmt.Errorf("decision maker %s returned non-OK status: %s", decisionMaker, resp.Status)
	}

	type runtimeConfigStatusResponse struct {
		Success bool `json:"success"`
		Data    *struct {
			ConfigVersion string `json:"configVersion,omitempty"`
			Applied       bool   `json:"applied"`
			AppliedAt     string `json:"appliedAt,omitempty"`
			RestartCount  int64  `json:"restartCount,omitempty"`
			LastError     string `json:"lastError,omitempty"`
		} `json:"data,omitempty"`
	}

	var statusResp runtimeConfigStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		return domain.RuntimeConfigApplyResult{}, err
	}

	if !statusResp.Success {
		return domain.RuntimeConfigApplyResult{}, fmt.Errorf("decision maker %s returned unsuccessful status response", decisionMaker)
	}

	result := domain.RuntimeConfigApplyResult{
		NodeID: decisionMaker.NodeID,
		Host:   decisionMaker.Host,
	}
	if statusResp.Data != nil {
		result.ConfigVersion = statusResp.Data.ConfigVersion
		result.AppliedAt = statusResp.Data.AppliedAt
		result.RestartCount = statusResp.Data.RestartCount
		result.LastError = statusResp.Data.LastError
		result.Success = statusResp.Data.Applied
		if !statusResp.Data.Applied {
			if statusResp.Data.LastError != "" {
				result.Error = statusResp.Data.LastError
			} else {
				result.Error = "runtime config not applied yet"
			}
		}
	} else {
		result.Error = "runtime config not applied yet"
	}

	return result, nil
}
