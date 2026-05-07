package webui

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/core"
)

func TestUI_PostEdit_GET_RendersForm(t *testing.T) {
	t.Parallel()
	post := samplePost("Hello world", core.StatusDraft, "go", "tdd")
	post.Content = "# Heading\n\nSome **bold** text"
	h := newTestHandler(post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String(), nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		"Hello world",
		"name=\"title\"",
		"name=\"content\"",
		"name=\"status\"",
		"name=\"tags\"",
		"go, tdd",
		// MD-rendered preview:
		"<h1", "<strong>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestUI_PostEdit_GET_NotFound(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/00000000-0000-0000-0000-000000000001", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d, want 404", rec.Code)
	}
}

func TestUI_PostEdit_GET_InvalidID(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/not-a-uuid", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want 400", rec.Code)
	}
}

func TestUI_PostEdit_POST_SavesAndRedirects(t *testing.T) {
	t.Parallel()
	post := samplePost("Original", core.StatusDraft)
	h := newTestHandler(post)
	form := url.Values{
		"title":   {"Updated title"},
		"status":  {"draft"},
		"tags":    {"new, tag"},
		"content": {"Updated content"},
	}
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d, want 303; body=%s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.HasSuffix(loc, "?saved=1") {
		t.Errorf("location=%q, want suffix ?saved=1", loc)
	}
}

func TestUI_PostEdit_POST_EmptyTitleErrors(t *testing.T) {
	t.Parallel()
	post := samplePost("Original", core.StatusDraft)
	h := newTestHandler(post)
	form := url.Values{"title": {""}, "content": {"x"}, "status": {"draft"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status=%d, want 422", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "не может быть пустым") {
		t.Error("expected validation error message")
	}
}

func TestUI_PostEdit_GET_SavedFlash(t *testing.T) {
	t.Parallel()
	post := samplePost("Hello", core.StatusDraft)
	h := newTestHandler(post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String()+"?saved=1", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Сохранено") {
		t.Error("expected saved flash")
	}
}

func TestUI_Preview_RendersMarkdown(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	form := url.Values{"content": {"# Title\n\nA **bold** paragraph and `inline code`."}}
	req := httptest.NewRequest(http.MethodPost, "/ui/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"<h1", "Title", "<strong>bold</strong>", "<code>inline code</code>"} {
		if !strings.Contains(body, want) {
			t.Errorf("preview missing %q; body=%s", want, body)
		}
	}
}

func TestUI_Preview_StripsRawHTML(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	form := url.Values{"content": {"<script>alert(1)</script>\n\nsafe text"}}
	req := httptest.NewRequest(http.MethodPost, "/ui/preview", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if strings.Contains(body, "<script>") {
		t.Errorf("raw <script> must be stripped/escaped: %s", body)
	}
	if !strings.Contains(body, "safe text") {
		t.Errorf("safe text must remain: %s", body)
	}
}

func TestUI_Preview_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/preview", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}
