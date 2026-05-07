package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
)

func newTestHandlerWithCfg(posts ...*core.Post) *Handler {
	repo := &fakeRepo{posts: posts}
	svc := core.NewPostService(repo, core.SystemClock{})
	cfg := config.NewDefaultConfig()
	cfg.Auth.TenantDefault = uuid.New()
	cfg.Auth.AuthorDefault = uuid.New()
	return NewHandler(Config{Service: svc, Cfg: cfg})
}

func TestUI_PostNew_GET_RendersForm(t *testing.T) {
	t.Parallel()
	h := newTestHandlerWithCfg()
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/new", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"<form", "name=\"title\"", "name=\"slug\"", "name=\"tags\"", "Create"} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q", want)
		}
	}
}

func TestUI_PostNew_POST_CreatesAndRedirects(t *testing.T) {
	t.Parallel()
	h := newTestHandlerWithCfg()
	form := url.Values{
		"title": {"Brand new post"},
		"tags":  {"go, web"},
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasPrefix(loc, "/ui/posts/") || !strings.HasSuffix(loc, "?saved=1") {
		t.Errorf("redirect to %q, want /ui/posts/<id>?saved=1", loc)
	}
}

func TestUI_PostNew_POST_EmptyTitleRendersError(t *testing.T) {
	t.Parallel()
	h := newTestHandlerWithCfg()
	form := url.Values{"title": {""}}
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "обязателен") {
		t.Error("expected validation message")
	}
}

func TestUI_PostNew_POST_NoConfig_503(t *testing.T) {
	t.Parallel()
	h := newTestHandler() // no Cfg
	form := url.Values{"title": {"x"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/new", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
}

func TestUI_PostDelete_POST_DeletesAndRedirects(t *testing.T) {
	t.Parallel()
	post := samplePost("ToDelete", core.StatusDraft)
	h := newTestHandlerWithCfg(post)
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String()+"/delete", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if rec.Header().Get("Location") != "/ui/" {
		t.Errorf("redirect to %q, want /ui/", rec.Header().Get("Location"))
	}
}

func TestUI_PostDelete_GET_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	post := samplePost("Static", core.StatusDraft)
	h := newTestHandlerWithCfg(post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String()+"/delete", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}

func TestUI_PostDelete_NotFound(t *testing.T) {
	t.Parallel()
	h := newTestHandlerWithCfg()
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/00000000-0000-0000-0000-000000000099/delete", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}

func TestUI_PostEdit_HasDeleteButton(t *testing.T) {
	t.Parallel()
	post := samplePost("Edit me", core.StatusDraft)
	h := newTestHandlerWithCfg(post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String(), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "/delete") {
		t.Error("edit page must include delete form")
	}
	if !strings.Contains(body, "Удалить пост") {
		t.Error("delete button label missing")
	}
}
