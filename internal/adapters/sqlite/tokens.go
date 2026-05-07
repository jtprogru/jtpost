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

// TokenRepository реализует core.TokenRepository поверх SQLite.
type TokenRepository struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// Tokens возвращает TokenRepository поверх того же подключения.
func (r *PostRepository) Tokens() *TokenRepository {
	return &TokenRepository{db: r.db, queries: r.queries}
}

// Compile-time проверка соответствия интерфейсу.
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
	err := r.queries.CreateToken(ctx, sqlitedb.CreateTokenParams{
		ID:         t.ID.String(),
		UserID:     t.UserID.String(),
		Prefix:     t.Prefix,
		SecretHash: t.SecretHash,
		Name:       t.Name,
		CreatedAt:  createdAt.UTC().Format(time.RFC3339),
		ExpiresAt:  nullableTime(t.ExpiresAt),
		LastUsedAt: nullableTime(t.LastUsedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return tokenFromRow(row)
}

// Delete удаляет токен по ID.
func (r *TokenRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.DeleteToken(ctx, id.String())
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// ListByUser возвращает токены пользователя, ORDER BY created_at ASC.
func (r *TokenRepository) ListByUser(ctx context.Context, userID uuid.UUID) ([]*core.APIToken, error) {
	rows, err := r.queries.ListTokensByUser(ctx, userID.String())
	if err != nil {
		return nil, err
	}
	out := make([]*core.APIToken, 0, len(rows))
	for _, row := range rows {
		tk, err := tokenFromRow(row)
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
	rows, err := r.queries.UpdateTokenLastUsedAt(ctx, sqlitedb.UpdateTokenLastUsedAtParams{
		LastUsedAt: nullableTime(&last),
		ID:         id.String(),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// tokenFromRow конвертирует sqlitedb.Token в *core.APIToken.
func tokenFromRow(row sqlitedb.Token) (*core.APIToken, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid token id: %w", err))
	}
	userID, err := uuid.Parse(row.UserID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid user_id: %w", err))
	}
	createdAt, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid created_at: %w", err))
	}
	expiresAt, err := parseTimeNS(row.ExpiresAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid expires_at: %w", err))
	}
	lastUsedAt, err := parseTimeNS(row.LastUsedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid last_used_at: %w", err))
	}
	return &core.APIToken{
		ID:         id,
		UserID:     userID,
		Prefix:     row.Prefix,
		SecretHash: row.SecretHash,
		Name:       row.Name,
		CreatedAt:  createdAt,
		ExpiresAt:  expiresAt,
		LastUsedAt: lastUsedAt,
	}, nil
}
