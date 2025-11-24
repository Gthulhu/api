package service

import (
	"crypto/rsa"

	"github.com/Gthulhu/api/manager/domain"
)

type Service struct {
	Repo          domain.Repository
	jwtPrivateKey *rsa.PrivateKey
}
