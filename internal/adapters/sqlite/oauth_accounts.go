package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/adapters/sqlite/sqlitedb"
	"github.com/jtprogru/jtpost/internal/core"
)

// OAuthAccountRepository реализует core.OAuthAccountRepository поверх SQLite.
type OAuthAccountRepository struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// OAuthAccounts возвращает OAuthAccountRepository поверх того же подключения.
func (r *PostRepository) OAuthAccounts() *OAuthAccountRepository {
	return &OAuthAccountRepository{db: r.db, queries: r.queries}
}

// Compile-time проверка соответствия интерфейсу.
var _ core.OAuthAccountRepository = (*OAuthAccountRepository)(nil)

// Create вставляет новый OAuthAccount.
func (r *OAuthAccountRepository) Create(ctx context.Context, a *core.OAuthAccount) error {
	if a == nil {
		return fmt.Errorf("%w: oauth account is nil", core.ErrValidation)
	}
	createdAt := a.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	err := r.queries.CreateOAuthAccount(ctx, sqlitedb.CreateOAuthAccountParams{
		ID:         a.ID.String(),
		UserID:     a.UserID.String(),
		Provider:   a.Provider,
		ExternalID: a.ExternalID,
		Email:      a.Email,
		CreatedAt:  createdAt.UTC().Format(time.RFC3339),
	})
	if err != nil {
		if isUniqueViolation(err) {
			return core.ErrAlreadyExists
		}
		return err
	}
	return nil
}

// GetByExternalID возвращает OAuthAccount по (provider, externalID).
func (r *OAuthAccountRepository) GetByExternalID(ctx context.Context, provider, externalID string) (*core.OAuthAccount, error) {
	row, err := r.queries.GetOAuthAccountByExternalID(ctx, sqlitedb.GetOAuthAccountByExternalIDParams{
		Provider:   provider,
		ExternalID: externalID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return oauthAccountFromRow(row)
}

// ListByUser возвращает все OAuthAccounts пользователя.
func (r *OAuthAccountRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*core.OAuthAccount, error) {
	rows, err := r.queries.ListOAuthAccountsByUser(ctx, userID.String())
	if err != nil {
		return nil, err
	}
	out := make([]*core.OAuthAccount, 0, len(rows))
	for _, row := range rows {
		a, err := oauthAccountFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// Delete удаляет OAuthAccount по ID.
func (r *OAuthAccountRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.DeleteOAuthAccount(ctx, id.String())
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// oauthAccountFromRow конвертирует sqlitedb.OauthAccount в *core.OAuthAccount.
func oauthAccountFromRow(row sqlitedb.OauthAccount) (*core.OAuthAccount, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid oauth_account id: %w", err))
	}
	userID, err := uuid.Parse(row.UserID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid user_id: %w", err))
	}
	createdAt, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid created_at: %w", err))
	}
	return &core.OAuthAccount{
		ID:         id,
		UserID:     userID,
		Provider:   row.Provider,
		ExternalID: row.ExternalID,
		Email:      row.Email,
		CreatedAt:  createdAt,
	}, nil
}
