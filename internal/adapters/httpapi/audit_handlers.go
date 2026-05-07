package httpapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

// AuditHandler возвращает GET /api/v1/audit. Требует role=owner.
// Filters: ?action=&actor=&limit= (default 100, max 500). Tenant scope —
// автоматический для не-owner (на будущее; пока owner-only).
func AuditHandler(repo core.AuditRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		role, _ := core.RoleFromContext(r.Context())
		if role != core.RoleOwner {
			writeJSONError(w, http.StatusForbidden, "forbidden")
			return
		}
		q := r.URL.Query()
		filter := core.AuditFilter{
			Action: core.AuditAction(q.Get("action")),
			Limit:  100,
		}
		if l := q.Get("limit"); l != "" {
			if n, err := strconv.Atoi(l); err == nil && n > 0 {
				if n > 500 {
					n = 500
				}
				filter.Limit = n
			}
		}
		if a := q.Get("actor"); a != "" {
			if id, err := uuid.Parse(a); err == nil {
				filter.ActorID = id
			}
		}
		if t := q.Get("tenant"); t != "" {
			if id, err := uuid.Parse(t); err == nil {
				filter.TenantID = id
			}
		}
		entries, err := repo.List(r.Context(), filter)
		if err != nil {
			writeJSONError(w, http.StatusInternalServerError, "audit_list_failed")
			return
		}
		out := make([]jsonAuditEntry, len(entries))
		for i, e := range entries {
			out[i] = toJSONAuditEntry(e)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"entries": out, "count": len(out)})
	}
}

type jsonAuditEntry struct {
	ID           string         `json:"id"`
	OccurredAt   time.Time      `json:"occurred_at"`
	TenantID     string         `json:"tenant_id,omitempty"`
	ActorID      string         `json:"actor_id,omitempty"`
	ActorType    string         `json:"actor_type,omitempty"`
	Action       string         `json:"action"`
	ResourceType string         `json:"resource_type,omitempty"`
	ResourceID   string         `json:"resource_id,omitempty"`
	Outcome      string         `json:"outcome"`
	IP           string         `json:"ip,omitempty"`
	UserAgent    string         `json:"user_agent,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
}

func toJSONAuditEntry(e *core.AuditEntry) jsonAuditEntry {
	out := jsonAuditEntry{
		ID:           e.ID.String(),
		OccurredAt:   e.OccurredAt,
		ActorType:    string(e.ActorType),
		Action:       string(e.Action),
		ResourceType: e.ResourceType,
		ResourceID:   e.ResourceID,
		Outcome:      string(e.Outcome),
		IP:           e.IP,
		UserAgent:    e.UserAgent,
		Metadata:     e.Metadata,
	}
	if e.TenantID != uuid.Nil {
		out.TenantID = e.TenantID.String()
	}
	if e.ActorID != uuid.Nil {
		out.ActorID = e.ActorID.String()
	}
	return out
}
