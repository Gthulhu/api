package domain

type PermissionKey string

const (
	CreateUser           PermissionKey = "user.create"
	UserRead             PermissionKey = "user.read"
	ChangeUserPermission PermissionKey = "user.permission.update"
	ResetUserPassword    PermissionKey = "user.password.reset"
	RoleCrete            PermissionKey = "role.create"
	RoleRead             PermissionKey = "role.read"
	RoleUpdate           PermissionKey = "role.update"
	RoleDelete           PermissionKey = "role.delete"
	PermissionRead       PermissionKey = "permission.read"
)

const (
	AdminRole = "admin"
)
