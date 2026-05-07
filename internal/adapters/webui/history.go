package webui

import (
	"errors"
	"net/http"
	"strings"

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

// handlePostRevision — GET /ui/posts/{id}/history/{hash}.
// Рендерит сырое содержимое файла поста на указанной ревизии.
func (h *Handler) handlePostRevision(w http.ResponseWriter, r *http.Request, id core.PostID, hash string) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.history == nil {
		http.Error(w, "history not available for this storage", http.StatusServiceUnavailable)
		return
	}
	post, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.log.Error("ui revision get: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	props := components.RevisionProps{
		Post:      post,
		Hash:      hash,
		ShortHash: hash,
		UserEmail: userEmailFromContext(r.Context()),
	}
	if len(hash) > 8 {
		props.ShortHash = hash[:8]
	}
	body, ferr := h.history.FileAtCommit(r.Context(), post, hash)
	switch {
	case ferr == nil:
		props.Content = string(body)
	case errors.Is(ferr, core.ErrNotFound):
		props.NotFound = true
	default:
		h.log.Error("ui revision fetch: %v", ferr)
		props.ErrMessage = "не удалось получить ревизию: " + ferr.Error()
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	switch {
	case props.NotFound:
		w.WriteHeader(http.StatusNotFound)
	case props.ErrMessage != "":
		w.WriteHeader(http.StatusInternalServerError)
	}
	if err := components.Revision(props).Render(r.Context(), w); err != nil {
		h.log.Error("ui revision render: %v", err)
	}
}

// extractRevisionHash проверяет суффикс пути `/history/<hash>` и возвращает
// hash. Возвращает "", false если pattern не совпал.
func extractRevisionHash(rest string) (string, bool) {
	_, hash, ok := strings.Cut(rest, "/history/")
	if !ok || hash == "" || strings.Contains(hash, "/") {
		return "", false
	}
	return hash, true
}
