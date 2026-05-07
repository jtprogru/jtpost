package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/core"
)

// newRepoWithSessions создаёт UserRepository, SessionRepository поверх одного DB.
func newRepoWithSessions(t *testing.T) (*UserRepository, *SessionRepository) {
	t.Helper()
	r := newRepo(t)
	return r.Users(), r.Sessions()
}

func makeSession(userID uuid.UUID, prefix string) *core.Session {
	now := time.Now().UTC().Truncate(time.Second)
	return &core.Session{
		ID:         uuid.New(),
		UserID:     userID,
		Prefix:     prefix,
		SecretHash: "secret-hash-stub",
		CSRFToken:  "csrf-stub",
		CreatedAt:  now,
		ExpiresAt:  now.Add(24 * time.Hour),
	}
}

func TestSQLiteSessionRepo_CRUD(t *testing.T) {
	users, sessions := newRepoWithSessions(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u := makeUser(tenantID, "alice@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	s := makeSession(u.ID, "sess0001")
	if err := sessions.Create(ctx, s); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	got, err := sessions.GetByPrefix(ctx, s.Prefix)
	if err != nil {
		t.Fatalf("GetByPrefix: %v", err)
	}
	if got.ID != s.ID || got.UserID != u.ID || got.CSRFToken != s.CSRFToken {
		t.Errorf("got %+v, want match %+v", got, s)
	}
	if !got.ExpiresAt.Equal(s.ExpiresAt) {
		t.Errorf("ExpiresAt mismatch: got %v want %v", got.ExpiresAt, s.ExpiresAt)
	}

	if err := sessions.Delete(ctx, s.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := sessions.GetByPrefix(ctx, s.Prefix); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
	if err := sessions.Delete(ctx, uuid.New()); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Delete missing err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteSessionRepo_GetByPrefix_NotFound(t *testing.T) {
	_, sessions := newRepoWithSessions(t)
	if _, err := sessions.GetByPrefix(context.Background(), "missing0"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteSessionRepo_DeleteByUser(t *testing.T) {
	users, sessions := newRepoWithSessions(t)
	ctx := context.Background()
	tenantID := uuid.New()
	u := makeUser(tenantID, "multi@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	prefixes := []string{"prefa001", "prefa002", "prefa003"}
	for _, p := range prefixes {
		if err := sessions.Create(ctx, makeSession(u.ID, p)); err != nil {
			t.Fatalf("Create session %s: %v", p, err)
		}
	}
	if err := sessions.DeleteByUser(ctx, u.ID); err != nil {
		t.Fatalf("DeleteByUser: %v", err)
	}
	for _, p := range prefixes {
		if _, err := sessions.GetByPrefix(ctx, p); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("session %s still exists: err=%v", p, err)
		}
	}
	// no-op для пустого user — без ошибки
	if err := sessions.DeleteByUser(ctx, uuid.New()); err != nil {
		t.Errorf("DeleteByUser empty: %v", err)
	}
}

func TestSQLiteSessionRepo_CascadeDelete(t *testing.T) {
	users, sessions := newRepoWithSessions(t)
	ctx := context.Background()
	tenantID := uuid.New()
	u := makeUser(tenantID, "cascade@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	s := makeSession(u.ID, "casc0001")
	if err := sessions.Create(ctx, s); err != nil {
		t.Fatalf("Create session: %v", err)
	}

	if err := users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete user: %v", err)
	}
	if _, err := sessions.GetByPrefix(ctx, s.Prefix); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("after cascade err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteSessionRepo_UpdateLastUsedAt(t *testing.T) {
	users, sessions := newRepoWithSessions(t)
	ctx := context.Background()
	tenantID := uuid.New()
	u := makeUser(tenantID, "lu@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	s := makeSession(u.ID, "lupref01")
	if err := sessions.Create(ctx, s); err != nil {
		t.Fatalf("Create session: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	if err := sessions.UpdateLastUsedAt(ctx, s.ID, now); err != nil {
		t.Fatalf("UpdateLastUsedAt: %v", err)
	}
	got, err := sessions.GetByPrefix(ctx, s.Prefix)
	if err != nil {
		t.Fatalf("GetByPrefix: %v", err)
	}
	if got.LastUsedAt == nil || !got.LastUsedAt.Equal(now) {
		t.Errorf("LastUsedAt = %v, want %v", got.LastUsedAt, now)
	}
	if err := sessions.UpdateLastUsedAt(ctx, uuid.New(), now); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing UpdateLastUsedAt err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteSessionRepo_UpdateCSRFToken(t *testing.T) {
	users, sessions := newRepoWithSessions(t)
	ctx := context.Background()
	tenantID := uuid.New()
	u := makeUser(tenantID, "csrf@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	s := makeSession(u.ID, "csrfp001")
	if err := sessions.Create(ctx, s); err != nil {
		t.Fatalf("Create session: %v", err)
	}
	if err := sessions.UpdateCSRFToken(ctx, s.ID, "new-csrf-value"); err != nil {
		t.Fatalf("UpdateCSRFToken: %v", err)
	}
	got, err := sessions.GetByPrefix(ctx, s.Prefix)
	if err != nil {
		t.Fatalf("GetByPrefix: %v", err)
	}
	if got.CSRFToken != "new-csrf-value" {
		t.Errorf("CSRFToken = %q, want %q", got.CSRFToken, "new-csrf-value")
	}
	if err := sessions.UpdateCSRFToken(ctx, uuid.New(), "x"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing UpdateCSRFToken err = %v, want ErrNotFound", err)
	}
}
