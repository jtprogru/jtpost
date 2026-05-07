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

func TestUI_Events_NoBus_503(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status=%d, want 503", rec.Code)
	}
}

func TestUI_Events_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	bus := core.NewMemoryBus(4)
	svc := core.NewPostService(&fakeRepo{}, core.SystemClock{})
	h := NewHandler(Config{Service: svc, Bus: bus})
	req := httptest.NewRequest(http.MethodPost, "/ui/events", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}

func TestUI_Events_StreamsPublishedEvent(t *testing.T) {
	t.Parallel()
	bus := core.NewMemoryBus(4)
	svc := core.NewPostService(&fakeRepo{}, core.SystemClock{})
	h := NewHandler(Config{Service: svc, Bus: bus})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/ui/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()

	// Дать handler'у подписаться на bus, потом publish.
	time.Sleep(50 * time.Millisecond)
	bus.Publish(core.Event{Topic: "post.created", Data: map[string]any{"id": "abc"}})
	time.Sleep(80 * time.Millisecond)
	cancel()
	<-done

	body := rec.Body.String()
	if !strings.Contains(body, ": connected") {
		t.Errorf("expected connected comment frame; body=%q", body)
	}
	if !strings.Contains(body, "event: post.created") {
		t.Errorf("expected post.created event; body=%q", body)
	}
	if !strings.Contains(body, `"abc"`) {
		t.Errorf("expected payload with id=abc; body=%q", body)
	}
}

func TestUI_Events_ContentType(t *testing.T) {
	t.Parallel()
	bus := core.NewMemoryBus(2)
	svc := core.NewPostService(&fakeRepo{}, core.SystemClock{})
	h := NewHandler(Config{Service: svc, Bus: bus})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/ui/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()
	time.Sleep(40 * time.Millisecond)
	cancel()
	<-done

	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Errorf("Content-Type=%q, want text/event-stream", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control=%q, want no-cache", got)
	}
}
