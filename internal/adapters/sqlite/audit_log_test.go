package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

func newTestAuditRepo(t *testing.T) *AuditLogRepository {
	t.Helper()
	repo, err := NewSQLitePostRepository(Config{DSN: ":memory:"})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo.AuditLog()
}

func TestAuditLog_AppendAndList(t *testing.T) {
	repo := newTestAuditRepo(t)
	ctx := context.Background()

	tenant := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	actor := uuid.MustParse("22222222-2222-2222-2222-222222222222")

	for i, action := range []core.AuditAction{
		core.AuditAuthLoginSuccess,
		core.AuditAuthLoginFail,
		core.AuditAuthLogout,
	} {
		entry := &core.AuditEntry{
			OccurredAt: time.Now().UTC().Add(time.Duration(i) * time.Millisecond),
			TenantID:   tenant,
			ActorID:    actor,
			ActorType:  core.AuditActorUser,
			Action:     action,
			Outcome:    core.AuditOutcomeSuccess,
			IP:         "203.0.113.1",
			UserAgent:  "test/1.0",
			Metadata:   map[string]any{"i": i},
		}
		if err := repo.Append(ctx, entry); err != nil {
			t.Fatalf("Append %d: %v", i, err)
		}
	}

	all, err := repo.List(ctx, core.AuditFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(all))
	}
	if all[0].OccurredAt.Before(all[2].OccurredAt) {
		t.Fatalf("expected DESC order, got first=%v last=%v", all[0].OccurredAt, all[2].OccurredAt)
	}

	filtered, err := repo.List(ctx, core.AuditFilter{Action: core.AuditAuthLoginFail})
	if err != nil {
		t.Fatalf("List filter: %v", err)
	}
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered entry, got %d", len(filtered))
	}
	if filtered[0].Action != core.AuditAuthLoginFail {
		t.Fatalf("expected action=%s, got %s", core.AuditAuthLoginFail, filtered[0].Action)
	}
	if v, ok := filtered[0].Metadata["i"]; !ok || v.(float64) != 1 {
		t.Fatalf("metadata roundtrip failed: %#v", filtered[0].Metadata)
	}
}

func TestAuditLog_AnonymousActor(t *testing.T) {
	repo := newTestAuditRepo(t)
	ctx := context.Background()
	if err := repo.Append(ctx, &core.AuditEntry{
		Action:    core.AuditAuthLoginFail,
		ActorType: core.AuditActorAnonymous,
		Outcome:   core.AuditOutcomeFailure,
	}); err != nil {
		t.Fatalf("Append anon: %v", err)
	}
	got, err := repo.List(ctx, core.AuditFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 entry")
	}
	if got[0].ActorID != uuid.Nil {
		t.Fatalf("expected nil actor, got %s", got[0].ActorID)
	}
}

func TestAuditService_FillsDefaults(t *testing.T) {
	repo := newTestAuditRepo(t)
	svc := core.NewAuditService(repo, core.SystemClock{})
	user := &core.User{ID: uuid.New(), TenantID: uuid.New()}
	ctx := core.WithUser(context.Background(), user)
	ctx = core.WithAuditContext(ctx, core.AuditContext{IP: "10.0.0.1", UserAgent: "ua"})

	if err := svc.Log(ctx, core.AuditEntry{Action: core.AuditAuthLoginSuccess}); err != nil {
		t.Fatalf("Log: %v", err)
	}
	got, err := repo.List(context.Background(), core.AuditFilter{})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1, got %d", len(got))
	}
	e := got[0]
	if e.ActorID != user.ID {
		t.Fatalf("expected ActorID=%s from ctx, got %s", user.ID, e.ActorID)
	}
	if e.TenantID != user.TenantID {
		t.Fatalf("expected TenantID from ctx, got %s", e.TenantID)
	}
	if e.IP != "10.0.0.1" || e.UserAgent != "ua" {
		t.Fatalf("expected IP/UA from ctx, got ip=%s ua=%s", e.IP, e.UserAgent)
	}
	if e.Outcome != core.AuditOutcomeSuccess {
		t.Fatalf("expected default Outcome=success, got %s", e.Outcome)
	}
	if e.ActorType != core.AuditActorUser {
		t.Fatalf("expected ActorType=user inferred, got %s", e.ActorType)
	}
}
