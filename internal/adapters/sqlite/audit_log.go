package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/adapters/sqlite/sqlitedb"
	"github.com/jtprogru/jtpost/internal/core"
)

// AuditLogRepository реализует core.AuditRepository поверх SQLite.
type AuditLogRepository struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// AuditLog возвращает AuditLogRepository поверх того же подключения.
func (r *PostRepository) AuditLog() *AuditLogRepository {
	return &AuditLogRepository{db: r.db, queries: r.queries}
}

var _ core.AuditRepository = (*AuditLogRepository)(nil)

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
	meta := ""
	if len(e.Metadata) > 0 {
		raw, err := json.Marshal(e.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		meta = string(raw)
	}
	tenant := ""
	if e.TenantID != uuid.Nil {
		tenant = e.TenantID.String()
	}
	actor := ""
	if e.ActorID != uuid.Nil {
		actor = e.ActorID.String()
	}
	return r.queries.AppendAuditLog(ctx, sqlitedb.AppendAuditLogParams{
		ID:           e.ID.String(),
		OccurredAt:   e.OccurredAt.UTC().Format(time.RFC3339Nano),
		TenantID:     tenant,
		ActorID:      actor,
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
	tenant := ""
	if filter.TenantID != uuid.Nil {
		tenant = filter.TenantID.String()
	}
	actor := ""
	if filter.ActorID != uuid.Nil {
		actor = filter.ActorID.String()
	}
	rows, err := r.queries.ListAuditLog(ctx, sqlitedb.ListAuditLogParams{
		TenantID: tenant,
		ActorID:  actor,
		Action:   string(filter.Action),
		Lim:      int64(limit),
	})
	if err != nil {
		return nil, err
	}
	out := make([]*core.AuditEntry, 0, len(rows))
	for _, row := range rows {
		e, err := auditFromRow(row)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func auditFromRow(row sqlitedb.AuditLog) (*core.AuditEntry, error) {
	id, err := uuid.Parse(row.ID)
	if err != nil {
		return nil, fmt.Errorf("parse audit id: %w", err)
	}
	occurredAt, err := time.Parse(time.RFC3339Nano, row.OccurredAt)
	if err != nil {
		return nil, fmt.Errorf("parse occurred_at: %w", err)
	}
	var tenant uuid.UUID
	if row.TenantID != "" {
		tenant, _ = uuid.Parse(row.TenantID)
	}
	var actor uuid.UUID
	if row.ActorID != "" {
		actor, _ = uuid.Parse(row.ActorID)
	}
	var meta map[string]any
	if row.Metadata != "" {
		_ = json.Unmarshal([]byte(row.Metadata), &meta)
	}
	return &core.AuditEntry{
		ID:           id,
		OccurredAt:   occurredAt,
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
	}, nil
}
