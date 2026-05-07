package webui

import (
	"errors"
	"net/http"

	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
)

// handlePostHistory — GET /ui/posts/{id}/history.
// Если storage не реализует HistoryProvider, рендерится stub-страница с
// сообщением (не 503), чтобы UX был связным даже на fs/sqlite/postgres.
func (h *Handler) handlePostHistory(w http.ResponseWriter, r *http.Request, id core.PostID) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	post, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.log.Error("ui history get: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	props := components.HistoryProps{
		Post:      post,
		UserEmail: userEmailFromContext(r.Context()),
	}
	if h.history != nil {
		entries, herr := h.history.History(r.Context(), post, 0)
		if herr != nil {
			h.log.Error("ui history fetch: %v", herr)
			http.Error(w, "не удалось получить историю: "+herr.Error(), http.StatusInternalServerError)
			return
		}
		props.Available = true
		props.Entries = entries
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.History(props).Render(r.Context(), w); err != nil {
		h.log.Error("ui history render: %v", err)
	}
}
