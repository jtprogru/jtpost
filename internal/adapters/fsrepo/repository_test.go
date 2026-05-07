package fsrepo

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

func tenantCtx(tenantID uuid.UUID) context.Context {
	return core.WithTenant(context.Background(), tenantID)
}

func TestFSRepo_Create_TenantSubdir(t *testing.T) {
	dir := t.TempDir()
	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("NewFileSystemRepository() error = %v", err)
	}

	post := newTestPost(t, func(p *core.Post) {
		p.ID = mustParsePostID("subdir-1")
		p.Slug = "subdir-post"
	})

	if err := repo.Create(tenantCtx(testTenant1), post); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	expected := filepath.Join(dir, post.TenantShortID(), "subdir-post.md")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected file at %s, got error: %v", expected, err)
	}
}

func TestFSRepo_GetByID_OtherTenant_NotFound(t *testing.T) {
	// Property: CP-1
	dir := t.TempDir()
	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("NewFileSystemRepository() error = %v", err)
	}

	post := newTestPost(t, func(p *core.Post) {
		p.ID = mustParsePostID("other-tenant-id")
		p.Slug = "other-tenant-post"
		p.TenantID = testTenant1
	})
	if err := repo.Create(tenantCtx(testTenant1), post); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	_, err = repo.GetByID(tenantCtx(testTenant2), post.ID)
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestFSRepo_GetByID_NoContext_TenantMismatch(t *testing.T) {
	dir := t.TempDir()
	repo, err := NewFileSystemRepository(dir)
	if err != nil {
		t.Fatalf("NewFileSystemRepository() error = %v", err)
	}

	_, err = repo.GetByID(context.Background(), mustParsePostID("any"))
	if !errors.Is(err, core.ErrTenantMismatch) {
		t.Errorf("GetByID() without tenant error = %v, want ErrTenantMismatch", err)
	}
}

func TestFSRepo_List_TenantScoped(t *testing.T) {
	// Property: CP-1
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	mk := func(id, slug string, tenant uuid.UUID) *core.Post {
		return newTestPost(t, func(p *core.Post) {
			p.ID = mustParsePostID(id)
			p.Slug = slug
			p.TenantID = tenant
		})
	}

	if err := repo.Create(tenantCtx(testTenant1), mk("a", "post-a", testTenant1)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(tenantCtx(testTenant1), mk("b", "post-b", testTenant1)); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(tenantCtx(testTenant2), mk("c", "post-c", testTenant2)); err != nil {
		t.Fatal(err)
	}

	got, err := repo.List(context.Background(), core.PostFilter{TenantID: testTenant1})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("List() length = %d, want 2", len(got))
	}
}

func TestFSRepo_List_RejectsZeroTenant(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	_, err := repo.List(context.Background(), core.PostFilter{})
	if !errors.Is(err, core.ErrValidation) {
		t.Errorf("List(zero tenant) error = %v, want ErrValidation", err)
	}
}

func TestFSRepo_List_EmptyTenantDir(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	got, err := repo.List(context.Background(), core.PostFilter{TenantID: testTenant1})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("List() length = %d, want 0", len(got))
	}
}

func TestFSRepo_List_AuthorFilter(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	mk := func(id, slug string, author uuid.UUID) *core.Post {
		return newTestPost(t, func(p *core.Post) {
			p.ID = mustParsePostID(id)
			p.Slug = slug
			p.AuthorID = author
		})
	}

	for _, p := range []*core.Post{
		mk("af-1", "af-1", testAuthor1),
		mk("af-2", "af-2", testAuthor1),
		mk("af-3", "af-3", testAuthor2),
	} {
		if err := repo.Create(tenantCtx(testTenant1), p); err != nil {
			t.Fatal(err)
		}
	}

	authorRef := testAuthor1
	got, err := repo.List(context.Background(), core.PostFilter{TenantID: testTenant1, AuthorID: &authorRef})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("List() length = %d, want 2", len(got))
	}
}

func TestFSRepo_List_Sort(t *testing.T) {
	// Property: CP-11
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	posts := []*core.Post{
		newTestPost(t, func(p *core.Post) {
			p.ID = mustParsePostID("s-c")
			p.Slug = "s-c"
			p.Title = "C"
			p.CreatedAt = base.Add(2 * time.Hour)
			p.UpdatedAt = base.Add(2 * time.Hour)
			p.Status = core.StatusReady
		}),
		newTestPost(t, func(p *core.Post) {
			p.ID = mustParsePostID("s-a")
			p.Slug = "s-a"
			p.Title = "A"
			p.CreatedAt = base.Add(0 * time.Hour)
			p.UpdatedAt = base.Add(0 * time.Hour)
			p.Status = core.StatusDraft
		}),
		newTestPost(t, func(p *core.Post) {
			p.ID = mustParsePostID("s-b")
			p.Slug = "s-b"
			p.Title = "B"
			p.CreatedAt = base.Add(1 * time.Hour)
			p.UpdatedAt = base.Add(1 * time.Hour)
			p.Status = core.StatusIdea
		}),
	}
	for _, p := range posts {
		if err := repo.Create(tenantCtx(testTenant1), p); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name      string
		sortBy    string
		sortOrder string
		wantFirst string
	}{
		{"created_at asc", "created_at", "asc", "A"},
		{"created_at desc", "created_at", "desc", "C"},
		{"updated_at asc", "updated_at", "asc", "A"},
		{"updated_at desc", "updated_at", "desc", "C"},
		{"title asc", "title", "asc", "A"},
		{"title desc", "title", "desc", "C"},
		{"status asc", "status", "asc", "draft"},
		{"status desc", "status", "desc", "ready"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := repo.List(context.Background(), core.PostFilter{
				TenantID: testTenant1, SortBy: tt.sortBy, SortOrder: tt.sortOrder,
			})
			if err != nil {
				t.Fatalf("List() error = %v", err)
			}
			if len(got) == 0 {
				t.Fatalf("List() empty")
			}
			switch tt.sortBy {
			case "status":
				if string(got[0].Status) != tt.wantFirst {
					t.Errorf("first status = %s, want %s", got[0].Status, tt.wantFirst)
				}
			default:
				if got[0].Title != tt.wantFirst {
					t.Errorf("first title = %s, want %s", got[0].Title, tt.wantFirst)
				}
			}
		})
	}

	t.Run("invalid sort key", func(t *testing.T) {
		_, err := repo.List(context.Background(), core.PostFilter{TenantID: testTenant1, SortBy: "garbage"})
		if !errors.Is(err, core.ErrValidation) {
			t.Errorf("List(invalid sort) error = %v, want ErrValidation", err)
		}
	})
}

func TestFSRepo_List_LimitOffset(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 5 {
		idx := i
		p := newTestPost(t, func(pp *core.Post) {
			pp.ID = mustParsePostID(filepath.Join("lo", string(rune('a'+idx))))
			pp.Slug = "lo-" + string(rune('a'+idx))
			pp.Title = "T" + string(rune('A'+idx))
			pp.CreatedAt = base.Add(time.Duration(idx) * time.Hour)
			pp.UpdatedAt = pp.CreatedAt
		})
		if err := repo.Create(tenantCtx(testTenant1), p); err != nil {
			t.Fatal(err)
		}
	}

	got, err := repo.List(context.Background(), core.PostFilter{
		TenantID: testTenant1, SortBy: "created_at", SortOrder: "asc", Limit: 2, Offset: 1,
	})
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("List() length = %d, want 2", len(got))
	}
	if got[0].Title != "TB" || got[1].Title != "TC" {
		t.Errorf("got titles = [%s, %s], want [TB, TC]", got[0].Title, got[1].Title)
	}
}

func TestFSRepo_Delete_RemovesFile(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	post := newTestPost(t, func(p *core.Post) {
		p.ID = mustParsePostID("del-1")
		p.Slug = "del-1"
	})
	if err := repo.Create(tenantCtx(testTenant1), post); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(tenantCtx(testTenant1), post.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	expected := filepath.Join(dir, post.TenantShortID(), "del-1.md")
	if _, err := os.Stat(expected); !os.IsNotExist(err) {
		t.Errorf("file still exists after Delete: %v", err)
	}

	if err := repo.Delete(tenantCtx(testTenant1), post.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Delete() second error = %v, want ErrNotFound", err)
	}
}

func TestFSRepo_Update_AfterCreate(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	post := newTestPost(t, func(p *core.Post) {
		p.ID = mustParsePostID("upd-1")
		p.Slug = "upd-1"
		p.Title = "Original"
	})
	if err := repo.Create(tenantCtx(testTenant1), post); err != nil {
		t.Fatal(err)
	}

	post.Title = "Updated"
	post.Revision = 2
	post.UpdatedAt = post.UpdatedAt.Add(time.Hour)
	if err := repo.Update(tenantCtx(testTenant1), post); err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, err := repo.GetByID(tenantCtx(testTenant1), post.ID)
	if err != nil {
		t.Fatalf("GetByID() error = %v", err)
	}
	if got.Title != "Updated" {
		t.Errorf("Title = %s, want Updated", got.Title)
	}
	if got.Revision != 2 {
		t.Errorf("Revision = %d, want 2", got.Revision)
	}
}

func TestFSRepo_Create_AlreadyExists(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	post := newTestPost(t, func(p *core.Post) {
		p.ID = mustParsePostID("dup")
		p.Slug = "dup"
	})
	if err := repo.Create(tenantCtx(testTenant1), post); err != nil {
		t.Fatal(err)
	}
	if err := repo.Create(tenantCtx(testTenant1), post); !errors.Is(err, core.ErrAlreadyExists) {
		t.Errorf("Create() second error = %v, want ErrAlreadyExists", err)
	}
}

func TestFSRepo_GetByID_NotFound(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	_, err := repo.GetByID(tenantCtx(testTenant1), mustParsePostID("nope"))
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetByID() error = %v, want ErrNotFound", err)
	}
}

func TestFSRepo_GetBySlug(t *testing.T) {
	dir := t.TempDir()
	repo, _ := NewFileSystemRepository(dir)

	post := newTestPost(t, func(p *core.Post) {
		p.ID = mustParsePostID("bs")
		p.Slug = "by-slug"
	})
	if err := repo.Create(tenantCtx(testTenant1), post); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetBySlug(tenantCtx(testTenant1), "by-slug")
	if err != nil {
		t.Fatalf("GetBySlug() error = %v", err)
	}
	if got.Title != post.Title {
		t.Errorf("Title = %s, want %s", got.Title, post.Title)
	}

	if _, err := repo.GetBySlug(tenantCtx(testTenant1), "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetBySlug(missing) error = %v, want ErrNotFound", err)
	}

	if _, err := repo.GetBySlug(context.Background(), "by-slug"); !errors.Is(err, core.ErrTenantMismatch) {
		t.Errorf("GetBySlug(no tenant) error = %v, want ErrTenantMismatch", err)
	}
}

func TestNewFileSystemRepository_CreatesDirectory(t *testing.T) {
	tempDir := t.TempDir()
	newDir := filepath.Join(tempDir, "new-posts")

	repo, err := NewFileSystemRepository(newDir)
	if err != nil {
		t.Fatalf("NewFileSystemRepository() error = %v", err)
	}
	if repo == nil {
		t.Fatal("NewFileSystemRepository() returned nil")
	}
	if _, err := os.Stat(newDir); os.IsNotExist(err) {
		t.Errorf("Directory was not created")
	}
}
