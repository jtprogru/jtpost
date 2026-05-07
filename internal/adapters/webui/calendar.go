package webui

import (
	"net/http"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
)

// handleCalendar — GET /ui/calendar?month=YYYY-MM (default: текущий месяц).
// Месячная сетка с постами по дате (scheduled / deadline / published).
func (h *Handler) handleCalendar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	now := time.Now()
	year, month := now.Year(), int(now.Month())
	if m := r.URL.Query().Get("month"); m != "" {
		if t, err := time.Parse("2006-01", m); err == nil {
			year, month = t.Year(), int(t.Month())
		}
	}
	loc := now.Location()
	monthStart := time.Date(year, time.Month(month), 1, 0, 0, 0, 0, loc)
	monthEnd := monthStart.AddDate(0, 1, 0)

	filter := core.PostFilter{}
	if t, ok := core.TenantFromContext(r.Context()); ok {
		filter.TenantID = t
	}
	posts, err := h.service.ListPosts(r.Context(), filter)
	if err != nil {
		h.log.Error("ui calendar list: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	byDay := map[string][]components.CalendarItem{}
	add := func(t *time.Time, p *core.Post, dt string) {
		if t == nil || t.Before(monthStart) || !t.Before(monthEnd) {
			return
		}
		k := t.Format("2006-01-02")
		byDay[k] = append(byDay[k], components.CalendarItem{Post: p, DateType: dt})
	}
	for _, p := range posts {
		if p.Status == core.StatusPublished {
			add(p.PublishedAt, p, "published")
			continue
		}
		switch {
		case p.ScheduledAt != nil:
			add(p.ScheduledAt, p, "schedule")
		case p.Deadline != nil:
			add(p.Deadline, p, "deadline")
		}
	}

	// Mon-first неделя: смещение от понедельника (Mon=0..Sun=6).
	startOffset := (int(monthStart.Weekday()) + 6) % 7
	gridStart := monthStart.AddDate(0, 0, -startOffset)
	lastDay := monthEnd.AddDate(0, 0, -1)
	endOffset := (int(lastDay.Weekday()) + 6) % 7
	gridEnd := lastDay.AddDate(0, 0, 7-endOffset) // exclusive

	var weeks [][]components.CalendarDay
	for cur := gridStart; cur.Before(gridEnd); {
		week := make([]components.CalendarDay, 0, 7)
		for range 7 {
			week = append(week, components.CalendarDay{
				Date:    cur,
				InMonth: cur.Month() == time.Month(month) && cur.Year() == year,
				Items:   byDay[cur.Format("2006-01-02")],
			})
			cur = cur.AddDate(0, 0, 1)
		}
		weeks = append(weeks, week)
	}

	prev := monthStart.AddDate(0, -1, 0)
	next := monthStart.AddDate(0, 1, 0)
	props := components.CalendarProps{
		Year:      year,
		Month:     time.Month(month),
		Weeks:     weeks,
		PrevMonth: prev.Format("2006-01"),
		NextMonth: next.Format("2006-01"),
		Today:     now,
		UserEmail: userEmailFromContext(r.Context()),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.Calendar(props).Render(r.Context(), w); err != nil {
		h.log.Error("ui calendar render: %v", err)
	}
}
