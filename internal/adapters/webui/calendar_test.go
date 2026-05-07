package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

func TestUI_Calendar_RendersCurrentMonthByDefault(t *testing.T) {
	t.Parallel()
	now := time.Now()
	in := now.AddDate(0, 0, 1)
	h := newTestHandler(samplePostScheduled("InCurrent", core.StatusReady, &in, nil))
	req := httptest.NewRequest(http.MethodGet, "/ui/calendar", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"cal__grid", "Mon", "Sun", now.Month().String()} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	if !strings.Contains(body, "InCurrent") {
		t.Error("post in current month must appear in grid")
	}
}

func TestUI_Calendar_PrevNextMonthLinks(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/calendar?month=2026-05", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	for _, want := range []string{
		"month=2026-04", "month=2026-06", "May", "2026",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
}

func TestUI_Calendar_FiltersPostsOutsideMonth(t *testing.T) {
	t.Parallel()
	// Месяц для теста (фиксированный): 2026-05.
	inMay := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	inJun := time.Date(2026, 6, 10, 12, 0, 0, 0, time.UTC)
	h := newTestHandler(
		samplePostScheduled("MayPost", core.StatusReady, &inMay, nil),
		samplePostScheduled("JunPost", core.StatusReady, &inJun, nil),
	)
	req := httptest.NewRequest(http.MethodGet, "/ui/calendar?month=2026-05", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "MayPost") {
		t.Error("MayPost must appear in May calendar")
	}
	if strings.Contains(body, "JunPost") {
		t.Error("JunPost must not appear in May calendar")
	}
}

func TestUI_Calendar_PublishedShownByPublishedAt(t *testing.T) {
	t.Parallel()
	pub := time.Date(2026, 5, 7, 9, 0, 0, 0, time.UTC)
	p := samplePost("Released", core.StatusPublished)
	p.PublishedAt = &pub
	h := newTestHandler(p)
	req := httptest.NewRequest(http.MethodGet, "/ui/calendar?month=2026-05", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "Released") {
		t.Error("published post must appear by PublishedAt")
	}
	if !strings.Contains(body, "cal__item--published") {
		t.Error("published item must use --published modifier class")
	}
}

func TestUI_Calendar_InvalidMonthFallsBackToCurrent(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/calendar?month=garbage", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), time.Now().Month().String()) {
		t.Error("expected fallback to current month for invalid input")
	}
}

func TestUI_Calendar_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/ui/calendar", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}
