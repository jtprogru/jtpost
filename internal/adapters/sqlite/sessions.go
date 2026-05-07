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

// SessionRepository реализует core.SessionRepository поверх SQLite.
type SessionRepository struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// Sessions возвращает SessionRepository поверх того же подключения.
func (r *PostRepository) Sessions() *SessionRepository {
	return &SessionRepository{db: r.db, queries: r.queries}
}

// Compile-time проверка соответствия интерфейсу.
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
	err := r.queries.CreateSession(ctx, sqlitedb.CreateSessionParams{
		ID:         s.ID.String(),
		UserID:     s.UserID.String(),
		Prefix:     s.Prefix,
		SecretHash: s.SecretHash,
		CsrfToken:  s.CSRFToken,
		CreatedAt:  createdAt.UTC().Format(time.RFC3339),
		ExpiresAt:  s.ExpiresAt.UTC().Format(time.RFC3339),
		LastUsedAt: nullableTime(s.LastUsedAt),
	})
	if err != nil {
		if isUniqueViolation(err) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return sessionFromRow(row)
}

// Delete удаляет session по ID.
func (r *SessionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	rows, err := r.queries.DeleteSession(ctx, id.String())
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
	_, err := r.queries.DeleteSessionsByUser(ctx, userID.String())
	return err
}

// UpdateLastUsedAt обновляет last_used_at у session.
func (r *SessionRepository) UpdateLastUsedAt(ctx context.Context, id uuid.UUID, t time.Time) error {
	last := t
	rows, err := r.queries.UpdateSessionLastUsedAt(ctx, sqlitedb.UpdateSessionLastUsedAtParams{
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

// UpdateCSRFToken обновляет csrf_token у session.
func (r *SessionRepository) UpdateCSRFToken(ctx context.Context, id uuid.UUID, csrf string) error {
	rows, err := r.queries.UpdateSessionCSRFToken(ctx, sqlitedb.UpdateSessionCSRFTokenParams{
		CsrfToken: csrf,
		ID:        id.String(),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// sessionFromRow конвертирует sqlitedb.Session в *core.Session.
func sessionFromRow(row sqlitedb.Session) (*core.Session, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid session id: %w", err))
	}
	userID, err := uuid.Parse(row.UserID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid user_id: %w", err))
	}
	createdAt, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid created_at: %w", err))
	}
	expiresAt, err := time.Parse(time.RFC3339, row.ExpiresAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid expires_at: %w", err))
	}
	lastUsedAt, err := parseTimeNS(row.LastUsedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid last_used_at: %w", err))
	}
	return &core.Session{
		ID:         id,
		UserID:     userID,
		Prefix:     row.Prefix,
		SecretHash: row.SecretHash,
		CSRFToken:  row.CsrfToken,
		CreatedAt:  createdAt,
		ExpiresAt:  expiresAt,
		LastUsedAt: lastUsedAt,
	}, nil
}
