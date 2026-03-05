package rest

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/errs"
	"github.com/Gthulhu/api/pkg/logger"
	"go.uber.org/fx"
)

// ErrorResponse represents error response structure
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// EmptyResponse is used for endpoints that return no data payload.
type EmptyResponse struct{}

// VersionResponse describes the version endpoint payload.
type VersionResponse struct {
	Message   string `json:"message"`
	Version   string `json:"version"`
	Endpoints string `json:"endpoints"`
}

// HealthResponse describes the health check payload.
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Service   string `json:"service"`
}

func NewSuccessResponse[T any](data *T) SuccessResponse[T] {
	return SuccessResponse[T]{
		Success:   true,
		Data:      data,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
}

// SuccessResponse represents the success response structure
type SuccessResponse[T any] struct {
	Success   bool   `json:"success"`
	Data      *T     `json:"data,omitempty"`
	Timestamp string `json:"timestamp"`
}

type Params struct {
	fx.In
	Svc domain.Service
}

func NewHandler(params Params) (*Handler, error) {
	return &Handler{
		Svc: params.Svc,
	}, nil
}

type Handler struct {
	Svc domain.Service
}

func (h *Handler) JSONResponse(ctx context.Context, w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	err := json.NewEncoder(w).Encode(data)
	if err != nil {
		logger.Logger(ctx).Error().Err(err).Msg("Failed to encode JSON response")
		http.Error(w, "Failed to encode JSON response", http.StatusInternalServerError)
	}
}

func (h *Handler) JSONBind(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(dst)
	if err != nil {
		return err
	}
	return nil
}

func (h *Handler) HandleError(ctx context.Context, w http.ResponseWriter, err error) {
	httpErr, ok := errs.IsHTTPStatusError(err)
	if ok {
		h.ErrorResponse(ctx, w, httpErr.StatusCode, httpErr.Message, httpErr.OriginalErr)
		return
	}
	h.ErrorResponse(ctx, w, http.StatusInternalServerError, "Internal Server Error", err)
}

func (h *Handler) ErrorResponse(ctx context.Context, w http.ResponseWriter, status int, errMsg string, err error) {
	if err != nil {
		if status >= 500 {
			logger.Logger(ctx).Error().Err(err).Msg(errMsg)
		} else {
			logger.Logger(ctx).Warn().Err(err).Msg(errMsg)
		}
	}
	resp := ErrorResponse{
		Success: false,
		Error:   errMsg,
	}
	h.JSONResponse(ctx, w, status, resp)
}

// Version godoc
// @Summary Get service version
// @Description Returns service version and exposed endpoints.
// @Tags System
// @Produce json
// @Success 200 {object} VersionResponse
// @Router /version [get]
func (h *Handler) Version(w http.ResponseWriter, r *http.Request) {
	response := VersionResponse{
		Message:   "BSS Metrics API Server",
		Version:   "1.0.0",
		Endpoints: "/api/v1/auth/token (POST), /api/v1/metrics (POST), /api/v1/pods/pids (GET), /api/v1/scheduling/strategies (GET, POST), /health (GET), /static/ (Frontend)",
	}
	h.JSONResponse(r.Context(), w, http.StatusOK, response)
}

// HealthCheck godoc
// @Summary Health check
// @Description Basic health check for readiness probes.
// @Tags System
// @Produce json
// @Success 200 {object} HealthResponse
// @Router /health [get]
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "healthy",
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Service:   "BSS Metrics API Server",
	}
	h.JSONResponse(r.Context(), w, http.StatusOK, response)
}

type claimsKey struct{}

// GetClaimsFromContext extracts domain.Claims from the request context
func (h *Handler) GetClaimsFromContext(ctx context.Context) (domain.Claims, bool) {
	claims, ok := ctx.Value(claimsKey{}).(domain.Claims)
	return claims, ok
}

func (h *Handler) SetClaimsInContext(ctx context.Context, claims domain.Claims) context.Context {
	return context.WithValue(ctx, claimsKey{}, claims)
}

type rolePolicyKey struct{}

func (h *Handler) SetRolePolicyInContext(ctx context.Context, rolePolicy domain.RolePolicy) context.Context {
	return context.WithValue(ctx, rolePolicyKey{}, rolePolicy)
}

func (h *Handler) GetRolePolicyFromContext(ctx context.Context) (domain.RolePolicy, bool) {
	rolePolicy, ok := ctx.Value(rolePolicyKey{}).(domain.RolePolicy)
	return rolePolicy, ok
}

func (h *Handler) VerifyResourcePolicy(ctx context.Context, resourceOwnerID string) error {
	claims, ok := h.GetClaimsFromContext(ctx)
	if !ok {
		return errs.NewHTTPStatusError(http.StatusUnauthorized, "unauthorized", errors.New("claims not found in context"))
	}
	rolePolicy, ok := h.GetRolePolicyFromContext(ctx)
	if !ok {
		return errs.NewHTTPStatusError(http.StatusUnauthorized, "unauthorized", errors.New("role policy not found in context"))
	}
	if rolePolicy.Self && claims.UID != resourceOwnerID {
		return errs.NewHTTPStatusError(http.StatusForbidden, "forbidden", errors.New("access to resource denied"))
	}
	return nil
}
