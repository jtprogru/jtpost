package core

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// OutboxStatus — статус outbox-записи.
type OutboxStatus string

const (
	OutboxStatusPending  OutboxStatus = "pending"
	OutboxStatusInFlight OutboxStatus = "in_flight"
	OutboxStatusDone     OutboxStatus = "done"
	OutboxStatusFailed   OutboxStatus = "failed"
)

// OutboxKind — тип задачи.
type OutboxKind string

const OutboxKindPublish OutboxKind = "publish"

// OutboxEntry — запись очереди публикации.
type OutboxEntry struct {
	ID            uuid.UUID
	PostID        PostID
	TenantID      uuid.UUID
	Kind          OutboxKind
	Status        OutboxStatus
	Attempts      int
	MaxAttempts   int
	NextAttemptAt time.Time
	LastError     string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// OutboxFilter параметры выборки для List.
type OutboxFilter struct {
	Status OutboxStatus
	Limit  int
}

// OutboxRepository интерфейс для outbox storage.
type OutboxRepository interface {
	Enqueue(ctx context.Context, entry *OutboxEntry) error
	ClaimNext(ctx context.Context, now time.Time) (*OutboxEntry, error)
	MarkDone(ctx context.Context, id uuid.UUID, now time.Time) error
	MarkRetry(ctx context.Context, id uuid.UUID, attempts int, nextAt time.Time, errMsg string, now time.Time) error
	MarkFailed(ctx context.Context, id uuid.UUID, errMsg string, now time.Time) error
	List(ctx context.Context, filter OutboxFilter) ([]*OutboxEntry, error)
	GetByID(ctx context.Context, id uuid.UUID) (*OutboxEntry, error)
	// SweepStuck сбрасывает в pending все in_flight записи, у которых
	// updated_at < now-threshold. Возвращает кол-во reset'нутых записей.
	SweepStuck(ctx context.Context, threshold time.Duration, now time.Time) (int, error)
}

// DefaultBackoffSchedule — exponential schedule per attempt index (0-based).
// Если attempts превышает len(schedule) — берём последний.
//
//nolint:gochecknoglobals // экспортированный default для WorkerConfig.BackoffSchedule
var DefaultBackoffSchedule = []time.Duration{
	1 * time.Minute,
	5 * time.Minute,
	25 * time.Minute,
	2 * time.Hour,
	8 * time.Hour,
}

// ComputeBackoff возвращает delay для следующей попытки. attempts — кол-во
// уже сделанных попыток (после неуспешной).
func ComputeBackoff(schedule []time.Duration, attempts int) time.Duration {
	if len(schedule) == 0 {
		return 1 * time.Minute
	}
	idx := max(attempts-1, 0)
	if idx >= len(schedule) {
		idx = len(schedule) - 1
	}
	return schedule[idx]
}
