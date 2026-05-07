//go:build integration

package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/core"
)

func newPgRepoForOutbox(t *testing.T) *PostRepository {
	t.Helper()
	dsn := setupContainer(t)
	repo, err := NewPostgresRepository(context.Background(), Config{DSN: dsn, MaxOpenConns: 4, MaxIdleConns: 1, ConnMaxLifetime: 5 * time.Minute})
	if err != nil {
		t.Fatalf("NewPostgresRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func makePgOutboxEntry() *core.OutboxEntry {
	// NextAttemptAt в прошлом: гарантирует claim-success даже при дрейфе
	// между моментом захвата `now` в тесте и моментом Enqueue (важно в CI).
	return &core.OutboxEntry{
		PostID:        core.PostID(uuid.New()),
		TenantID:      uuid.New(),
		Kind:          core.OutboxKindPublish,
		Status:        core.OutboxStatusPending,
		MaxAttempts:   3,
		NextAttemptAt: time.Now().UTC().Add(-time.Second),
	}
}

func TestPostgresOutbox_FullLifecycle(t *testing.T) {
	r := newPgRepoForOutbox(t)
	ctx := context.Background()
	box := r.Outbox()
	now := time.Now().UTC()

	e := makePgOutboxEntry()
	if err := box.Enqueue(ctx, e); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	got, err := box.GetByID(ctx, e.ID)
	if err != nil || got.Status != core.OutboxStatusPending {
		t.Fatalf("GetByID: %v %v", got, err)
	}
	claimed, err := box.ClaimNext(ctx, now.Add(time.Minute))
	if err != nil || claimed == nil || claimed.ID != e.ID || claimed.Status != core.OutboxStatusInFlight {
		t.Fatalf("ClaimNext: %v %v", claimed, err)
	}
	if err := box.MarkRetry(ctx, e.ID, 1, now.Add(time.Hour), "boom", now); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}
	got2, _ := box.GetByID(ctx, e.ID)
	if got2.Status != core.OutboxStatusPending || got2.Attempts != 1 || got2.LastError != "boom" {
		t.Errorf("retry state wrong: %+v", got2)
	}
	if err := box.MarkFailed(ctx, e.ID, "permanent", now); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	got3, _ := box.GetByID(ctx, e.ID)
	if got3.Status != core.OutboxStatusFailed {
		t.Errorf("expected failed, got %s", got3.Status)
	}
	if err := box.MarkDone(ctx, uuid.New(), now); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgresOutbox_ClaimOrdering(t *testing.T) {
	r := newPgRepoForOutbox(t)
	ctx := context.Background()
	box := r.Outbox()
	now := time.Now().UTC()
	e1 := makePgOutboxEntry()
	e1.NextAttemptAt = now.Add(-2 * time.Minute)
	e2 := makePgOutboxEntry()
	e2.NextAttemptAt = now.Add(-1 * time.Minute)
	_ = box.Enqueue(ctx, e1)
	_ = box.Enqueue(ctx, e2)
	c1, _ := box.ClaimNext(ctx, now)
	if c1 == nil || c1.ID != e1.ID {
		t.Errorf("expected e1 first, got %v", c1)
	}
	c2, _ := box.ClaimNext(ctx, now)
	if c2 == nil || c2.ID != e2.ID {
		t.Errorf("expected e2 second, got %v", c2)
	}
	c3, _ := box.ClaimNext(ctx, now)
	if c3 != nil {
		t.Errorf("expected empty third claim, got %v", c3)
	}
}

func TestPostgresOutbox_SweepStuck(t *testing.T) {
	r := newPgRepoForOutbox(t)
	ctx := context.Background()
	box := r.Outbox()
	now := time.Now().UTC()

	stuck := makePgOutboxEntry()
	_ = box.Enqueue(ctx, stuck)
	_, _ = box.ClaimNext(ctx, now)
	if _, err := r.pool.Exec(ctx, "UPDATE outbox_entries SET updated_at = $1 WHERE id = $2",
		now.Add(-30*time.Minute), stuck.ID); err != nil {
		t.Fatal(err)
	}
	fresh := makePgOutboxEntry()
	_ = box.Enqueue(ctx, fresh)
	_, _ = box.ClaimNext(ctx, now)

	n, err := box.SweepStuck(ctx, 10*time.Minute, now)
	if err != nil {
		t.Fatalf("SweepStuck: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 swept, got %d", n)
	}
	g1, _ := box.GetByID(ctx, stuck.ID)
	if g1.Status != core.OutboxStatusPending {
		t.Errorf("stuck should be pending, got %s", g1.Status)
	}
	g2, _ := box.GetByID(ctx, fresh.ID)
	if g2.Status != core.OutboxStatusInFlight {
		t.Errorf("fresh should remain in_flight, got %s", g2.Status)
	}
}

func TestPostgresOutbox_List(t *testing.T) {
	r := newPgRepoForOutbox(t)
	ctx := context.Background()
	box := r.Outbox()
	for i := 0; i < 3; i++ {
		_ = box.Enqueue(ctx, makePgOutboxEntry())
	}
	all, err := box.List(ctx, core.OutboxFilter{})
	if err != nil || len(all) != 3 {
		t.Fatalf("expected 3, got %d (%v)", len(all), err)
	}
	pending, _ := box.List(ctx, core.OutboxFilter{Status: core.OutboxStatusPending})
	if len(pending) != 3 {
		t.Errorf("expected 3 pending, got %d", len(pending))
	}
}
