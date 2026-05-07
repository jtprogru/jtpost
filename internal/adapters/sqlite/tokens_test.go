package sqlite

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/core"
)

// newRepoWithUsers создаёт PostRepository, UserRepository, TokenRepository поверх одного DB.
func newRepoWithUsers(t *testing.T) (*PostRepository, *UserRepository, *TokenRepository) {
	t.Helper()
	r := newRepo(t)
	return r, r.Users(), r.Tokens()
}

func makeToken(userID uuid.UUID, prefix, name string) *core.APIToken {
	now := time.Now().UTC().Truncate(time.Second)
	return &core.APIToken{
		ID:         uuid.New(),
		UserID:     userID,
		Prefix:     prefix,
		SecretHash: "hash-stub",
		Name:       name,
		CreatedAt:  now,
	}
}

func TestSQLiteTokenRepo_CRUD(t *testing.T) {
	_, users, tokens := newRepoWithUsers(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u := makeUser(tenantID, "owner@example.com", core.RoleOwner)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	tk := makeToken(u.ID, "prefix01", "first")
	if err := tokens.Create(ctx, tk); err != nil {
		t.Fatalf("Create token: %v", err)
	}

	got, err := tokens.GetByPrefix(ctx, tk.Prefix)
	if err != nil {
		t.Fatalf("GetByPrefix: %v", err)
	}
	if got.ID != tk.ID || got.Name != "first" {
		t.Errorf("got %+v, want match %+v", got, tk)
	}

	list, err := tokens.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("ListByUser len = %d, want 1", len(list))
	}

	if err := tokens.Delete(ctx, tk.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := tokens.GetByPrefix(ctx, tk.Prefix); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteTokenRepo_GetByPrefix_NotFound(t *testing.T) {
	_, _, tokens := newRepoWithUsers(t)
	if _, err := tokens.GetByPrefix(context.Background(), "missing0"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteTokenRepo_CascadeDelete(t *testing.T) {
	_, users, tokens := newRepoWithUsers(t)
	ctx := context.Background()
	tenantID := uuid.New()
	u := makeUser(tenantID, "cascade@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	for i, prefix := range []string{"casc0001", "casc0002"} {
		tk := makeToken(u.ID, prefix, fmt.Sprintf("t%d", i))
		if err := tokens.Create(ctx, tk); err != nil {
			t.Fatalf("Create token %d: %v", i, err)
		}
	}
	list, _ := tokens.ListByUser(ctx, u.ID)
	if len(list) != 2 {
		t.Fatalf("expected 2 tokens before delete, got %d", len(list))
	}

	if err := users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete user: %v", err)
	}

	list, err := tokens.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser after cascade: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("after cascade len = %d, want 0", len(list))
	}
}

func TestSQLiteTokenRepo_UpdateLastUsedAt(t *testing.T) {
	_, users, tokens := newRepoWithUsers(t)
	ctx := context.Background()
	tenantID := uuid.New()
	u := makeUser(tenantID, "lu@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	tk := makeToken(u.ID, "lupref01", "lu")
	if err := tokens.Create(ctx, tk); err != nil {
		t.Fatalf("Create token: %v", err)
	}

	now := time.Now().UTC().Truncate(time.Second)
	if err := tokens.UpdateLastUsedAt(ctx, tk.ID, now); err != nil {
		t.Fatalf("UpdateLastUsedAt: %v", err)
	}
	got, err := tokens.GetByPrefix(ctx, tk.Prefix)
	if err != nil {
		t.Fatalf("GetByPrefix: %v", err)
	}
	if got.LastUsedAt == nil || !got.LastUsedAt.Equal(now) {
		t.Errorf("LastUsedAt = %v, want %v", got.LastUsedAt, now)
	}

	if err := tokens.UpdateLastUsedAt(ctx, uuid.New(), now); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing token UpdateLastUsedAt err = %v, want ErrNotFound", err)
	}
}

// fmt referenced via fmt.Sprintf above.
var _ = errors.New
