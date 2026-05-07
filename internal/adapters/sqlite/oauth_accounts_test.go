package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/core"
)

// newRepoWithOAuth создаёт PostRepository, UserRepository, OAuthAccountRepository
// поверх одного DB.
func newRepoWithOAuth(t *testing.T) (*PostRepository, *UserRepository, *OAuthAccountRepository) {
	t.Helper()
	r := newRepo(t)
	return r, r.Users(), r.OAuthAccounts()
}

func makeOAuthAccount(userID uuid.UUID, provider, externalID, email string) *core.OAuthAccount {
	now := time.Now().UTC().Truncate(time.Second)
	return &core.OAuthAccount{
		ID:         uuid.New(),
		UserID:     userID,
		Provider:   provider,
		ExternalID: externalID,
		Email:      email,
		CreatedAt:  now,
	}
}

func TestSQLiteOAuthRepo_CRUD(t *testing.T) {
	_, users, oauth := newRepoWithOAuth(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u := makeUser(tenantID, "alice@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}

	a := makeOAuthAccount(u.ID, "github", "12345", "alice@example.com")
	if err := oauth.Create(ctx, a); err != nil {
		t.Fatalf("Create oauth: %v", err)
	}

	got, err := oauth.GetByExternalID(ctx, "github", "12345")
	if err != nil {
		t.Fatalf("GetByExternalID: %v", err)
	}
	if got.ID != a.ID || got.UserID != u.ID || got.Provider != "github" || got.ExternalID != "12345" || got.Email != a.Email {
		t.Errorf("got %+v, want match %+v", got, a)
	}
	if !got.CreatedAt.Equal(a.CreatedAt) {
		t.Errorf("CreatedAt mismatch: got %v want %v", got.CreatedAt, a.CreatedAt)
	}

	list, err := oauth.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 1 || list[0].ID != a.ID {
		t.Errorf("ListByUser returned %d items, want 1; first=%v", len(list), list)
	}

	if err := oauth.Delete(ctx, a.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := oauth.GetByExternalID(ctx, "github", "12345"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
	if err := oauth.Delete(ctx, uuid.New()); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Delete missing err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteOAuthRepo_GetByExternalID_NotFound(t *testing.T) {
	_, _, oauth := newRepoWithOAuth(t)
	if _, err := oauth.GetByExternalID(context.Background(), "github", "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteOAuthRepo_DuplicateExternalID(t *testing.T) {
	_, users, oauth := newRepoWithOAuth(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u := makeUser(tenantID, "dup@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	a := makeOAuthAccount(u.ID, "github", "duplicate-id", "dup@example.com")
	if err := oauth.Create(ctx, a); err != nil {
		t.Fatalf("Create #1: %v", err)
	}
	// Same provider+external_id, different ID — should be ErrAlreadyExists.
	a2 := makeOAuthAccount(u.ID, "github", "duplicate-id", "dup@example.com")
	if err := oauth.Create(ctx, a2); !errors.Is(err, core.ErrAlreadyExists) {
		t.Errorf("Create dup err = %v, want ErrAlreadyExists", err)
	}
}

func TestSQLiteOAuthRepo_CascadeDelete(t *testing.T) {
	_, users, oauth := newRepoWithOAuth(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u := makeUser(tenantID, "cascade-oauth@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	a := makeOAuthAccount(u.ID, "github", "cascade-1", u.Email)
	if err := oauth.Create(ctx, a); err != nil {
		t.Fatalf("Create oauth: %v", err)
	}

	if err := users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete user: %v", err)
	}
	list, err := oauth.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser after cascade: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("ListByUser after cascade len=%d, want 0", len(list))
	}
	if _, err := oauth.GetByExternalID(ctx, "github", "cascade-1"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("after cascade err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteOAuthRepo_ListByUser_OrderAndMulti(t *testing.T) {
	_, users, oauth := newRepoWithOAuth(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u := makeUser(tenantID, "multi-oauth@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create user: %v", err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	a1 := makeOAuthAccount(u.ID, "github", "ext1", u.Email)
	a1.CreatedAt = now
	a2 := makeOAuthAccount(u.ID, "gitlab", "ext2", u.Email)
	a2.CreatedAt = now.Add(time.Second)
	for _, a := range []*core.OAuthAccount{a1, a2} {
		if err := oauth.Create(ctx, a); err != nil {
			t.Fatalf("Create %s: %v", a.Provider, err)
		}
	}
	list, err := oauth.ListByUser(ctx, u.ID)
	if err != nil {
		t.Fatalf("ListByUser: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("len=%d, want 2", len(list))
	}
	if list[0].ID != a1.ID || list[1].ID != a2.ID {
		t.Errorf("order mismatch: got [%s,%s], want [%s,%s]",
			list[0].Provider, list[1].Provider, a1.Provider, a2.Provider)
	}
}
