package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/core"
)

const validRevisionFile = `---
title: Restored Title
tags:
  - go
  - revert
---

restored body content
`

func TestUI_PostRevert_Success(t *testing.T) {
	t.Parallel()
	post := samplePost("Current Title", core.StatusReady, "current")
	post.Content = "current body"
	hp := &stubHistory{fileBody: []byte(validRevisionFile)}
	h := newHistoryHandler(t, hp, post)
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String()+"/history/abc12345/revert", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if loc := rec.Header().Get("Location"); !strings.HasSuffix(loc, "?reverted=1") {
		t.Errorf("redirect=%q, want suffix ?reverted=1", loc)
	}
	if post.Title != "Restored Title" {
		t.Errorf("title=%q, want Restored Title", post.Title)
	}
	if post.Content != "restored body content" {
		t.Errorf("content=%q", post.Content)
	}
	if len(post.Tags) != 2 || post.Tags[0] != "go" || post.Tags[1] != "revert" {
		t.Errorf("tags=%v", post.Tags)
	}
	if post.Status != core.StatusReady {
		t.Errorf("status changed: %s", post.Status)
	}
}

func TestUI_PostRevert_NoProvider_503(t *testing.T) {
	t.Parallel()
	post := samplePost("X", core.StatusDraft)
	h := newHistoryHandler(t, nil, post)
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String()+"/history/abc/revert", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status=%d, want 503", rec.Code)
	}
}

func TestUI_PostRevert_PostNotFound(t *testing.T) {
	t.Parallel()
	h := newHistoryHandler(t, &stubHistory{fileBody: []byte(validRevisionFile)})
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/00000000-0000-7000-8000-000000000000/history/abc/revert", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rec.Code)
	}
}

func TestUI_PostRevert_HashNotFound(t *testing.T) {
	t.Parallel()
	post := samplePost("X", core.StatusDraft)
	hp := &stubHistory{fileErr: core.ErrNotFound}
	h := newHistoryHandler(t, hp, post)
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String()+"/history/deadbeef/revert", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rec.Code)
	}
}

func TestUI_PostRevert_InvalidFrontmatter_422(t *testing.T) {
	t.Parallel()
	post := samplePost("X", core.StatusDraft)
	hp := &stubHistory{fileBody: []byte("no frontmatter at all")}
	h := newHistoryHandler(t, hp, post)
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String()+"/history/abc/revert", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status=%d, want 422", rec.Code)
	}
}

func TestUI_PostRevert_MissingTitle_422(t *testing.T) {
	t.Parallel()
	post := samplePost("X", core.StatusDraft)
	hp := &stubHistory{fileBody: []byte("---\ntags: [a]\n---\n\nbody\n")}
	h := newHistoryHandler(t, hp, post)
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String()+"/history/abc/revert", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("status=%d, want 422", rec.Code)
	}
}

func TestUI_PostRevert_GET_405(t *testing.T) {
	t.Parallel()
	post := samplePost("X", core.StatusDraft)
	h := newHistoryHandler(t, &stubHistory{fileBody: []byte(validRevisionFile)}, post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String()+"/history/abc/revert", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status=%d, want 405", rec.Code)
	}
}

func TestUI_RevisionPage_RendersRevertButton(t *testing.T) {
	t.Parallel()
	post := samplePost("X", core.StatusDraft)
	hp := &stubHistory{fileBody: []byte(validRevisionFile)}
	h := newHistoryHandler(t, hp, post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String()+"/history/abc12345", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{"Вернуть к этой версии", "/revert", "confirm("} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}
