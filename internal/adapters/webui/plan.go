package webui

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
)

// handlePlan — GET /ui/plan?days=30. Группирует посты по дате
// (schedule_at приоритетнее deadline). Published посты не показываются.
func (h *Handler) handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err := strconv.Atoi(d); err == nil && n > 0 && n <= 365 {
			days = n
		}
	}
	groups, err := h.collectPlanGroups(r.Context(), days)
	if err != nil {
		h.log.Error("ui plan: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	props := components.PlanProps{
		Groups:    groups,
		Days:      days,
		UserEmail: userEmailFromContext(r.Context()),
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.Plan(props).Render(r.Context(), w); err != nil {
		h.log.Error("ui plan render: %v", err)
	}
}

func (h *Handler) collectPlanGroups(ctx context.Context, days int) ([]components.PlanGroup, error) {
	filter := core.PostFilter{}
	if t, ok := core.TenantFromContext(ctx); ok {
		filter.TenantID = t
	}
	posts, err := h.service.ListPosts(ctx, filter)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	cutoff := now.AddDate(0, 0, days)

	type bucketKey struct{ y, m, d int }
	buckets := map[bucketKey]*components.PlanGroup{}

	for _, post := range posts {
		if post.Status == core.StatusPublished {
			continue
		}
		var date *time.Time
		var dateType string
		switch {
		case post.ScheduledAt != nil:
			date = post.ScheduledAt
			dateType = "schedule"
		case post.Deadline != nil:
			date = post.Deadline
			dateType = "deadline"
		default:
			continue
		}
		if date.After(cutoff) {
			continue
		}
		k := bucketKey{date.Year(), int(date.Month()), date.Day()}
		g, ok := buckets[k]
		if !ok {
			g = &components.PlanGroup{Date: time.Date(k.y, time.Month(k.m), k.d, 0, 0, 0, 0, date.Location())}
			buckets[k] = g
		}
		g.Items = append(g.Items, components.PlanItem{Post: post, Date: *date, DateType: dateType})
	}

	out := make([]components.PlanGroup, 0, len(buckets))
	for _, g := range buckets {
		// Внутри группы — по времени.
		sort.Slice(g.Items, func(i, j int) bool { return g.Items[i].Date.Before(g.Items[j].Date) })
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out, nil
}
