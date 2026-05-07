package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// UserRepository — хранилище пользовательских записей.
type UserRepository interface {
	GetByID(ctx context.Context, id uuid.UUID) (*User, error)
	GetByEmail(ctx context.Context, tenantID uuid.UUID, email string) (*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id uuid.UUID) error
	List(ctx context.Context, tenantID uuid.UUID) ([]*User, error)
	Count(ctx context.Context, tenantID uuid.UUID) (int64, error)
	CountOwners(ctx context.Context, tenantID uuid.UUID) (int64, error)
}

// TokenRepository — хранилище Personal Access Tokens.
type TokenRepository interface {
	GetByPrefix(ctx context.Context, prefix string) (*APIToken, error)
	Create(ctx context.Context, t *APIToken) error
	Delete(ctx context.Context, id uuid.UUID) error
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*APIToken, error)
	UpdateLastUsedAt(ctx context.Context, id uuid.UUID, t time.Time) error
}
