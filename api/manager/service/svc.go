package service

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/domain"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	Repo          domain.Repository
	KeyConfig     config.KeyConfig
	AccountConfig config.AccountConfig
	K8SAdapter    domain.K8SAdapter
	DMAdapter     domain.DecisionMakerAdapter
}

func NewService(params Params) (domain.Service, error) {
	jwtPrivateKey, err := initRSAPrivateKey(string(params.KeyConfig.RsaPrivateKeyPem))
	if err != nil {
		return nil, fmt.Errorf("initialize RSA private key: %w", err)
	}

	svc := &Service{
		K8SAdapter:    params.K8SAdapter,
		DMAdapter:     params.DMAdapter,
		Repo:          params.Repo,
		jwtPrivateKey: jwtPrivateKey,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	err = svc.CreateAdminUserIfNotExists(ctx, params.AccountConfig.AdminEmail, params.AccountConfig.AdminPassword.Value())
	if err != nil {
		return nil, fmt.Errorf("create admin user if not exists: %w", err)
	}

	return svc, nil
}

type Service struct {
	K8SAdapter    domain.K8SAdapter
	DMAdapter     domain.DecisionMakerAdapter
	Repo          domain.Repository
	jwtPrivateKey *rsa.PrivateKey
}

func initRSAPrivateKey(pemStr string) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		// Try PKCS8 format
		keyInterface, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %v", err)
		}
		var ok bool
		key, ok = keyInterface.(*rsa.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("private key is not RSA")
		}
	}
	return key, nil
}

func (svc Service) ListAuditLogs(ctx context.Context, opt *domain.QueryAuditLogOptions) error {
	return errors.New("not implemented")
}
