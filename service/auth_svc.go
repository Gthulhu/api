package service

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// VerifyAndGenerateToken verifies the provided public key and generates a JWT token if valid
func (svc *Service) VerifyAndGenerateToken(ctx context.Context, publicKey string) (string, error) {
	err := svc.verifyPublicKey(publicKey)
	if err != nil {
		return "", fmt.Errorf("public key verification failed: %v", err)
	}
	// Generate client ID from public key hash (simplified)
	clientID := fmt.Sprintf("client_%d", time.Now().Unix())
	token, err := svc.generateJWT(clientID)
	if err != nil {
		return "", fmt.Errorf("JWT generation failed: %v", err)
	}
	return token, nil
}

// verifyPublicKey verifies if the provided public key matches our private key
func (svc *Service) verifyPublicKey(publicKeyPEM string) error {
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return fmt.Errorf("failed to decode PEM block containing public key")
	}

	publicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %v", err)
	}

	rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not RSA")
	}

	// Compare public key with our private key's public key
	if !rsaPublicKey.Equal(svc.jwtPrivateKey.PublicKey) {
		return fmt.Errorf("public key does not match server's private key")
	}

	return nil
}

// generateJWT generates a JWT token for authenticated client
func (svc *Service) generateJWT(clientID string) (string, error) {
	claims := Claims{
		ClientID: clientID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Duration(svc.config.JWT.TokenDuration) * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "bss-api-server",
			Subject:   clientID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(svc.jwtPrivateKey)
}

// Claims represents JWT token claims
type Claims struct {
	ClientID string `json:"client_id"`
	jwt.RegisteredClaims
}
