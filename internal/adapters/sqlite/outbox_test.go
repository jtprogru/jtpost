package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/core"
)

func makeOutboxEntry() *core.OutboxEntry {
	return &core.OutboxEntry{
		PostID:        core.PostID(uuid.New()),
		TenantID:      uuid.New(),
		Kind:          core.OutboxKindPublish,
		Status:        core.OutboxStatusPending,
		MaxAttempts:   3,
		NextAttemptAt: time.Now().UTC(),
	}
}

func TestSQLiteOutbox_EnqueueAndGetByID(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	box := r.Outbox()

	e := makeOutboxEntry()
	if err := box.Enqueue(ctx, e); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if e.ID == uuid.Nil {
		t.Error("expected ID assigned")
	}
	got, err := box.GetByID(ctx, e.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.PostID != e.PostID || got.Status != core.OutboxStatusPending {
		t.Errorf("got %+v want match", got)
	}
}

func TestSQLiteOutbox_ClaimNext_AtomicOrdering(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	box := r.Outbox()

	now := time.Now().UTC()
	e1 := makeOutboxEntry()
	e1.NextAttemptAt = now.Add(-2 * time.Minute)
	e2 := makeOutboxEntry()
	e2.NextAttemptAt = now.Add(-1 * time.Minute)
	if err := box.Enqueue(ctx, e1); err != nil {
		t.Fatal(err)
	}
	if err := box.Enqueue(ctx, e2); err != nil {
		t.Fatal(err)
	}

	claimed1, err := box.ClaimNext(ctx, now)
	if err != nil || claimed1 == nil {
		t.Fatalf("first claim: %v %v", claimed1, err)
	}
	if claimed1.ID != e1.ID {
		t.Errorf("expected oldest first (e1), got %s", claimed1.ID)
	}
	if claimed1.Status != core.OutboxStatusInFlight {
		t.Errorf("expected in_flight, got %s", claimed1.Status)
	}
	claimed2, err := box.ClaimNext(ctx, now)
	if err != nil || claimed2 == nil {
		t.Fatalf("second claim: %v %v", claimed2, err)
	}
	if claimed2.ID != e2.ID {
		t.Errorf("expected e2 next, got %s", claimed2.ID)
	}
	// Third claim — empty.
	c3, err := box.ClaimNext(ctx, now)
	if err != nil || c3 != nil {
		t.Errorf("expected empty third claim, got %v %v", c3, err)
	}
}

func TestSQLiteOutbox_ClaimNext_RespectsNextAttemptAt(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	box := r.Outbox()
	e := makeOutboxEntry()
	e.NextAttemptAt = time.Now().UTC().Add(1 * time.Hour)
	if err := box.Enqueue(ctx, e); err != nil {
		t.Fatal(err)
	}
	c, err := box.ClaimNext(ctx, time.Now().UTC())
	if err != nil || c != nil {
		t.Errorf("expected nil claim (future entry), got %v %v", c, err)
	}
}

func TestSQLiteOutbox_MarkDone_MarkRetry_MarkFailed(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	box := r.Outbox()
	e := makeOutboxEntry()
	if err := box.Enqueue(ctx, e); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	if err := box.MarkDone(ctx, e.ID, now); err != nil {
		t.Fatalf("MarkDone: %v", err)
	}
	got, _ := box.GetByID(ctx, e.ID)
	if got.Status != core.OutboxStatusDone {
		t.Errorf("expected done, got %s", got.Status)
	}

	e2 := makeOutboxEntry()
	_ = box.Enqueue(ctx, e2)
	if err := box.MarkRetry(ctx, e2.ID, 2, now.Add(time.Minute), "boom", now); err != nil {
		t.Fatalf("MarkRetry: %v", err)
	}
	got2, _ := box.GetByID(ctx, e2.ID)
	if got2.Status != core.OutboxStatusPending || got2.Attempts != 2 || got2.LastError != "boom" {
		t.Errorf("retry state wrong: %+v", got2)
	}

	if err := box.MarkFailed(ctx, e2.ID, "permanent", now); err != nil {
		t.Fatalf("MarkFailed: %v", err)
	}
	got3, _ := box.GetByID(ctx, e2.ID)
	if got3.Status != core.OutboxStatusFailed {
		t.Errorf("expected failed, got %s", got3.Status)
	}

	// MarkDone несуществующего id.
	if err := box.MarkDone(ctx, uuid.New(), now); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestSQLiteOutbox_SweepStuck(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	box := r.Outbox()
	now := time.Now().UTC()

	// stuck entry — claim'нутая 30m назад.
	stuck := makeOutboxEntry()
	stuck.NextAttemptAt = now.Add(-30 * time.Minute)
	if err := box.Enqueue(ctx, stuck); err != nil {
		t.Fatal(err)
	}
	if _, err := box.ClaimNext(ctx, now.Add(-29*time.Minute)); err != nil {
		t.Fatal(err)
	}
	// Pretend updated_at — давно (имитируем через прямой UPDATE).
	if _, err := r.db.ExecContext(ctx,
		"UPDATE outbox_entries SET updated_at = ? WHERE id = ?",
		now.Add(-30*time.Minute).Format(time.RFC3339Nano), stuck.ID.String()); err != nil {
		t.Fatal(err)
	}

	// fresh entry — claim'нутая прямо сейчас.
	fresh := makeOutboxEntry()
	fresh.NextAttemptAt = now.Add(-time.Second)
	if err := box.Enqueue(ctx, fresh); err != nil {
		t.Fatal(err)
	}
	claimed, err := box.ClaimNext(ctx, now)
	if err != nil || claimed == nil || claimed.ID != fresh.ID {
		t.Fatalf("expected to claim fresh, got %v err=%v", claimed, err)
	}

	// Sweep с threshold=10m → должна попасть только stuck.
	n, err := box.SweepStuck(ctx, 10*time.Minute, now)
	if err != nil {
		t.Fatalf("SweepStuck: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 swept, got %d", n)
	}
	got1, _ := box.GetByID(ctx, stuck.ID)
	if got1.Status != core.OutboxStatusPending {
		t.Errorf("stuck should be pending, got %s", got1.Status)
	}
	got2, _ := box.GetByID(ctx, fresh.ID)
	if got2.Status != core.OutboxStatusInFlight {
		t.Errorf("fresh should remain in_flight, got %s", got2.Status)
	}
}

func TestSQLiteOutbox_List(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	box := r.Outbox()
	for range 3 {
		_ = box.Enqueue(ctx, makeOutboxEntry())
	}
	all, err := box.List(ctx, core.OutboxFilter{})
	if err != nil || len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d %v", len(all), err)
	}
	pending, _ := box.List(ctx, core.OutboxFilter{Status: core.OutboxStatusPending})
	if len(pending) != 3 {
		t.Errorf("expected 3 pending, got %d", len(pending))
	}
	done, _ := box.List(ctx, core.OutboxFilter{Status: core.OutboxStatusDone})
	if len(done) != 0 {
		t.Errorf("expected 0 done, got %d", len(done))
	}
}
