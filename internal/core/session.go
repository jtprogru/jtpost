package core

import (
	"time"

	"github.com/google/uuid"
)

// Session — server-side stateful запись о Web-аутентификации.
// Token хранится как prefix (indexed lookup) + secret_hash (bcrypt).
// Pattern зеркалит APIToken из F4a.
type Session struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	Prefix     string // 8 chars, indexed UNIQUE
	SecretHash string // bcrypt of secret
	CSRFToken  string // plaintext, double-submit pattern
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastUsedAt *time.Time
}

// LoginInput — DTO для AuthService.Login.
type LoginInput struct {
	TenantID uuid.UUID
	Email    string
	Password string
}

// LoginResult — результат успешного Login. RawToken (cookie value)
// показывается клиенту один раз — формат `jts_<prefix>_<secret>`.
type LoginResult struct {
	RawToken  string // "jts_<prefix>_<secret>"
	CSRFToken string
	Session   *Session
	User      *User
}
