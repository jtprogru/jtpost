package core

import (
	"context"

	"github.com/google/uuid"
)

// OAuthAccountRepository управляет связями между users и OAuth-провайдерами.
// Реализации: sqlite/postgres адаптеры. fs не поддерживает.
type OAuthAccountRepository interface {
	// GetByExternalID возвращает OAuthAccount по (provider, externalID).
	// На отсутствие → ErrNotFound.
	GetByExternalID(ctx context.Context, provider, externalID string) (*OAuthAccount, error)
	// Create вставляет новый OAuthAccount. На UNIQUE(provider, external_id) → ErrAlreadyExists.
	Create(ctx context.Context, a *OAuthAccount) error
	// ListByUser возвращает все OAuthAccounts пользователя, ORDER BY created_at ASC.
	ListByUser(ctx context.Context, userID uuid.UUID) ([]*OAuthAccount, error)
	// Delete удаляет OAuthAccount по ID. На отсутствие → ErrNotFound.
	Delete(ctx context.Context, id uuid.UUID) error
}
