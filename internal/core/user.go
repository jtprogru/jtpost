package core

import (
	"time"

	"github.com/google/uuid"
)

// Role — роль пользователя в RBAC.
type Role string

// Hardcoded roles (F4a). Custom roles — отложены.
const (
	RoleOwner  Role = "owner"
	RoleEditor Role = "editor"
	RoleAuthor Role = "author"
	RoleViewer Role = "viewer"
)

// Permission — атомарное действие, которое может быть разрешено или запрещено.
type Permission string

// 6 permissions покрывающих core-операции jtpost.
const (
	PermPostsCreate  Permission = "posts:create"
	PermPostsEdit    Permission = "posts:edit"
	PermPostsDelete  Permission = "posts:delete"
	PermPostsPublish Permission = "posts:publish"
	PermUsersManage  Permission = "users:manage"
	PermTokensManage Permission = "tokens:manage"
)

// rolePermissionsTable — фиксированный маппинг ролей на permissions.
//
//nolint:gochecknoglobals // immutable lookup table
var rolePermissionsTable = map[Role][]Permission{
	RoleOwner: {
		PermPostsCreate, PermPostsEdit, PermPostsDelete, PermPostsPublish,
		PermUsersManage, PermTokensManage,
	},
	RoleEditor: {
		PermPostsCreate, PermPostsEdit, PermPostsDelete, PermPostsPublish,
	},
	RoleAuthor: {
		PermPostsCreate, PermPostsEdit,
	},
	RoleViewer: {},
}

// RolePermissions возвращает permissions, разрешённые роли. Для неизвестной
// роли возвращает пустой слайс.
func RolePermissions(r Role) []Permission {
	perms, ok := rolePermissionsTable[r]
	if !ok {
		return []Permission{}
	}
	out := make([]Permission, len(perms))
	copy(out, perms)
	return out
}

// User — учётная запись.
type User struct {
	ID           uuid.UUID
	TenantID     uuid.UUID
	Email        string
	PasswordHash string
	Role         Role
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// APIToken — Personal Access Token.
type APIToken struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Prefix     string // 8 chars, indexed
	SecretHash string // bcrypt
	Name       string
	CreatedAt  time.Time
	ExpiresAt  *time.Time
	LastUsedAt *time.Time
}

// CreateUserInput — DTO для AuthService.CreateUser.
type CreateUserInput struct {
	TenantID uuid.UUID
	Email    string
	Password string
	Role     Role
}

// IssuedToken — результат IssueToken; .Raw показывается caller'у один раз
// и НЕ сохраняется в БД (только SecretHash).
type IssuedToken struct {
	Raw   string // "jtpat_<prefix>_<secret>"
	Token *APIToken
}
