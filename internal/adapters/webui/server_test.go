package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

// fakeRepo — минимальный mock core.PostRepository для UI-тестов.
type fakeRepo struct {
	posts []*core.Post
}

func (r *fakeRepo) List(_ context.Context, _ core.PostFilter) ([]*core.Post, error) {
	return r.posts, nil
}

func (r *fakeRepo) GetByID(_ context.Context, id core.PostID) (*core.Post, error) {
	for _, p := range r.posts {
		if p.ID == id {
			return p, nil
		}
	}
	return nil, core.ErrNotFound
}

func (r *fakeRepo) Create(_ context.Context, p *core.Post) error {
	r.posts = append(r.posts, p)
	return nil
}
func (r *fakeRepo) Update(_ context.Context, _ *core.Post) error  { return nil }
func (r *fakeRepo) Delete(_ context.Context, _ core.PostID) error { return nil }
func (r *fakeRepo) GetBySlug(_ context.Context, _ string) (*core.Post, error) {
	return nil, core.ErrNotFound
}

func samplePost(title string, status core.PostStatus, tags ...string) *core.Post {
	return &core.Post{
		ID:        core.PostID(uuid.New()),
		TenantID:  uuid.New(),
		AuthorID:  uuid.New(),
		Title:     title,
		Slug:      strings.ToLower(strings.ReplaceAll(title, " ", "-")),
		Status:    status,
		Tags:      tags,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

func newTestHandler(posts ...*core.Post) *Handler {
	repo := &fakeRepo{posts: posts}
	svc := core.NewPostService(repo, core.SystemClock{})
	return NewHandler(svc, nil)
}

func TestUI_Dashboard_RendersHTML(t *testing.T) {
	t.Parallel()
	h := newTestHandler(samplePost("Hello", core.StatusDraft, "go"))
	req := httptest.NewRequest(http.MethodGet, "/ui/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<!doctype html>", "Posts", "Hello", "htmx.min.js", "app.css"} {
		if !strings.Contains(strings.ToLower(body), strings.ToLower(want)) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestUI_PostsPartial_ReturnsTableOnly(t *testing.T) {
	t.Parallel()
	h := newTestHandler(samplePost("Alpha", core.StatusDraft), samplePost("Beta", core.StatusReady))
	req := httptest.NewRequest(http.MethodGet, "/ui/posts", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "<html") || strings.Contains(body, "<body") {
		t.Errorf("partial must not include full HTML wrapper, got: %s", body[:min(200, len(body))])
	}
	for _, want := range []string{"Alpha", "Beta", "posts-table"} {
		if !strings.Contains(body, want) {
			t.Errorf("partial missing %q", want)
		}
	}
}

func TestUI_PostsPartial_FilterByStatus(t *testing.T) {
	t.Parallel()
	h := newTestHandler(
		samplePost("DraftOne", core.StatusDraft),
		samplePost("ReadyOne", core.StatusReady),
		samplePost("DraftTwo", core.StatusDraft),
	)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts?status=draft", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "DraftOne") || !strings.Contains(body, "DraftTwo") {
		t.Errorf("expected drafts shown")
	}
	if strings.Contains(body, "ReadyOne") {
		t.Errorf("ReadyOne must be filtered out")
	}
}

func TestUI_PostsPartial_FilterBySearch(t *testing.T) {
	t.Parallel()
	h := newTestHandler(
		samplePost("Go programming", core.StatusDraft),
		samplePost("Rust note", core.StatusDraft),
	)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts?search=rust", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "Rust note") {
		t.Errorf("expected Rust note shown")
	}
	if strings.Contains(body, "Go programming") {
		t.Errorf("Go programming must be filtered out")
	}
}

func TestUI_PostsPartial_Empty(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/posts", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Нет постов") {
		t.Errorf("expected empty state message")
	}
}

func TestUI_Static_HTMXServed(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/static/htmx.min.js", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if rec.Body.Len() < 1000 {
		t.Errorf("htmx.min.js looks too small: %d bytes", rec.Body.Len())
	}
}

func TestUI_Static_AppCSS(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/static/app.css", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "--bg") {
		t.Errorf("app.css missing CSS variables")
	}
}

func TestUI_NotFound(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/unknown-page", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}
