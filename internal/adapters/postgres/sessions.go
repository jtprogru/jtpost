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

// SessionRepository реализует core.SessionRepository поверх pgxpool.Pool.
type SessionRepository struct {
	pool    *pgxpool.Pool
	queries *pgdb.Queries
}

// Sessions возвращает SessionRepository поверх того же пула.
func (r *PostRepository) Sessions() *SessionRepository {
	return &SessionRepository{pool: r.pool, queries: r.queries}
}

var _ core.SessionRepository = (*SessionRepository)(nil)

// Create вставляет новую Session.
func (r *SessionRepository) Create(ctx context.Context, s *core.Session) error {
	if s == nil {
		return fmt.Errorf("%w: session is nil", core.ErrValidation)
	}
	createdAt := s.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	err := r.queries.CreateSession(ctx, pgdb.CreateSessionParams{
		ID:         toPgUUID(s.ID),
		UserID:     toPgUUID(s.UserID),
		Prefix:     s.Prefix,
		SecretHash: s.SecretHash,
		CsrfToken:  s.CSRFToken,
		CreatedAt:  toPgTimestampVal(createdAt),
		ExpiresAt:  toPgTimestampVal(s.ExpiresAt),
		LastUsedAt: toPgTimestamp(s.LastUsedAt),
	})
	if err != nil {
		if isPgUniqueViolation(err) {
			return core.ErrAlreadyExists
		}
		return err
	}
	return nil
}

// GetByPrefix возвращает Session по prefix.
func (r *SessionRepository) GetByPrefix(ctx context.Context, prefix string) (*core.Session, error) {
	row, err := r.queries.GetSessionByPrefix(ctx, prefix)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return sessionFromPgRow(row)
}

// Delete удаляет session по ID.
func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.DeleteSession(ctx, toPgUUID(id))
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// DeleteByUser удаляет все sessions пользователя.
func (r *SessionRepository) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	_, err := r.queries.DeleteSessionsByUser(ctx, toPgUUID(userID))
	return err
}

// UpdateLastUsedAt обновляет last_used_at у session.
func (r *SessionRepository) UpdateLastUsedAt(ctx context.Context, id uuid.UUID, t time.Time) error {
	last := t
	rows, err := r.queries.UpdateSessionLastUsedAt(ctx, pgdb.UpdateSessionLastUsedAtParams{
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

// UpdateCSRFToken обновляет csrf_token у session.
func (r *SessionRepository) UpdateCSRFToken(ctx context.Context, id uuid.UUID, csrf string) error {
	rows, err := r.queries.UpdateSessionCSRFToken(ctx, pgdb.UpdateSessionCSRFTokenParams{
		CsrfToken: csrf,
		ID:        toPgUUID(id),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// sessionFromPgRow конвертирует pgdb.Session в *core.Session.
func sessionFromPgRow(row pgdb.Session) (*core.Session, error) {
	return &core.Session{
		ID:         fromPgUUID(row.ID),
		UserID:     fromPgUUID(row.UserID),
		Prefix:     row.Prefix,
		SecretHash: row.SecretHash,
		CSRFToken:  row.CsrfToken,
		CreatedAt:  row.CreatedAt.Time,
		ExpiresAt:  row.ExpiresAt.Time,
		LastUsedAt: fromPgTimestamp(row.LastUsedAt),
	}, nil
}
