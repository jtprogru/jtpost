package webui

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

type stubHistory struct {
	entries []core.HistoryEntry
	err     error
	called  int
}

func (s *stubHistory) History(_ context.Context, _ *core.Post, _ int) ([]core.HistoryEntry, error) {
	s.called++
	return s.entries, s.err
}

func newHistoryHandler(t *testing.T, hp core.HistoryProvider, posts ...*core.Post) *Handler {
	t.Helper()
	repo := &fakeRepo{posts: posts}
	svc := core.NewPostService(repo, core.SystemClock{})
	return NewHandler(Config{Service: svc, History: hp})
}

func TestUI_PostHistory_NoProvider_ShowsStub(t *testing.T) {
	t.Parallel()
	post := samplePost("Hello", core.StatusDraft)
	h := newHistoryHandler(t, nil, post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String()+"/history", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "не поддерживает версионирование") {
		t.Error("expected stub message when HistoryProvider is nil")
	}
}

func TestUI_PostHistory_RendersEntries(t *testing.T) {
	t.Parallel()
	post := samplePost("Cat", core.StatusReady)
	hp := &stubHistory{entries: []core.HistoryEntry{
		{Hash: "abc12345def", ShortHash: "abc12345", Author: "Alice", Message: "initial", At: time.Date(2026, 5, 1, 10, 0, 0, 0, time.UTC)},
		{Hash: "fff99988bbb", ShortHash: "fff99988", Author: "Bob", Message: "tweak", At: time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)},
	}}
	h := newHistoryHandler(t, hp, post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String()+"/history", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{"abc12345", "fff99988", "Alice", "Bob", "initial", "tweak"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if hp.called != 1 {
		t.Errorf("expected 1 call, got %d", hp.called)
	}
}

func TestUI_PostHistory_EmptyEntries(t *testing.T) {
	t.Parallel()
	post := samplePost("Empty", core.StatusDraft)
	h := newHistoryHandler(t, &stubHistory{}, post)
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/"+post.ID.String()+"/history", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Изменений ещё не зафиксировано") {
		t.Error("expected empty-state message")
	}
}

func TestUI_PostHistory_NotFound(t *testing.T) {
	t.Parallel()
	h := newHistoryHandler(t, &stubHistory{})
	req := httptest.NewRequest(http.MethodGet, "/ui/posts/00000000-0000-7000-8000-000000000000/history", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("status=%d, want 404", rec.Code)
	}
}

func TestUI_PostHistory_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	post := samplePost("X", core.StatusDraft)
	h := newHistoryHandler(t, &stubHistory{}, post)
	req := httptest.NewRequest(http.MethodPost, "/ui/posts/"+post.ID.String()+"/history", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status=%d, want 405", rec.Code)
	}
}
