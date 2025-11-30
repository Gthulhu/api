package domain

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type QueryUserOptions struct {
	IDs       []bson.ObjectID
	UserNames []string
	Result    []*User
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
	CreateNewUser(ctx context.Context, operator *Claims, username, password string) error
	CreateAdminUserIfNotExists(ctx context.Context, username, password string) error
	Login(ctx context.Context, email, password string) (token string, err error)
	ChangePassword(ctx context.Context, user *Claims, oldPassword, newPassword string) error
	ResetPassword(ctx context.Context, operator *Claims, id, newPassword string) error
	UpdateUserPermissions(ctx context.Context, operator *Claims, id string, opt UpdateUserPermissionsOptions) error
	VerifyJWTToken(ctx context.Context, tokenString string, permissionKey PermissionKey) (Claims, RolePolicy, error)
	QueryUsers(ctx context.Context, opt *QueryUserOptions) error

	CreateRole(ctx context.Context, operator *Claims, role *Role) error
	UpdateRole(ctx context.Context, operator *Claims, roleID string, opt UpdateRoleOptions) error
	DeleteRole(ctx context.Context, operator *Claims, roleID string) error
	QueryRoles(ctx context.Context, opt *QueryRoleOptions) error
	QueryPermissions(ctx context.Context, opt *QueryPermissionOptions) error

	ListAuditLogs(ctx context.Context, opt *QueryAuditLogOptions) error
}
