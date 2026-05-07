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

type OutboxRepository struct {
	pool    *pgxpool.Pool
	queries *pgdb.Queries
}

func (r *PostRepository) Outbox() *OutboxRepository {
	return &OutboxRepository{pool: r.pool, queries: r.queries}
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
	return r.queries.CreateOutbox(ctx, pgdb.CreateOutboxParams{
		ID:            toPgUUID(e.ID),
		PostID:        toPgUUID(uuid.UUID(e.PostID)),
		TenantID:      toPgUUID(e.TenantID),
		Kind:          string(e.Kind),
		Status:        string(e.Status),
		Attempts:      int32(e.Attempts),
		MaxAttempts:   int32(e.MaxAttempts),
		NextAttemptAt: toPgTimestampVal(e.NextAttemptAt),
		LastError:     e.LastError,
		CreatedAt:     toPgTimestampVal(e.CreatedAt),
		UpdatedAt:     toPgTimestampVal(e.UpdatedAt),
	})
}

func (r *OutboxRepository) ClaimNext(ctx context.Context, now time.Time) (*core.OutboxEntry, error) {
	row, err := r.queries.ClaimNextOutbox(ctx, pgdb.ClaimNextOutboxParams{
		UpdatedAt:     toPgTimestampVal(now),
		NextAttemptAt: toPgTimestampVal(now),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return outboxFromPgRow(row), nil
}

func (r *OutboxRepository) MarkDone(ctx context.Context, id uuid.UUID, now time.Time) error {
	rows, err := r.queries.MarkOutboxDone(ctx, pgdb.MarkOutboxDoneParams{
		UpdatedAt: toPgTimestampVal(now),
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

func (r *OutboxRepository) MarkRetry(ctx context.Context, id uuid.UUID, attempts int, nextAt time.Time, errMsg string, now time.Time) error {
	rows, err := r.queries.MarkOutboxRetry(ctx, pgdb.MarkOutboxRetryParams{
		Attempts:      int32(attempts),
		NextAttemptAt: toPgTimestampVal(nextAt),
		LastError:     errMsg,
		UpdatedAt:     toPgTimestampVal(now),
		ID:            toPgUUID(id),
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
	rows, err := r.queries.MarkOutboxFailed(ctx, pgdb.MarkOutboxFailedParams{
		LastError: errMsg,
		UpdatedAt: toPgTimestampVal(now),
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

func (r *OutboxRepository) List(ctx context.Context, filter core.OutboxFilter) ([]*core.OutboxEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	var rows []pgdb.OutboxEntry
	var err error
	if filter.Status != "" {
		rows, err = r.queries.ListOutboxByStatus(ctx, pgdb.ListOutboxByStatusParams{
			Status: string(filter.Status),
			Limit:  int32(limit),
		})
	} else {
		rows, err = r.queries.ListOutboxAll(ctx, int32(limit))
	}
	if err != nil {
		return nil, err
	}
	out := make([]*core.OutboxEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, outboxFromPgRow(row))
	}
	return out, nil
}

func (r *OutboxRepository) GetByID(ctx context.Context, id uuid.UUID) (*core.OutboxEntry, error) {
	row, err := r.queries.GetOutboxByID(ctx, toPgUUID(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return outboxFromPgRow(row), nil
}

func outboxFromPgRow(row pgdb.OutboxEntry) *core.OutboxEntry {
	var nextAt, createdAt, updatedAt time.Time
	if row.NextAttemptAt.Valid {
		nextAt = row.NextAttemptAt.Time
	}
	if row.CreatedAt.Valid {
		createdAt = row.CreatedAt.Time
	}
	if row.UpdatedAt.Valid {
		updatedAt = row.UpdatedAt.Time
	}
	return &core.OutboxEntry{
		ID:            fromPgUUID(row.ID),
		PostID:        core.PostID(fromPgUUID(row.PostID)),
		TenantID:      fromPgUUID(row.TenantID),
		Kind:          core.OutboxKind(row.Kind),
		Status:        core.OutboxStatus(row.Status),
		Attempts:      int(row.Attempts),
		MaxAttempts:   int(row.MaxAttempts),
		NextAttemptAt: nextAt,
		LastError:     row.LastError,
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
	}
}
