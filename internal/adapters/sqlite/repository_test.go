package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/adapters/repotest"
	"github.com/jtprogru/jtpost/internal/core"
)

// TestSQLite_RunContract — общий repotest.RunContract против sqlite-адаптера.
// SQLite поддерживает optimistic-lock и транзакции.
func TestSQLite_RunContract(t *testing.T) {
	repotest.RunContract(t, func(t *testing.T) (core.PostRepository, repotest.Capabilities, func()) {
		t.Helper()
		repo := newRepo(t)
		return repo, repotest.Capabilities{
			OptimisticLock: true,
			Transactions:   true,
		}, func() {}
	})
}

// newRepo создаёт SQLite репозиторий поверх временного файла.
func newRepo(t *testing.T) *PostRepository {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	repo, err := NewSQLitePostRepository(Config{DSN: dbPath})
	if err != nil {
		t.Fatalf("NewSQLitePostRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

// withTenant добавляет в контекст TenantID поста.
func withTenant(ctx context.Context, post *core.Post) context.Context {
	return core.WithTenant(ctx, post.TenantID)
}

// makePost создаёт пост-фикстуру с указанным tenant'ом.
func makePost(t *testing.T, tenantID uuid.UUID, slug string) *core.Post {
	t.Helper()
	authorID := uuid.New()
	now := time.Now().UTC().Truncate(time.Second)
	return &core.Post{
		ID:        core.PostID(uuid.New()),
		TenantID:  tenantID,
		AuthorID:  authorID,
		Title:     "Title " + slug,
		Slug:      slug,
		Status:    core.StatusDraft,
		Tags:      []string{"go", "test"},
		Content:   "content " + slug,
		Revision:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func TestSQLite_CreateGetRoundtrip(t *testing.T) {
	repo := newRepo(t)
	tenantID := uuid.New()
	deadline := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Second)
	scheduled := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Second)
	excerpt := "short excerpt"
	revSHA := "deadbeef"
	cover := &core.Attachment{
		ID:   uuid.New(),
		Type: core.AttachmentTypePhoto,
		Path: "cover.jpg",
	}

	post := makePost(t, tenantID, "round-trip")
	post.Deadline = &deadline
	post.ScheduledAt = &scheduled
	post.Excerpt = &excerpt
	post.RevisionSHA = &revSHA
	post.CoverImage = cover
	post.Attachments = []core.Attachment{
		{ID: uuid.New(), Type: core.AttachmentTypeDocument, Path: "doc.pdf"},
	}
	post.External.TelegramURL = "https://t.me/x/1"

	ctx := withTenant(context.Background(), post)
	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := repo.GetByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}

	if got.Title != post.Title || got.Slug != post.Slug {
		t.Errorf("title/slug mismatch: got %q/%q want %q/%q", got.Title, got.Slug, post.Title, post.Slug)
	}
	if got.TenantID != post.TenantID || got.AuthorID != post.AuthorID {
		t.Errorf("tenant/author mismatch")
	}
	if got.Excerpt == nil || *got.Excerpt != excerpt {
		t.Errorf("Excerpt mismatch: %v", got.Excerpt)
	}
	if got.CoverImage == nil || got.CoverImage.Path != cover.Path {
		t.Errorf("CoverImage mismatch: %+v", got.CoverImage)
	}
	if len(got.Attachments) != 1 {
		t.Errorf("attachments len = %d, want 1", len(got.Attachments))
	}
	if got.Deadline == nil || !got.Deadline.Equal(deadline) {
		t.Errorf("Deadline mismatch: %v vs %v", got.Deadline, deadline)
	}
	if got.External.TelegramURL != post.External.TelegramURL {
		t.Errorf("TelegramURL mismatch: %q", got.External.TelegramURL)
	}

	// GetBySlug
	gotSlug, err := repo.GetBySlug(ctx, post.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if gotSlug.ID != post.ID {
		t.Errorf("GetBySlug ID mismatch")
	}
}

func TestSQLite_GetByID_NoTenantInCtx(t *testing.T) {
	repo := newRepo(t)
	_, err := repo.GetByID(context.Background(), core.PostID(uuid.New()))
	if !errors.Is(err, core.ErrTenantMismatch) {
		t.Errorf("err = %v, want ErrTenantMismatch", err)
	}
}

func TestSQLite_GetByID_NotFound(t *testing.T) {
	repo := newRepo(t)
	ctx := core.WithTenant(context.Background(), uuid.New())
	_, err := repo.GetByID(ctx, core.PostID(uuid.New()))
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSQLite_TenantIsolation(t *testing.T) {
	repo := newRepo(t)
	tenantA := uuid.New()
	tenantB := uuid.New()

	postA := makePost(t, tenantA, "isolation-a")
	postB := makePost(t, tenantB, "isolation-b")

	ctxA := core.WithTenant(context.Background(), tenantA)
	ctxB := core.WithTenant(context.Background(), tenantB)

	if err := repo.Create(ctxA, postA); err != nil {
		t.Fatalf("Create A: %v", err)
	}
	if err := repo.Create(ctxB, postB); err != nil {
		t.Fatalf("Create B: %v", err)
	}

	// tenantB не видит postA по ID
	if _, err := repo.GetByID(ctxB, postA.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cross-tenant GetByID = %v, want ErrNotFound", err)
	}
	// tenantB не видит postA по slug
	if _, err := repo.GetBySlug(ctxB, postA.Slug); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("cross-tenant GetBySlug = %v, want ErrNotFound", err)
	}
	// Delete чужого поста — не падает, но ничего не удаляет
	if err := repo.Delete(ctxB, postA.ID); err != nil {
		t.Errorf("Delete cross-tenant: %v", err)
	}
	if _, err := repo.GetByID(ctxA, postA.ID); err != nil {
		t.Errorf("postA unexpectedly deleted: %v", err)
	}
}

func TestSQLite_List_FilterSortLimit(t *testing.T) {
	repo := newRepo(t)
	tenantID := uuid.New()
	ctx := core.WithTenant(context.Background(), tenantID)

	now := time.Now().UTC().Truncate(time.Second)
	posts := []*core.Post{
		{ID: core.PostID(uuid.New()), TenantID: tenantID, AuthorID: uuid.New(), Title: "Alpha", Slug: "alpha", Status: core.StatusDraft, Content: "c", Revision: 1, CreatedAt: now.Add(-3 * time.Hour), UpdatedAt: now},
		{ID: core.PostID(uuid.New()), TenantID: tenantID, AuthorID: uuid.New(), Title: "Beta", Slug: "beta", Status: core.StatusReady, Content: "c", Revision: 1, CreatedAt: now.Add(-2 * time.Hour), UpdatedAt: now, Tags: []string{"go"}},
		{ID: core.PostID(uuid.New()), TenantID: tenantID, AuthorID: uuid.New(), Title: "Gamma", Slug: "gamma", Status: core.StatusDraft, Content: "c", Revision: 1, CreatedAt: now.Add(-1 * time.Hour), UpdatedAt: now, Tags: []string{"go", "cli"}},
	}
	for _, p := range posts {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
	}

	tt := []struct {
		name      string
		filter    core.PostFilter
		wantCount int
	}{
		{"no filter", core.PostFilter{TenantID: tenantID}, 3},
		{"status draft", core.PostFilter{TenantID: tenantID, Statuses: []core.PostStatus{core.StatusDraft}}, 2},
		{"tag go", core.PostFilter{TenantID: tenantID, Tags: []string{"go"}}, 2},
		{"limit 2", core.PostFilter{TenantID: tenantID, Limit: 2}, 2},
		{"offset 1 limit 10", core.PostFilter{TenantID: tenantID, Limit: 10, Offset: 1}, 2},
		{"sort by title asc", core.PostFilter{TenantID: tenantID, SortBy: "title", SortOrder: "asc"}, 3},
		{"search Alpha", core.PostFilter{TenantID: tenantID, Search: "Alpha"}, 1},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			got, err := repo.List(ctx, tc.filter)
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if len(got) != tc.wantCount {
				t.Errorf("len = %d, want %d", len(got), tc.wantCount)
			}
		})
	}

	// Sort title asc: Alpha, Beta, Gamma
	t.Run("sort title asc order", func(t *testing.T) {
		got, err := repo.List(ctx, core.PostFilter{TenantID: tenantID, SortBy: "title", SortOrder: "asc"})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		want := []string{"Alpha", "Beta", "Gamma"}
		for i, p := range got {
			if p.Title != want[i] {
				t.Errorf("[%d] = %q, want %q", i, p.Title, want[i])
			}
		}
	})
}

func TestSQLite_List_NoTenant(t *testing.T) {
	repo := newRepo(t)
	_, err := repo.List(context.Background(), core.PostFilter{})
	if !errors.Is(err, core.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestSQLite_List_InvalidSort(t *testing.T) {
	repo := newRepo(t)
	_, err := repo.List(context.Background(), core.PostFilter{
		TenantID: uuid.New(),
		SortBy:   "invalid_key",
	})
	if !errors.Is(err, core.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

func TestSQLite_Update_Success(t *testing.T) {
	repo := newRepo(t)
	tenantID := uuid.New()
	post := makePost(t, tenantID, "upd-success")
	ctx := withTenant(context.Background(), post)

	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create: %v", err)
	}

	post.Title = "Updated"
	post.Revision = 2 // service incremented; adapter uses Revision-1 in WHERE
	if err := repo.Update(ctx, post); err != nil {
		t.Fatalf("Update: %v", err)
	}

	got, err := repo.GetByID(ctx, post.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Title != "Updated" || got.Revision != 2 {
		t.Errorf("got title=%q rev=%d", got.Title, got.Revision)
	}
}

func TestSQLite_Update_RevisionConflict(t *testing.T) {
	repo := newRepo(t)
	tenantID := uuid.New()
	post := makePost(t, tenantID, "upd-conflict")
	ctx := withTenant(context.Background(), post)

	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Stale revision: WHERE revision = post.Revision-1 = 0, no row matches.
	stale := *post
	stale.Title = "Stale"
	stale.Revision = 1 // service did NOT increment; WHERE revision=0 matches nothing
	err := repo.Update(ctx, &stale)
	if !errors.Is(err, core.ErrConflict) {
		t.Errorf("err = %v, want ErrConflict", err)
	}
}

func TestSQLite_Update_NotFound(t *testing.T) {
	repo := newRepo(t)
	tenantID := uuid.New()
	post := makePost(t, tenantID, "upd-nf")
	post.Revision = 2
	ctx := withTenant(context.Background(), post)

	err := repo.Update(ctx, post)
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSQLite_Update_NoTenantCtx(t *testing.T) {
	repo := newRepo(t)
	post := makePost(t, uuid.New(), "upd-no-tenant")
	if err := repo.Update(context.Background(), post); !errors.Is(err, core.ErrTenantMismatch) {
		t.Errorf("err = %v, want ErrTenantMismatch", err)
	}
}

func TestSQLite_Delete(t *testing.T) {
	repo := newRepo(t)
	tenantID := uuid.New()
	post := makePost(t, tenantID, "del")
	ctx := withTenant(context.Background(), post)

	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := repo.Delete(ctx, post.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, post.ID); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("after delete err = %v, want ErrNotFound", err)
	}
}

func TestSQLite_ImportPosts_Count(t *testing.T) {
	repo := newRepo(t)
	tenantID := uuid.New()
	posts := []*core.Post{
		makePost(t, tenantID, "imp-1"),
		makePost(t, tenantID, "imp-2"),
	}
	if err := repo.ImportPosts(context.Background(), posts); err != nil {
		t.Fatalf("ImportPosts: %v", err)
	}
	count, err := repo.Count(context.Background())
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count != 2 {
		t.Errorf("Count = %d, want 2", count)
	}

	// Re-import same posts (UPSERT semantics) — count stays at 2.
	if err := repo.ImportPosts(context.Background(), posts); err != nil {
		t.Fatalf("re-ImportPosts: %v", err)
	}
	count, _ = repo.Count(context.Background())
	if count != 2 {
		t.Errorf("after re-import Count = %d, want 2", count)
	}
}

func TestSQLite_Migrate_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dsn := filepath.Join(dir, "migrate.db")
	repo1, err := NewSQLitePostRepository(Config{DSN: dsn})
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	if err := repo1.Close(); err != nil {
		t.Fatalf("close 1: %v", err)
	}
	repo2, err := NewSQLitePostRepository(Config{DSN: dsn})
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	defer repo2.Close()
}

func TestSQLite_JSONDecodeError(t *testing.T) {
	repo := newRepo(t)
	tenantID := uuid.New()
	post := makePost(t, tenantID, "broken-json")
	ctx := withTenant(context.Background(), post)
	if err := repo.Create(ctx, post); err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Inject corrupted JSON directly via SQL.
	if _, err := repo.db.ExecContext(ctx, `UPDATE posts SET attachments = ? WHERE id = ?`, "{not json", post.ID.String()); err != nil {
		t.Fatalf("inject broken json: %v", err)
	}

	_, err := repo.GetByID(ctx, post.ID)
	if !errors.Is(err, core.ErrValidation) {
		t.Errorf("err = %v, want ErrValidation", err)
	}
}

// sanity: sqlitedb.Post + sql.NullString work as Scanner targets in List().
var _ = sql.NullString{}
