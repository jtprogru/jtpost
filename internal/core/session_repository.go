package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SessionRepository — хранилище server-side sessions.
type SessionRepository interface {
	GetByPrefix(ctx context.Context, prefix string) (*Session, error)
	Create(ctx context.Context, s *Session) error
	Delete(ctx context.Context, id uuid.UUID) error
	DeleteByUser(ctx context.Context, userID uuid.UUID) error
	UpdateLastUsedAt(ctx context.Context, id uuid.UUID, t time.Time) error
	UpdateCSRFToken(ctx context.Context, id uuid.UUID, csrf string) error
}
