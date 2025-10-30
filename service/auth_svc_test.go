package service_test

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyJWTToken(t *testing.T) {
	_, f, _, _ := runtime.Caller(0)
	configPath := filepath.Join(filepath.Dir(f), "..", "config", "jwt_private_key.key")

	privateKey, err := config.InitJWTRsaKey(config.JWTConfig{
		PrivateKeyPath: configPath,
	})
	require.NoError(t, err, "can't init jwt rsa key")

	svc, err := service.NewService(context.Background(), service.Params{
		Config: &config.Config{
			Strategies: config.StrategiesConfig{},
		},
		JWTPrivateKey: privateKey,
	})
	require.NoError(t, err, "new service failed")

	pubKeyString, err := PublicKeyToString(&privateKey.PublicKey)
	require.NoError(t, err, "generate public key string failed")

	token, err := svc.VerifyAndGenerateToken(context.Background(), pubKeyString)
	require.NoError(t, err, "verify public key and generate token failed")
	assert.NotEmpty(t, token)
}

func PublicKeyToString(pub *rsa.PublicKey) (string, error) {
	pubASN1, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", err
	}

	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubASN1,
	})

	return string(pubPEM), nil
}
