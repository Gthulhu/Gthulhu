package rest

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/golang-jwt/jwt/v5"
)

func GetJwtAuthMiddleware(tokenConfig config.TokenConfig) (func(next http.Handler) http.Handler, error) {
	rasKey, err := util.InitRSAPrivateKey(string(tokenConfig.RsaPrivateKeyPem))
	if err != nil {
		return nil, err
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Skip auth for OPTIONS requests, health check, root endpoint, token endpoint, and static files
			if r.Method == "OPTIONS" ||
				r.URL.Path == "/health" ||
				r.URL.Path == "/" ||
				r.URL.Path == "/api/v1/auth/token" ||
				strings.HasPrefix(r.URL.Path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				if err := json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error:   "Authorization header is required",
				}); err != nil {
					logger.Logger(r.Context()).Error().Err(err).Msg("Failed to write unauthorized response")
				}
				return
			}

			// Check Bearer token format
			const bearerSchema = "Bearer "
			if !strings.HasPrefix(authHeader, bearerSchema) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				if err := json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error:   "Authorization header must start with 'Bearer '",
				}); err != nil {
					logger.Logger(r.Context()).Error().Err(err).Msg("Failed to write unauthorized response")
				}
				return
			}

			tokenString := authHeader[len(bearerSchema):]

			// Validate JWT token
			claims, err := validateJWT(rasKey, tokenString)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				if err := json.NewEncoder(w).Encode(ErrorResponse{
					Success: false,
					Error:   "Invalid or expired token: " + err.Error(),
				}); err != nil {
					logger.Logger(r.Context()).Error().Err(err).Msg("Failed to write unauthorized response")
				}
				return
			}

			logger.Logger(r.Context()).Info().Str("client_id", claims.ClientID).Msg("JWT token validated successfully")
			next.ServeHTTP(w, r)
		})
	}, nil
}

// Claims represents JWT token claims
type Claims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}

// validateJWT validates a JWT token and returns the claims
func validateJWT(rasKey *rsa.PrivateKey, tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return &rasKey.PublicKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, fmt.Errorf("invalid token")
}
