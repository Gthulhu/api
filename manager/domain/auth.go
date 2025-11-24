package domain

import "github.com/golang-jwt/jwt/v5"

// Claims represents JWT token claims
type Claims struct {
	UID   string   `json:"uid"`
	Roles []string `json:"roles"`
	jwt.RegisteredClaims
}
