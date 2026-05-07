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

// TokenRepository реализует core.TokenRepository поверх pgxpool.Pool.
type TokenRepository struct {
	pool    *pgxpool.Pool
	queries *pgdb.Queries
}

// Tokens возвращает TokenRepository поверх того же пула.
func (r *PostRepository) Tokens() *TokenRepository {
	return &TokenRepository{pool: r.pool, queries: r.queries}
}

var _ core.TokenRepository = (*TokenRepository)(nil)

// Create вставляет новый APIToken.
func (r *TokenRepository) Create(ctx context.Context, t *core.APIToken) error {
	if t == nil {
		return fmt.Errorf("%w: token is nil", core.ErrValidation)
	}
	createdAt := t.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	err := r.queries.CreateToken(ctx, pgdb.CreateTokenParams{
		ID:         toPgUUID(t.ID),
		UserID:     toPgUUID(t.UserID),
		Prefix:     t.Prefix,
		SecretHash: t.SecretHash,
		Name:       t.Name,
		CreatedAt:  toPgTimestampVal(createdAt),
		ExpiresAt:  toPgTimestamp(t.ExpiresAt),
		LastUsedAt: toPgTimestamp(t.LastUsedAt),
	})
	if err != nil {
		if isPgUniqueViolation(err) {
			return core.ErrAlreadyExists
		}
		return err
	}
	return nil
}

// GetByPrefix возвращает APIToken по prefix.
func (r *TokenRepository) GetByPrefix(ctx context.Context, prefix string) (*core.APIToken, error) {
	row, err := r.queries.GetTokenByPrefix(ctx, prefix)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return tokenFromPgRow(row)
}

// Delete удаляет токен.
func (r *TokenRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.DeleteToken(ctx, toPgUUID(id))
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// ListByUser возвращает токены пользователя.
func (r *TokenRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*core.APIToken, error) {
	rows, err := r.queries.ListTokensByUser(ctx, toPgUUID(userID))
	if err != nil {
		return nil, err
	}
	out := make([]*core.APIToken, 0, len(rows))
	for _, row := range rows {
		tk, err := tokenFromPgRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, tk)
	}
	return out, nil
}

// UpdateLastUsedAt обновляет last_used_at у токена.
func (r *TokenRepository) UpdateLastUsedAt(ctx context.Context, id uuid.UUID, t time.Time) error {
	last := t
	rows, err := r.queries.UpdateTokenLastUsedAt(ctx, pgdb.UpdateTokenLastUsedAtParams{
		LastUsedAt: toPgTimestamp(&last),
		ID:         toPgUUID(id),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// tokenFromPgRow конвертирует pgdb.Token в *core.APIToken.
func tokenFromPgRow(row pgdb.Token) (*core.APIToken, error) {
	return &core.APIToken{
		ID:         fromPgUUID(row.ID),
		UserID:     fromPgUUID(row.UserID),
		Prefix:     row.Prefix,
		SecretHash: row.SecretHash,
		Name:       row.Name,
		CreatedAt:  row.CreatedAt.Time,
		ExpiresAt:  fromPgTimestamp(row.ExpiresAt),
		LastUsedAt: fromPgTimestamp(row.LastUsedAt),
	}, nil
}

// pool/queries fields are also kept for future extension; reference once for vet.
var _ = (*pgxpool.Pool)(nil)
