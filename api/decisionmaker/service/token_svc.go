package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/golang-jwt/jwt/v5"
)

// VerifyAndGenerateToken verifies the provided public key and generates a JWT token if valid
func (svc *Service) VerifyAndGenerateToken(ctx context.Context, clientID string, publicKey string) (string, int64, error) {
	err := svc.VerifyPublicKey(publicKey)
	if err != nil {
		return "", 0, fmt.Errorf("public key verification failed: %v", err)
	}
	token, claims, err := svc.generateJWT(ctx, clientID)
	if err != nil {
		return "", 0, fmt.Errorf("JWT generation failed: %v", err)
	}
	return token, claims.ExpiresAt.Unix(), nil
}

// verifyPublicKey verifies if the provided public key matches our private key
func (svc *Service) VerifyPublicKey(publicKeyPEM string) error {
	rsaPublicKey, err := util.PEMToRSAPublicKey(publicKeyPEM)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %v", err)
	}
	// Compare public key with our private key's public key
	if !rsaPublicKey.Equal(&svc.jwtPrivateKey.PublicKey) {
		return fmt.Errorf("public key does not match server's private key")
	}

	return nil
}

// generateJWT generates a JWT token for authenticated client
func (svc *Service) generateJWT(ctx context.Context, clientID string) (string, Claims, error) {
	expireHr := svc.tokenConfig.TokenDurationHr
	if expireHr <= 0 {
		logger.Logger(ctx).Warn().Msgf("invalid token duration hr %d, defaulting to 24 hours", expireHr)
		expireHr = 24 // default to 24 hours
	}

	claims := Claims{
		ClientID: clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(expireHr) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "decision-maker-service",
			Subject:   clientID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	tokenStr, err := token.SignedString(svc.jwtPrivateKey)
	if err != nil {
		return "", Claims{}, fmt.Errorf("failed to sign JWT token: %v", err)
	}
	return tokenStr, claims, nil
}

// Claims represents JWT token claims
type Claims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}
