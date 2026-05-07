//go:build integration

package postgres

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/repotest"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestPostgres_RunContract — общий repotest.RunContract против postgres.
// Каждый субтест поднимает отдельный testcontainer; при отсутствии Docker
// субтест скипается через t.Skip внутри setupContainer.
func TestPostgres_RunContract(t *testing.T) {
	repotest.RunContract(t, func(t *testing.T) (core.PostRepository, repotest.Capabilities, func()) {
		dsn := setupContainer(t)
		repo, err := NewPostgresRepository(context.Background(), Config{
			DSN:             dsn,
			MaxOpenConns:    4,
			MaxIdleConns:    1,
			ConnMaxLifetime: 5 * time.Minute,
		})
		if err != nil {
			t.Fatalf("NewPostgresRepository: %v", err)
		}
		return repo, repotest.Capabilities{
			OptimisticLock: true,
			Transactions:   true,
		}, func() { _ = repo.Close() }
	})
}

// setupContainer стартует Postgres-контейнер; на отсутствие Docker — t.Skip.
func setupContainer(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	pgC, err := tcpostgres.Run(ctx, "postgres:16-alpine",
		tcpostgres.WithDatabase("jtpost"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		// Postgres alpine стартует postgres дважды (init + serve). Wait по порту
		// ловит первый запуск и приводит к "connection reset by peer" в CI.
		// Используем log-strategy + readiness-проверку через pg_isready.
		testcontainers.WithWaitStrategy(
			wait.ForAll(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2),
				wait.ForListeningPort("5432/tcp"),
			).WithStartupTimeoutDefault(90*time.Second),
		),
	)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "Cannot connect") ||
			strings.Contains(msg, "docker") ||
			strings.Contains(msg, "Docker") ||
			strings.Contains(msg, "rootless") ||
			strings.Contains(msg, "daemon") {
			t.Skip("docker not available: " + msg)
		}
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(ctx) })

	connStr, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatalf("ConnectionString: %v", err)
	}
	return connStr
}

func newRepo(t *testing.T) *PostRepository {
	t.Helper()
	dsn := setupContainer(t)
	repo, err := NewPostgresRepository(context.Background(), Config{
		DSN:             dsn,
		MaxOpenConns:    4,
		MaxIdleConns:    1,
		ConnMaxLifetime: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("NewPostgresRepository: %v", err)
	}
	t.Cleanup(func() { _ = repo.Close() })
	return repo
}

func samplePost(tenant, author uuid.UUID, slug string) *core.Post {
	now := time.Now().UTC().Truncate(time.Microsecond)
	excerpt := "intro"
	return &core.Post{
		ID:        core.PostID(uuid.New()),
		TenantID:  tenant,
		AuthorID:  author,
		Title:     "Hello " + slug,
		Slug:      slug,
		Status:    core.StatusIdea,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
		Tags:      []string{"go", "tdd"},
		Excerpt:   &excerpt,
		Content:   "# Hello",
		External:  core.ExternalLinks{},
	}
}

func TestPostgres_CreateGetRoundtrip(t *testing.T) {
	repo := newRepo(t)
	tenant := uuid.New()
	author := uuid.New()
	ctx := core.WithTenant(context.Background(), tenant)
	p := samplePost(tenant, author, "round-trip")

	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.Slug != p.Slug || got.Title != p.Title || got.Revision != 1 {
		t.Fatalf("mismatch: %+v", got)
	}
	if len(got.Tags) != 2 {
		t.Fatalf("tags lost: %+v", got.Tags)
	}
}

func TestPostgres_TenantIsolation(t *testing.T) {
	repo := newRepo(t)
	tenantA := uuid.New()
	tenantB := uuid.New()
	author := uuid.New()
	ctxA := core.WithTenant(context.Background(), tenantA)
	ctxB := core.WithTenant(context.Background(), tenantB)
	p := samplePost(tenantA, author, "alpha")
	if err := repo.Create(ctxA, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	if _, err := repo.GetByID(ctxB, p.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("expected ErrNotFound from other tenant, got %v", err)
	}
	if err := repo.Delete(ctxB, p.ID); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("Delete cross-tenant: expected ErrNotFound, got %v", err)
	}
}

func TestPostgres_List_FilterSortLimit(t *testing.T) {
	repo := newRepo(t)
	tenant := uuid.New()
	author := uuid.New()
	ctx := core.WithTenant(context.Background(), tenant)
	for _, slug := range []string{"a", "b", "c"} {
		p := samplePost(tenant, author, slug)
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("Create: %v", err)
		}
		time.Sleep(2 * time.Millisecond)
	}
	got, err := repo.List(ctx, core.PostFilter{
		TenantID:  tenant,
		SortBy:    "title",
		SortOrder: "asc",
		Limit:     2,
	})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("limit not applied: %d", len(got))
	}
	if got[0].Slug != "a" || got[1].Slug != "b" {
		t.Fatalf("sort wrong: %+v", []string{got[0].Slug, got[1].Slug})
	}
}

func TestPostgres_List_InvalidSort(t *testing.T) {
	repo := newRepo(t)
	tenant := uuid.New()
	_, err := repo.List(context.Background(), core.PostFilter{
		TenantID: tenant,
		SortBy:   "DROP TABLE",
	})
	if !errors.Is(err, core.ErrValidation) {
		t.Fatalf("expected ErrValidation, got %v", err)
	}
}

func TestPostgres_Update_RevisionConflict(t *testing.T) {
	repo := newRepo(t)
	tenant := uuid.New()
	author := uuid.New()
	ctx := core.WithTenant(context.Background(), tenant)
	p := samplePost(tenant, author, "concurrent")
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Bump in DB by simulating a successful update.
	p2 := *p
	p2.Title = "v2"
	p2.Revision = 2
	p2.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, &p2); err != nil {
		t.Fatalf("first update: %v", err)
	}
	// Stale write — uses Revision=2 (so WHERE revision = 2-1 = 1) but actual is 2.
	stale := *p
	stale.Title = "stale"
	stale.Revision = 2 // pretending caller still thinks revision was 1 -> service incremented to 2
	stale.UpdatedAt = time.Now().UTC()
	if err := repo.Update(ctx, &stale); !errors.Is(err, core.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestPostgres_Update_NotFound(t *testing.T) {
	repo := newRepo(t)
	tenant := uuid.New()
	author := uuid.New()
	ctx := core.WithTenant(context.Background(), tenant)
	p := samplePost(tenant, author, "ghost")
	p.Revision = 2
	if err := repo.Update(ctx, p); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestPostgres_PoolLifecycle(t *testing.T) {
	repo := newRepo(t)
	if err := repo.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	tenant := uuid.New()
	ctx := core.WithTenant(context.Background(), tenant)
	if _, err := repo.GetByID(ctx, core.PostID(uuid.New())); err == nil {
		t.Fatalf("expected error after Close")
	}
}

func TestPostgres_PingFailFast(t *testing.T) {
	// No container — point at bogus DSN; should fail-fast within seconds.
	if testing.Short() {
		t.Skip("short")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_, err := NewPostgresRepository(ctx, Config{
		DSN: "postgres://invalid:invalid@127.0.0.1:1/nope?sslmode=disable&connect_timeout=2",
	})
	if err == nil {
		t.Fatalf("expected error for bad DSN")
	}
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Fatalf("expected ErrConfigInvalid, got %v", err)
	}
}

func TestPostgres_MigrateIdempotent(t *testing.T) {
	dsn := setupContainer(t)
	r1, err := NewPostgresRepository(context.Background(), Config{DSN: dsn})
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	_ = r1.Close()
	r2, err := NewPostgresRepository(context.Background(), Config{DSN: dsn})
	if err != nil {
		t.Fatalf("second open: %v", err)
	}
	t.Cleanup(func() { _ = r2.Close() })
}
