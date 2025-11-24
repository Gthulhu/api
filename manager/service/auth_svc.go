package service

import (
	"context"
	"fmt"
	"time"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/golang-jwt/jwt/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func (svc *Service) SignUp(ctx context.Context, email, password string) error {
	creatorID := bson.NewObjectID()
	user := domain.User{
		BaseEntity: domain.NewBaseEntity(util.Ptr(creatorID), util.Ptr(creatorID)),
		Email:      email,
		Password:   domain.EncryptedPassword(password),
		Status:     domain.UserStatusWaitChangePassword,
	}
	err := svc.Repo.CreateUser(ctx, &user)
	if err != nil {
		// TODO: handle duplicate email error
		return err
	}

	return nil
}

func (svc *Service) Login(ctx context.Context, email, password string) (string, error) {
	user, err := svc.getUserByEmaiL(ctx, email)
	if err != nil {
		return "", err
	}
	ok, err := user.Password.Cmp(password)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", fmt.Errorf("invalid password")
	}
	token, err := svc.genJWTToken(ctx, user)
	if err != nil {
		return "", err
	}
	return token, nil
}

func (svc *Service) Logout(ctx context.Context, token string) error {
	return nil
}

func (svc *Service) ChangePassword(ctx context.Context, email string, oldPassword, newPassword string) error {
	user, err := svc.getUserByEmaiL(ctx, email)
	if err != nil {
		return err
	}
	ok, err := user.Password.Cmp(oldPassword)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("invalid old password")
	}
	user.Password = domain.EncryptedPassword(newPassword)
	user.UpdatedTime = time.Now().UnixMilli()
	err = svc.Repo.UpdateUser(ctx, user)
	if err != nil {
		return err
	}
	return nil
}

func (svc *Service) getUserByEmaiL(ctx context.Context, email string) (*domain.User, error) {
	opts := &domain.QueryUserOptions{
		Email: email,
	}
	err := svc.Repo.QueryUsers(ctx, opts)
	if err != nil {
		return nil, err
	}
	users := opts.Result
	if len(users) == 0 {
		// TODO: return specific not found error
		return nil, fmt.Errorf("user with email %s not found", email)
	}

	return users[0], nil
}

func (svc *Service) genJWTToken(ctx context.Context, user *domain.User) (string, error) {
	tokenTTL := time.Duration(3) * time.Hour
	uid := user.ID.Hex()

	roles := []string{}
	for _, roleID := range user.RoleIDs {
		roles = append(roles, roleID.Hex())
	}
	claims := domain.Claims{
		UID:   uid,
		Roles: roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(tokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "bss-api-server",
			Subject:   uid,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(svc.jwtPrivateKey)
}
