package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jtprogru/jtpost/internal/adapters/postgres/pgdb"
	"github.com/jtprogru/jtpost/internal/core"
)

// AuditLogRepository реализует core.AuditRepository поверх Postgres.
type AuditLogRepository struct {
	pool    *pgxpool.Pool
	queries *pgdb.Queries
}

// AuditLog возвращает AuditLogRepository поверх того же подключения.
func (r *PostRepository) AuditLog() *AuditLogRepository {
	return &AuditLogRepository{pool: r.pool, queries: r.queries}
}

var _ core.AuditRepository = (*AuditLogRepository)(nil)

func nullableUUID(u uuid.UUID) pgtype.UUID {
	if u == uuid.Nil {
		return pgtype.UUID{Valid: false}
	}
	return pgtype.UUID{Bytes: u, Valid: true}
}

func (r *AuditLogRepository) Append(ctx context.Context, e *core.AuditEntry) error {
	if e == nil {
		return fmt.Errorf("%w: audit entry is nil", core.ErrValidation)
	}
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = time.Now().UTC()
	}
	meta := []byte("{}")
	if len(e.Metadata) > 0 {
		raw, err := json.Marshal(e.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		meta = raw
	}
	return r.queries.AppendAuditLog(ctx, pgdb.AppendAuditLogParams{
		ID:           toPgUUID(e.ID),
		OccurredAt:   pgtype.Timestamptz{Time: e.OccurredAt.UTC(), Valid: true},
		TenantID:     nullableUUID(e.TenantID),
		ActorID:      nullableUUID(e.ActorID),
		ActorType:    string(e.ActorType),
		Action:       string(e.Action),
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		Outcome:      string(e.Outcome),
		Ip:           e.IP,
		UserAgent:    e.UserAgent,
		Metadata:     meta,
	})
}

func (r *AuditLogRepository) List(ctx context.Context, filter core.AuditFilter) ([]*core.AuditEntry, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}
	var actionParam pgtype.Text
	if filter.Action != "" {
		actionParam = pgtype.Text{String: string(filter.Action), Valid: true}
	}
	rows, err := r.queries.ListAuditLog(ctx, pgdb.ListAuditLogParams{
		TenantID: nullableUUID(filter.TenantID),
		ActorID:  nullableUUID(filter.ActorID),
		Action:   actionParam,
		Lim:      int32(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*core.AuditEntry, 0, len(rows))
	for _, row := range rows {
		out = append(out, auditFromRow(row))
	}
	return out, nil
}

func auditFromRow(row pgdb.AuditLog) *core.AuditEntry {
	var meta map[string]any
	if len(row.Metadata) > 0 {
		_ = json.Unmarshal(row.Metadata, &meta)
	}
	var tenant uuid.UUID
	if row.TenantID.Valid {
		tenant = fromPgUUID(row.TenantID)
	}
	var actor uuid.UUID
	if row.ActorID.Valid {
		actor = fromPgUUID(row.ActorID)
	}
	return &core.AuditEntry{
		ID:           fromPgUUID(row.ID),
		OccurredAt:   row.OccurredAt.Time,
		TenantID:     tenant,
		ActorID:      actor,
		ActorType:    core.AuditActorType(row.ActorType),
		Action:       core.AuditAction(row.Action),
		ResourceType: row.ResourceType,
		ResourceID:   row.ResourceID,
		Outcome:      core.AuditOutcome(row.Outcome),
		IP:           row.Ip,
		UserAgent:    row.UserAgent,
		Metadata:     meta,
	}
}
