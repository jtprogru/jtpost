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

// OutboxRepository реализует core.OutboxRepository поверх SQLite.
type OutboxRepository struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// Outbox возвращает OutboxRepository поверх того же подключения.
func (r *PostRepository) Outbox() *OutboxRepository {
	return &OutboxRepository{db: r.db, queries: r.queries}
}

var _ core.OutboxRepository = (*OutboxRepository)(nil)

func (r *OutboxRepository) Enqueue(ctx context.Context, e *core.OutboxEntry) error {
	if e == nil {
		return fmt.Errorf("%w: outbox entry is nil", core.ErrValidation)
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	now := time.Now().UTC()
	if e.CreatedAt.IsZero() {
		e.CreatedAt = now
	}
	if e.UpdatedAt.IsZero() {
		e.UpdatedAt = now
	}
	if e.Status == "" {
		e.Status = core.OutboxStatusPending
	}
	if e.Kind == "" {
		e.Kind = core.OutboxKindPublish
	}
	if e.MaxAttempts == 0 {
		e.MaxAttempts = 5
	}
	if e.NextAttemptAt.IsZero() {
		e.NextAttemptAt = now
	}
	return r.queries.CreateOutbox(ctx, sqlitedb.CreateOutboxParams{
		ID:            e.ID.String(),
		PostID:        uuid.UUID(e.PostID).String(),
		TenantID:      e.TenantID.String(),
		Kind:          string(e.Kind),
		Status:        string(e.Status),
		Attempts:      int64(e.Attempts),
		MaxAttempts:   int64(e.MaxAttempts),
		NextAttemptAt: e.NextAttemptAt.UTC().Format(time.RFC3339Nano),
		LastError:     e.LastError,
		CreatedAt:     e.CreatedAt.UTC().Format(time.RFC3339Nano),
		UpdatedAt:     e.UpdatedAt.UTC().Format(time.RFC3339Nano),
	})
}

func (r *OutboxRepository) ClaimNext(ctx context.Context, now time.Time) (*core.OutboxEntry, error) {
	row, err := r.queries.ClaimNextOutbox(ctx, sqlitedb.ClaimNextOutboxParams{
		UpdatedAt:     now.UTC().Format(time.RFC3339Nano),
		NextAttemptAt: now.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return outboxFromRow(row)
}

func (r *OutboxRepository) MarkDone(ctx context.Context, id uuid.UUID, now time.Time) error {
	rows, err := r.queries.MarkOutboxDone(ctx, sqlitedb.MarkOutboxDoneParams{
		UpdatedAt: now.UTC().Format(time.RFC3339Nano),
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

func (r *OutboxRepository) MarkRetry(ctx context.Context, id uuid.UUID, attempts int, nextAt time.Time, errMsg string, now time.Time) error {
	rows, err := r.queries.MarkOutboxRetry(ctx, sqlitedb.MarkOutboxRetryParams{
		Attempts:      int64(attempts),
		NextAttemptAt: nextAt.UTC().Format(time.RFC3339Nano),
		LastError:     errMsg,
		UpdatedAt:     now.UTC().Format(time.RFC3339Nano),
		ID:            id.String(),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (r *OutboxRepository) MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, now time.Time) error {
	rows, err := r.queries.MarkOutboxFailed(ctx, sqlitedb.MarkOutboxFailedParams{
		LastError: errMsg,
		UpdatedAt: now.UTC().Format(time.RFC3339Nano),
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

func (r *OutboxRepository) List(ctx context.Context, filter core.OutboxFilter) ([]*core.OutboxEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	var rows []sqlitedb.OutboxEntry
	var err error
	if filter.Status != "" {
		rows, err = r.queries.ListOutboxByStatus(ctx, sqlitedb.ListOutboxByStatusParams{
			Status: string(filter.Status),
			Limit:  int64(limit),
		})
	} else {
		rows, err = r.queries.ListOutboxAll(ctx, int64(limit))
	}
	if err != nil {
		return nil, err
	}
	out := make([]*core.OutboxEntry, 0, len(rows))
	for _, row := range rows {
		e, convErr := outboxFromRow(row)
		if convErr != nil {
			return nil, convErr
		}
		out = append(out, e)
	}
	return out, nil
}

func (r *OutboxRepository) GetByID(ctx context.Context, id uuid.UUID) (*core.OutboxEntry, error) {
	row, err := r.queries.GetOutboxByID(ctx, id.String())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return outboxFromRow(row)
}

func (r *OutboxRepository) SweepStuck(ctx context.Context, threshold time.Duration, now time.Time) (int, error) {
	cutoff := now.Add(-threshold).UTC().Format(time.RFC3339Nano)
	rows, err := r.queries.SweepStuckOutbox(ctx, sqlitedb.SweepStuckOutboxParams{
		UpdatedAt:   now.UTC().Format(time.RFC3339Nano),
		UpdatedAt_2: cutoff,
	})
	if err != nil {
		return 0, err
	}
	return int(rows), nil
}

func outboxFromRow(row sqlitedb.OutboxEntry) (*core.OutboxEntry, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, fmt.Errorf("parse outbox id: %w", err)
	}
	postID, err := uuid.Parse(row.PostID)
	if err != nil {
		return nil, fmt.Errorf("parse post id: %w", err)
	}
	tenantID, err := uuid.Parse(row.TenantID)
	if err != nil {
		return nil, fmt.Errorf("parse tenant id: %w", err)
	}
	nextAt, err := time.Parse(time.RFC3339Nano, row.NextAttemptAt)
	if err != nil {
		return nil, fmt.Errorf("parse next_attempt_at: %w", err)
	}
	createdAt, _ := time.Parse(time.RFC3339Nano, row.CreatedAt)
	updatedAt, _ := time.Parse(time.RFC3339Nano, row.UpdatedAt)
	return &core.OutboxEntry{
		ID:            id,
		PostID:        core.PostID(postID),
		TenantID:      tenantID,
		Kind:          core.OutboxKind(row.Kind),
		Status:        core.OutboxStatus(row.Status),
		Attempts:      int(row.Attempts),
		MaxAttempts:   int(row.MaxAttempts),
		NextAttemptAt: nextAt,
		LastError:     row.LastError,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}, nil
}
