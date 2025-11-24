package domain

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type QueryUserOptions struct {
	IDs    []bson.ObjectID
	Email  string
	Result []*User
}

type QueryRoleOptions struct {
	IDs    []bson.ObjectID
	Names  []string
	Result []*Role
}

type QueryPermissionOptions struct {
	IDs       []bson.ObjectID
	Keys      []string
	Resources []string
	Result    []*Permission
}

type QueryAuditLogOptions struct {
	TimestampGTE int64
	TimestampLTE int64
	UserIDs      []bson.ObjectID
	Result       []*AuditLog
}

type Repository interface {
	CreateUser(ctx context.Context, user *User) error
	UpdateUser(ctx context.Context, user *User) error
	QueryUsers(ctx context.Context, opt *QueryUserOptions) error
	CreateRole(ctx context.Context, role *Role) error
	UpdateRole(ctx context.Context, role *Role) error
	QueryRoles(ctx context.Context, opt *QueryRoleOptions) error
	CreatePermission(ctx context.Context, permission *Permission) error
	UpdatePermission(ctx context.Context, permission *Permission) error
	QueryPermissions(ctx context.Context, opt *QueryPermissionOptions) error
	CreateAuditLog(ctx context.Context, log *AuditLog) error
	QueryAuditLogs(ctx context.Context, opt *QueryAuditLogOptions) error
}

type Service interface {
	SignUp(ctx context.Context, email, password string) error
	Login(ctx context.Context, email, password string) (string, error)
	Logout(ctx context.Context, token string) error
	ChangePassword(ctx context.Context, email string, oldPassword, newPassword EncryptedPassword) error
	CreateUser(ctx context.Context, operator Claims, user *User) error
	DeleteUser(ctx context.Context, operator Claims, userID bson.ObjectID) error
	UpdateUser(ctx context.Context, operator Claims, user *User) error
	ListAuditLogs(ctx context.Context, opt *QueryAuditLogOptions) error
	CreateRole(ctx context.Context, role *Role) error
	UpdateRole(ctx context.Context, role *Role) error
	QueryRoles(ctx context.Context, opt *QueryRoleOptions) error
	CreatePermission(ctx context.Context, permission *Permission) error
	UpdatePermission(ctx context.Context, permission *Permission) error
	QueryPermissions(ctx context.Context, opt *QueryPermissionOptions) error
}
