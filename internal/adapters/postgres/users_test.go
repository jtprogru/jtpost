//go:build integration

package postgres

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/core"
)

func newUserRepo(t *testing.T) (*UserRepository, *TokenRepository) {
	t.Helper()
	r := newRepo(t)
	return r.Users(), r.Tokens()
}

func makeUser(tenantID uuid.UUID, email string, role core.Role) *core.User {
	now := time.Now().UTC().Truncate(time.Microsecond)
	return &core.User{
		ID:           uuid.New(),
		TenantID:     tenantID,
		Email:        email,
		PasswordHash: "hash-stub",
		Role:         role,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func TestPostgresUserRepo_CRUD(t *testing.T) {
	users, _ := newUserRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u := makeUser(tenantID, "alice@example.com", core.RoleAuthor)
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := users.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != u.Email || got.Role != u.Role || got.TenantID != tenantID {
		t.Errorf("got %+v, want match %+v", got, u)
	}

	u.Email = "alice2@example.com"
	u.Role = core.RoleEditor
	u.UpdatedAt = time.Now().UTC().Truncate(time.Microsecond)
	if err := users.Update(ctx, u); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, err := users.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got2.Email != "alice2@example.com" || got2.Role != core.RoleEditor {
		t.Errorf("update mismatch: %+v", got2)
	}

	list, err := users.List(ctx, tenantID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("List len = %d, want 1", len(list))
	}

	if err := users.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := users.GetByID(ctx, u.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
}

func TestPostgresUserRepo_GetByEmail_TenantScope(t *testing.T) {
	users, _ := newUserRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()
	uA := makeUser(tenantA, "same@example.com", core.RoleAuthor)
	uB := makeUser(tenantB, "same@example.com", core.RoleAuthor)
	if err := users.Create(ctx, uA); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if err := users.Create(ctx, uB); err != nil {
		t.Fatalf("Create B: %v", err)
	}
	gotA, err := users.GetByEmail(ctx, tenantA, "same@example.com")
	if err != nil || gotA.ID != uA.ID {
		t.Fatalf("GetByEmail A: %v", err)
	}
	if _, err := users.GetByEmail(ctx, tenantA, "missing@x.com"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("missing email err = %v, want ErrNotFound", err)
	}
}

func TestPostgresUserRepo_EmailCollision(t *testing.T) {
	users, _ := newUserRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()
	u1 := makeUser(tenantID, "dup@example.com", core.RoleAuthor)
	u2 := makeUser(tenantID, "dup@example.com", core.RoleEditor)
	if err := users.Create(ctx, u1); err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	err := users.Create(ctx, u2)
	if !errors.Is(err, core.ErrAlreadyExists) {
		t.Errorf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestPostgresUserRepo_CountOwners(t *testing.T) {
	users, _ := newUserRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()
	for i, role := range []core.Role{core.RoleOwner, core.RoleOwner, core.RoleAuthor} {
		u := makeUser(tenantID, fmt.Sprintf("u%d@example.com", i), role)
		if err := users.Create(ctx, u); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}
	owners, err := users.CountOwners(ctx, tenantID)
	if err != nil {
		t.Fatalf("CountOwners: %v", err)
	}
	if owners != 2 {
		t.Errorf("CountOwners = %d, want 2", owners)
	}
	total, _ := users.Count(ctx, tenantID)
	if total != 3 {
		t.Errorf("Count = %d, want 3", total)
	}
}

func TestPostgresUserRepo_NotFound(t *testing.T) {
	users, _ := newUserRepo(t)
	ctx := context.Background()
	if _, err := users.GetByID(ctx, uuid.New()); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetByID missing err = %v, want ErrNotFound", err)
	}
	if err := users.Delete(ctx, uuid.New()); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Delete missing err = %v, want ErrNotFound", err)
	}
}
