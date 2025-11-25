package service

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/manager/domain"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	Repo          domain.Repository
	KeyConfig     config.KeyConfig
	AccountConfig config.AccountConfig
}

func NewService(params Params) (domain.Service, error) {
	jwtPrivateKey, err := initRSAPrivateKey(params.KeyConfig.RsaPrivateKeyPem)
	if err != nil {
		return nil, fmt.Errorf("initialize RSA private key: %w", err)
	}

	svc := &Service{
		Repo:          params.Repo,
		jwtPrivateKey: jwtPrivateKey,
	}

	return svc, nil
}

type Service struct {
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
func (svc Service) CreateRole(ctx context.Context, role *domain.Role) error {
	return errors.New("not implemented")
}
func (svc Service) UpdateRole(ctx context.Context, role *domain.Role) error {
	return errors.New("not implemented")
}
func (svc Service) QueryRoles(ctx context.Context, opt *domain.QueryRoleOptions) error {
	return errors.New("not implemented")
}
func (svc Service) CreatePermission(ctx context.Context, permission *domain.Permission) error {
	return errors.New("not implemented")
}
func (svc Service) UpdatePermission(ctx context.Context, permission *domain.Permission) error {
	return errors.New("not implemented")
}
func (svc Service) QueryPermissions(ctx context.Context, opt *domain.QueryPermissionOptions) error {
	return errors.New("not implemented")
}
