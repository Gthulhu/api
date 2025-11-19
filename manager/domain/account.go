package domain

import "go.mongodb.org/mongo-driver/v2/bson"

type UserStatus int8

const (
	UserStatusActive             UserStatus = 1
	UserStatusInactive           UserStatus = 2
	UserStatusWaitChangePassword UserStatus = 3
	UserStatusBanned             UserStatus = 4
)

type User struct {
	BaseEntity
	Email          string            `bson:"email,omitempty"`
	Password       EncryptedPassword `bson:"password,omitempty"`
	Status         UserStatus        `bson:"status,omitempty"`
	RoleIDs        []bson.ObjectID   `bson:"_id,omitempty"`
	PermissionKeys []string          `bson:"permission_keys,omitempty"`
}

type Role struct {
	BaseEntity
	Name        string   `bson:"name,omitempty"`
	Description string   `bson:"description,omitempty"`
	Policies    []Policy `bson:"policies,omitempty"`
}

type Policy struct {
	PermissionKey   string `bson:"permission_key,omitempty"`
	Self            bool   `bson:"self,omitempty"`
	K8SNamespace    string `bson:"k8s_namespace,omitempty"`
	PolicyNamespace string `bson:"policy_namespace,omitempty"`
}

type Permission struct {
	BaseEntity
	Key         string `bson:"key,omitempty"`
	Description string `bson:"description,omitempty"`
	Resource    string `bson:"resource,omitempty"`
	Action      string `bson:"action,omitempty"`
}
