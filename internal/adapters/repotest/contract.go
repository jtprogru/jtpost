// Package repotest содержит переиспользуемый поведенческий контракт для
// реализаций core.PostRepository. Используется fs/sqlite/postgres адаптерами,
// чтобы гарантировать семантическую идентичность между бэкендами.
//
// Контракт публикует функцию RunContract(t, factory). Каждый вызов factory
// должен возвращать "пустой" репозиторий с уникальным состоянием — обычно через
// t.TempDir() или testcontainers. Cleanup-функция гарантированно вызывается
// через t.Cleanup.
//
// Capabilities-флаги позволяют скипать кейсы, не поддерживаемые конкретным
// бэкендом (например, optimistic-lock для FS).
package repotest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/jtprogru/jtpost/internal/core"
)

// Capabilities описывает поддерживаемые бэкендом фичи.
// Используется RunContract для условного скипа SQL-only сценариев.
type Capabilities struct {
	// OptimisticLock=true означает, что Update со stale Revision возвращает
	// core.ErrConflict (поддержано SQL-адаптерами через WHERE revision=?).
	OptimisticLock bool
	// Transactions=true означает, что репозиторий реализует
	// core.TransactionalRepository (BeginTx).
	Transactions bool
}

// Factory создаёт новый "пустой" репозиторий для одного субтеста.
// Возвращает репозиторий, его capabilities и функцию cleanup, которую
// RunContract регистрирует через t.Cleanup.
type Factory func(t *testing.T) (core.PostRepository, Capabilities, func())

// RunContract запускает поведенческий контракт-сьют.
// Минимум 15 субтестов; SQL-only сценарии скипаются по Capabilities.
func RunContract(t *testing.T, factory Factory) {
	t.Helper()

	t.Run("CreateGet_RoundTrip", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)

		tenant := uuid.New()
		ctx := core.WithTenant(context.Background(), tenant)

		excerpt := "intro"
		revSHA := "deadbeef"
		cover := &core.Attachment{
			ID:   uuid.New(),
			Type: core.AttachmentTypePhoto,
			Path: "cover.jpg",
		}
		p := newPost(t, tenant, "round-trip")
		p.Excerpt = &excerpt
		p.RevisionSHA = &revSHA
		p.CoverImage = cover
		p.Attachments = []core.Attachment{
			{ID: uuid.New(), Type: core.AttachmentTypeDocument, Path: "doc.pdf"},
		}

		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		got, err := repo.GetByID(ctx, p.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Title != p.Title || got.Slug != p.Slug {
			t.Errorf("title/slug mismatch: %q/%q vs %q/%q", got.Title, got.Slug, p.Title, p.Slug)
		}
		if got.TenantID != p.TenantID || got.AuthorID != p.AuthorID {
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
		if got.RevisionSHA == nil || *got.RevisionSHA != revSHA {
			t.Errorf("RevisionSHA mismatch")
		}
	})

	t.Run("GetByID_NoTenantInCtx", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		_, err := repo.GetByID(context.Background(), core.PostID(uuid.New()))
		if !errors.Is(err, core.ErrTenantMismatch) {
			t.Errorf("err = %v, want ErrTenantMismatch", err)
		}
	})

	t.Run("GetByID_OtherTenant_ReturnsNotFound", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenantA := uuid.New()
		tenantB := uuid.New()
		ctxA := core.WithTenant(context.Background(), tenantA)
		ctxB := core.WithTenant(context.Background(), tenantB)
		p := newPost(t, tenantA, "iso-a")
		if err := repo.Create(ctxA, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		if _, err := repo.GetByID(ctxB, p.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("cross-tenant GetByID = %v, want ErrNotFound", err)
		}
	})

	t.Run("GetBySlug_TenantScope", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenantA := uuid.New()
		tenantB := uuid.New()
		ctxA := core.WithTenant(context.Background(), tenantA)
		ctxB := core.WithTenant(context.Background(), tenantB)
		pA := newPost(t, tenantA, "shared-slug")
		pB := newPost(t, tenantB, "shared-slug")
		if err := repo.Create(ctxA, pA); err != nil {
			t.Fatalf("Create A: %v", err)
		}
		if err := repo.Create(ctxB, pB); err != nil {
			t.Fatalf("Create B: %v", err)
		}
		gotA, err := repo.GetBySlug(ctxA, "shared-slug")
		if err != nil {
			t.Fatalf("GetBySlug A: %v", err)
		}
		if gotA.ID != pA.ID {
			t.Errorf("GetBySlug A returned wrong post")
		}
		gotB, err := repo.GetBySlug(ctxB, "shared-slug")
		if err != nil {
			t.Fatalf("GetBySlug B: %v", err)
		}
		if gotB.ID != pB.ID {
			t.Errorf("GetBySlug B returned wrong post")
		}
	})

	t.Run("GetBySlug_NotFound", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		ctx := core.WithTenant(context.Background(), uuid.New())
		if _, err := repo.GetBySlug(ctx, "nope-not-here"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("List_NilTenant_Validation", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		_, err := repo.List(context.Background(), core.PostFilter{})
		if !errors.Is(err, core.ErrValidation) {
			t.Errorf("err = %v, want ErrValidation", err)
		}
	})

	t.Run("List_TenantOnly", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenantA := uuid.New()
		tenantB := uuid.New()
		ctxA := core.WithTenant(context.Background(), tenantA)
		ctxB := core.WithTenant(context.Background(), tenantB)
		if err := repo.Create(ctxA, newPost(t, tenantA, "ta-1")); err != nil {
			t.Fatal(err)
		}
		if err := repo.Create(ctxA, newPost(t, tenantA, "ta-2")); err != nil {
			t.Fatal(err)
		}
		if err := repo.Create(ctxB, newPost(t, tenantB, "tb-1")); err != nil {
			t.Fatal(err)
		}
		got, err := repo.List(ctxA, core.PostFilter{TenantID: tenantA})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("List_AuthorFilter", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenant := uuid.New()
		ctx := core.WithTenant(context.Background(), tenant)
		authorA := uuid.New()
		authorB := uuid.New()
		p1 := newPost(t, tenant, "af-1")
		p1.AuthorID = authorA
		p2 := newPost(t, tenant, "af-2")
		p2.AuthorID = authorA
		p3 := newPost(t, tenant, "af-3")
		p3.AuthorID = authorB
		for _, p := range []*core.Post{p1, p2, p3} {
			if err := repo.Create(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.List(ctx, core.PostFilter{TenantID: tenant, AuthorID: &authorA})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("List_StatusFilter", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenant := uuid.New()
		ctx := core.WithTenant(context.Background(), tenant)
		p1 := newPost(t, tenant, "st-1")
		p1.Status = core.StatusDraft
		p2 := newPost(t, tenant, "st-2")
		p2.Status = core.StatusReady
		p3 := newPost(t, tenant, "st-3")
		p3.Status = core.StatusDraft
		for _, p := range []*core.Post{p1, p2, p3} {
			if err := repo.Create(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.List(ctx, core.PostFilter{
			TenantID: tenant,
			Statuses: []core.PostStatus{core.StatusDraft},
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Errorf("len = %d, want 2", len(got))
		}
	})

	t.Run("List_SortBy_Created", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenant := uuid.New()
		ctx := core.WithTenant(context.Background(), tenant)
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		for i, slug := range []string{"sa", "sb", "sc"} {
			p := newPost(t, tenant, slug)
			p.Title = "T" + string(rune('A'+i))
			p.CreatedAt = base.Add(time.Duration(i) * time.Hour)
			p.UpdatedAt = p.CreatedAt
			if err := repo.Create(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		gotAsc, err := repo.List(ctx, core.PostFilter{
			TenantID: tenant, SortBy: "created_at", SortOrder: "asc",
		})
		if err != nil {
			t.Fatalf("List asc: %v", err)
		}
		if len(gotAsc) < 3 || gotAsc[0].Title != "TA" {
			t.Errorf("asc first = %v, want TA", titles(gotAsc))
		}
		gotDesc, err := repo.List(ctx, core.PostFilter{
			TenantID: tenant, SortBy: "created_at", SortOrder: "desc",
		})
		if err != nil {
			t.Fatalf("List desc: %v", err)
		}
		if len(gotDesc) < 3 || gotDesc[0].Title != "TC" {
			t.Errorf("desc first = %v, want TC", titles(gotDesc))
		}
	})

	t.Run("List_LimitOffset", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenant := uuid.New()
		ctx := core.WithTenant(context.Background(), tenant)
		base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
		for i := range 5 {
			p := newPost(t, tenant, string(rune('a'+i))+"-lo")
			p.Title = "T" + string(rune('A'+i))
			p.CreatedAt = base.Add(time.Duration(i) * time.Hour)
			p.UpdatedAt = p.CreatedAt
			if err := repo.Create(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		got, err := repo.List(ctx, core.PostFilter{
			TenantID: tenant, SortBy: "created_at", SortOrder: "asc",
			Limit: 2, Offset: 1,
		})
		if err != nil {
			t.Fatalf("List: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("len = %d, want 2", len(got))
		}
		if got[0].Title != "TB" || got[1].Title != "TC" {
			t.Errorf("got [%s,%s], want [TB,TC]", got[0].Title, got[1].Title)
		}
	})

	t.Run("List_InvalidSort", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		_, err := repo.List(context.Background(), core.PostFilter{
			TenantID: uuid.New(),
			SortBy:   "DROP TABLE",
		})
		if !errors.Is(err, core.ErrValidation) {
			t.Errorf("err = %v, want ErrValidation", err)
		}
	})

	t.Run("Update_Success", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenant := uuid.New()
		ctx := core.WithTenant(context.Background(), tenant)
		p := newPost(t, tenant, "upd-ok")
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		p.Title = "Updated"
		p.Revision = 2
		p.UpdatedAt = p.UpdatedAt.Add(time.Hour)
		if err := repo.Update(ctx, p); err != nil {
			t.Fatalf("Update: %v", err)
		}
		got, err := repo.GetByID(ctx, p.ID)
		if err != nil {
			t.Fatalf("GetByID: %v", err)
		}
		if got.Title != "Updated" || got.Revision != 2 {
			t.Errorf("title=%q rev=%d", got.Title, got.Revision)
		}
	})

	t.Run("Update_RevisionConflict", func(t *testing.T) {
		repo, caps, cleanup := factory(t)
		t.Cleanup(cleanup)
		if !caps.OptimisticLock {
			t.Skip("backend does not support optimistic locking")
		}
		tenant := uuid.New()
		ctx := core.WithTenant(context.Background(), tenant)
		p := newPost(t, tenant, "upd-conflict")
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		// First update bumps revision: 1 -> 2.
		p2 := *p
		p2.Title = "v2"
		p2.Revision = 2
		p2.UpdatedAt = time.Now().UTC()
		if err := repo.Update(ctx, &p2); err != nil {
			t.Fatalf("first update: %v", err)
		}
		// Stale: caller still thinks current revision is 1, sets new to 2.
		stale := *p
		stale.Title = "stale"
		stale.Revision = 2
		stale.UpdatedAt = time.Now().UTC()
		if err := repo.Update(ctx, &stale); !errors.Is(err, core.ErrConflict) {
			t.Errorf("err = %v, want ErrConflict", err)
		}
	})

	t.Run("Update_NotFound", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenant := uuid.New()
		ctx := core.WithTenant(context.Background(), tenant)
		p := newPost(t, tenant, "ghost")
		p.Revision = 2
		if err := repo.Update(ctx, p); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	})

	t.Run("Delete_TenantIsolation", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		tenantA := uuid.New()
		tenantB := uuid.New()
		ctxA := core.WithTenant(context.Background(), tenantA)
		ctxB := core.WithTenant(context.Background(), tenantB)
		p := newPost(t, tenantA, "del-iso")
		if err := repo.Create(ctxA, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		// Delete from other tenant: либо ErrNotFound, либо nil — но в любом случае
		// пост tenantA НЕ должен исчезнуть.
		_ = repo.Delete(ctxB, p.ID)
		if _, err := repo.GetByID(ctxA, p.ID); err != nil {
			t.Errorf("post unexpectedly lost: %v", err)
		}
	})

	t.Run("ImportPosts_Count", func(t *testing.T) {
		repo, _, cleanup := factory(t)
		t.Cleanup(cleanup)
		mig, ok := repo.(core.MigratableRepository)
		if !ok {
			t.Skip("backend does not implement MigratableRepository")
		}
		tenant := uuid.New()
		posts := []*core.Post{
			newPost(t, tenant, "imp-1"),
			newPost(t, tenant, "imp-2"),
			newPost(t, tenant, "imp-3"),
		}
		if err := mig.ImportPosts(context.Background(), posts); err != nil {
			t.Fatalf("ImportPosts: %v", err)
		}
		count, err := mig.Count(context.Background())
		if err != nil {
			t.Fatalf("Count: %v", err)
		}
		if count != 3 {
			t.Errorf("Count = %d, want 3", count)
		}
	})

	t.Run("BeginTx_CommitNoOp", func(t *testing.T) {
		repo, caps, cleanup := factory(t)
		t.Cleanup(cleanup)
		if !caps.Transactions {
			t.Skip("backend does not support transactions")
		}
		tr, ok := repo.(core.TransactionalRepository)
		if !ok {
			t.Skip("backend does not implement TransactionalRepository")
		}
		ctx := core.WithTenant(context.Background(), uuid.New())
		_, commit, err := tr.BeginTx(ctx)
		if err != nil {
			t.Fatalf("BeginTx: %v", err)
		}
		if err := commit(); err != nil {
			t.Errorf("commit: %v", err)
		}
	})
}

// newPost — фабрика поста с минимальным набором обязательных полей.
func newPost(t *testing.T, tenantID uuid.UUID, slug string) *core.Post {
	t.Helper()
	now := time.Now().UTC().Truncate(time.Second)
	return &core.Post{
		ID:        core.PostID(uuid.New()),
		TenantID:  tenantID,
		AuthorID:  uuid.New(),
		Title:     "Title " + slug,
		Slug:      slug,
		Status:    core.StatusDraft,
		Tags:      []string{"go", "tdd"},
		Content:   "content " + slug,
		Revision:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

func titles(ps []*core.Post) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Title
	}
	return out
}
