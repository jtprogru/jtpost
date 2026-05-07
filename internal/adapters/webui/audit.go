package webui

import (
	"net/http"

	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
)

// handleAudit — GET /ui/audit. Owner-only. Список с фильтром по action.
func (h *Handler) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.auditRepo == nil {
		http.Error(w, "audit storage не настроен (требуется sqlite/postgres)", http.StatusServiceUnavailable)
		return
	}
	role, _ := core.RoleFromContext(r.Context())
	// Если auth.type=token и role не owner — 403. При auth-disabled — pass-through.
	if h.cfg != nil && h.cfg.Auth.Type == "token" && role != core.RoleOwner {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	filter := core.AuditFilter{Limit: 200}
	action := r.URL.Query().Get("action")
	if action != "" {
		filter.Action = core.AuditAction(action)
	}

	entries, err := h.auditRepo.List(r.Context(), filter)
	if err != nil {
		h.log.Error("ui audit list: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.Audit(components.AuditProps{
		Entries:      entries,
		FilterAction: action,
		UserEmail:    userEmailFromContext(r.Context()),
	}).Render(r.Context(), w); err != nil {
		h.log.Error("ui audit render: %v", err)
	}
}
