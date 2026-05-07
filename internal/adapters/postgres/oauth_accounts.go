package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jtprogru/jtpost/internal/adapters/postgres/pgdb"
	"github.com/jtprogru/jtpost/internal/core"
)

// OAuthAccountRepository реализует core.OAuthAccountRepository поверх pgxpool.Pool.
type OAuthAccountRepository struct {
	pool    *pgxpool.Pool
	queries *pgdb.Queries
}

// OAuthAccounts возвращает OAuthAccountRepository поверх того же пула.
func (r *PostRepository) OAuthAccounts() *OAuthAccountRepository {
	return &OAuthAccountRepository{pool: r.pool, queries: r.queries}
}

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
	err := r.queries.CreateOAuthAccount(ctx, pgdb.CreateOAuthAccountParams{
		ID:         toPgUUID(a.ID),
		UserID:     toPgUUID(a.UserID),
		Provider:   a.Provider,
		ExternalID: a.ExternalID,
		Email:      a.Email,
		CreatedAt:  toPgTimestampVal(createdAt),
	})
	if err != nil {
		if isPgUniqueViolation(err) {
			return core.ErrAlreadyExists
		}
		return err
	}
	return nil
}

// GetByExternalID возвращает OAuthAccount по (provider, externalID).
func (r *OAuthAccountRepository) GetByExternalID(ctx context.Context, provider, externalID string) (*core.OAuthAccount, error) {
	row, err := r.queries.GetOAuthAccountByExternalID(ctx, pgdb.GetOAuthAccountByExternalIDParams{
		Provider:   provider,
		ExternalID: externalID,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return oauthAccountFromPgRow(row), nil
}

// ListByUser возвращает все OAuthAccounts пользователя.
func (r *OAuthAccountRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*core.OAuthAccount, error) {
	rows, err := r.queries.ListOAuthAccountsByUser(ctx, toPgUUID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]*core.OAuthAccount, 0, len(rows))
	for _, row := range rows {
		out = append(out, oauthAccountFromPgRow(row))
	}
	return out, nil
}

// Delete удаляет OAuthAccount по ID.
func (r *OAuthAccountRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.DeleteOAuthAccount(ctx, toPgUUID(id))
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// oauthAccountFromPgRow конвертирует pgdb.OauthAccount в *core.OAuthAccount.
func oauthAccountFromPgRow(row pgdb.OauthAccount) *core.OAuthAccount {
	return &core.OAuthAccount{
		ID:         fromPgUUID(row.ID),
		UserID:     fromPgUUID(row.UserID),
		Provider:   row.Provider,
		ExternalID: row.ExternalID,
		Email:      row.Email,
		CreatedAt:  row.CreatedAt.Time,
	}
}
