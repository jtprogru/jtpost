package webui

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

func samplePostScheduled(title string, status core.PostStatus, sched *time.Time, deadline *time.Time) *core.Post {
	p := samplePost(title, status)
	p.ScheduledAt = sched
	p.Deadline = deadline
	return p
}

func TestUI_Plan_GroupsByDateAndShowsScheduledAndDeadline(t *testing.T) {
	t.Parallel()
	now := time.Now()
	day1 := now.Add(24 * time.Hour)
	day2 := now.Add(48 * time.Hour)
	h := newTestHandler(
		samplePostScheduled("ScheduledOne", core.StatusReady, &day1, nil),
		samplePostScheduled("DeadlineOne", core.StatusDraft, nil, &day2),
		samplePostScheduled("AlsoDay1", core.StatusReady, &day1, nil),
	)
	req := httptest.NewRequest(http.MethodGet, "/ui/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{"ScheduledOne", "DeadlineOne", "AlsoDay1", "scheduled", "deadline"} {
		if !strings.Contains(body, want) {
			t.Errorf("body missing %q", want)
		}
	}
	// Day1 должен идти до Day2 (chronological order).
	if strings.Index(body, "AlsoDay1") > strings.Index(body, "DeadlineOne") {
		t.Errorf("expected day1 before day2 in chronological order")
	}
}

func TestUI_Plan_EmptyState(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodGet, "/ui/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Нет запланированных") {
		t.Error("expected empty-state message")
	}
}

func TestUI_Plan_ExcludesPublished(t *testing.T) {
	t.Parallel()
	tomorrow := time.Now().Add(24 * time.Hour)
	h := newTestHandler(
		samplePostScheduled("Pending", core.StatusReady, &tomorrow, nil),
		samplePostScheduled("Done", core.StatusPublished, &tomorrow, nil),
	)
	req := httptest.NewRequest(http.MethodGet, "/ui/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "Pending") {
		t.Error("expected Pending shown")
	}
	if strings.Contains(body, ">Done<") {
		t.Errorf("published post must not appear: %s", body)
	}
}

func TestUI_Plan_RespectsDaysWindow(t *testing.T) {
	t.Parallel()
	soon := time.Now().Add(2 * 24 * time.Hour)
	far := time.Now().Add(60 * 24 * time.Hour)
	h := newTestHandler(
		samplePostScheduled("SoonPost", core.StatusReady, &soon, nil),
		samplePostScheduled("FarPost", core.StatusReady, &far, nil),
	)
	req := httptest.NewRequest(http.MethodGet, "/ui/plan?days=7", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body := rec.Body.String()
	if !strings.Contains(body, "SoonPost") {
		t.Error("expected SoonPost shown within 7 days")
	}
	if strings.Contains(body, "FarPost") {
		t.Errorf("FarPost outside 7-day window must not appear")
	}
	if !strings.Contains(body, `value="7"`) {
		t.Errorf("days input must reflect query value")
	}
}

func TestUI_Plan_MethodNotAllowed(t *testing.T) {
	t.Parallel()
	h := newTestHandler()
	req := httptest.NewRequest(http.MethodPost, "/ui/plan", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status=%d, want 405", rec.Code)
	}
}
