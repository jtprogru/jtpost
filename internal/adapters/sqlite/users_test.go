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

func newUserRepo(t *testing.T) *UserRepository {
	t.Helper()
	return newRepo(t).Users()
}

func makeUser(tenantID uuid.UUID, email string, role core.Role) *core.User {
	now := time.Now().UTC().Truncate(time.Second)
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

func TestSQLiteUserRepo_CRUD(t *testing.T) {
	repo := newUserRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u := makeUser(tenantID, "alice@example.com", core.RoleAuthor)
	if err := repo.Create(ctx, u); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Email != u.Email || got.Role != u.Role || got.TenantID != tenantID {
		t.Errorf("got %+v, want match %+v", got, u)
	}

	// Update
	u.Email = "alice2@example.com"
	u.Role = core.RoleEditor
	u.UpdatedAt = time.Now().UTC().Truncate(time.Second)
	if err := repo.Update(ctx, u); err != nil {
		t.Fatalf("Update: %v", err)
	}
	got2, err := repo.GetByID(ctx, u.ID)
	if err != nil {
		t.Fatalf("GetByID after update: %v", err)
	}
	if got2.Email != "alice2@example.com" || got2.Role != core.RoleEditor {
		t.Errorf("update mismatch: %+v", got2)
	}

	// List
	users, err := repo.List(ctx, tenantID)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) != 1 {
		t.Errorf("List len = %d, want 1", len(users))
	}

	// Count
	cnt, err := repo.Count(ctx, tenantID)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if cnt != 1 {
		t.Errorf("Count = %d, want 1", cnt)
	}

	// Delete
	if err := repo.Delete(ctx, u.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, u.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteUserRepo_GetByEmail_TenantScope(t *testing.T) {
	repo := newUserRepo(t)
	ctx := context.Background()
	tenantA := uuid.New()
	tenantB := uuid.New()

	uA := makeUser(tenantA, "same@example.com", core.RoleAuthor)
	uB := makeUser(tenantB, "same@example.com", core.RoleAuthor)
	if err := repo.Create(ctx, uA); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if err := repo.Create(ctx, uB); err != nil {
		t.Fatalf("Create B: %v", err)
	}

	gotA, err := repo.GetByEmail(ctx, tenantA, "same@example.com")
	if err != nil {
		t.Fatalf("GetByEmail A: %v", err)
	}
	if gotA.ID != uA.ID {
		t.Errorf("GetByEmail A returned wrong user")
	}
	gotB, err := repo.GetByEmail(ctx, tenantB, "same@example.com")
	if err != nil {
		t.Fatalf("GetByEmail B: %v", err)
	}
	if gotB.ID != uB.ID {
		t.Errorf("GetByEmail B returned wrong user")
	}

	// Cross-tenant lookup with non-existing email
	if _, err := repo.GetByEmail(ctx, tenantA, "nope@example.com"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSQLiteUserRepo_EmailCollision(t *testing.T) {
	repo := newUserRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()

	u1 := makeUser(tenantID, "dup@example.com", core.RoleAuthor)
	u2 := makeUser(tenantID, "dup@example.com", core.RoleEditor)

	if err := repo.Create(ctx, u1); err != nil {
		t.Fatalf("Create 1: %v", err)
	}
	err := repo.Create(ctx, u2)
	if !errors.Is(err, core.ErrAlreadyExists) {
		t.Errorf("err = %v, want ErrAlreadyExists", err)
	}
}

func TestSQLiteUserRepo_CountOwners(t *testing.T) {
	repo := newUserRepo(t)
	ctx := context.Background()
	tenantID := uuid.New()

	for i, role := range []core.Role{core.RoleOwner, core.RoleOwner, core.RoleAuthor} {
		u := makeUser(tenantID, fmt.Sprintf("u%d@example.com", i), role)
		if err := repo.Create(ctx, u); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	owners, err := repo.CountOwners(ctx, tenantID)
	if err != nil {
		t.Fatalf("CountOwners: %v", err)
	}
	if owners != 2 {
		t.Errorf("CountOwners = %d, want 2", owners)
	}

	total, err := repo.Count(ctx, tenantID)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if total != 3 {
		t.Errorf("Count = %d, want 3", total)
	}
}

func TestSQLiteUserRepo_NotFound(t *testing.T) {
	repo := newUserRepo(t)
	ctx := context.Background()

	if _, err := repo.GetByID(ctx, uuid.New()); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetByID missing err = %v, want ErrNotFound", err)
	}
	if err := repo.Update(ctx, makeUser(uuid.New(), "ghost@x.com", core.RoleAuthor)); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Update missing err = %v, want ErrNotFound", err)
	}
	if err := repo.Delete(ctx, uuid.New()); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Delete missing err = %v, want ErrNotFound", err)
	}
}
