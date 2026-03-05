package domain

import "go.mongodb.org/mongo-driver/v2/bson"

type UserStatus int8

const (
	UserStatusActive             UserStatus = 1
	UserStatusInactive           UserStatus = 2
	UserStatusWaitChangePassword UserStatus = 3
)

type User struct {
	BaseEntity     `bson:",inline"`
	UserName       string            `bson:"username,omitempty"`
	Password       EncryptedPassword `bson:"password,omitempty"`
	Status         UserStatus        `bson:"status,omitempty"`
	Roles          []string          `bson:"roles,omitempty"`
	PermissionKeys []string          `bson:"permissionKeys,omitempty"`
}

type Role struct {
	BaseEntity  `bson:",inline"`
	Name        string       `bson:"name,omitempty"`
	Description string       `bson:"description,omitempty"`
	Policies    []RolePolicy `bson:"policies,omitempty"`
}

type UpdateRoleOptions struct {
	Name        *string       `bson:"name,omitempty"`
	Description *string       `bson:"description,omitempty"`
	Policies    *[]RolePolicy `bson:"policies,omitempty"`
}

type RolePolicy struct {
	PermissionKey   PermissionKey `bson:"permissionKey,omitempty"`
	Self            bool          `bson:"self,omitempty"`
	K8SNamespace    string        `bson:"k8sNamespace,omitempty"`
	PolicyNamespace string        `bson:"policeNamespace,omitempty"`
}

type Permission struct {
	ID          bson.ObjectID    `bson:"_id,omitempty"`
	Key         PermissionKey    `bson:"key,omitempty"`
	Description string           `bson:"description,omitempty"`
	Resource    string           `bson:"resource,omitempty"`
	Action      PermissionAction `bson:"action,omitempty"`
}

type UpdateUserPermissionsOptions struct {
	Roles  *[]string
	Status *UserStatus
}

type PermissionAction string

var (
	PermissionActionCreate PermissionAction = "create"
	PermissionActionRead   PermissionAction = "read"
	PermissionActionUpdate PermissionAction = "update"
	PermissionActionDelete PermissionAction = "delete"
)
