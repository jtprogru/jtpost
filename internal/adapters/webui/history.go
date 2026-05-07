package webui

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
	"gopkg.in/yaml.v3"
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

// handlePostRevert — POST /ui/posts/{id}/history/{hash}/revert.
// Откатывает Title/Content/Tags поста к содержимому файла на указанной ревизии.
// Lifecycle-поля (Status, ScheduledAt, Deadline, PublishedAt, External) не трогаются.
func (h *Handler) handlePostRevert(w http.ResponseWriter, r *http.Request, id core.PostID, hash string) {
	if r.Method != http.MethodPost {
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
		h.log.Error("ui revert get: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	body, ferr := h.history.FileAtCommit(r.Context(), post, hash)
	if ferr != nil {
		if errors.Is(ferr, core.ErrNotFound) {
			http.Error(w, "ревизия не найдена", http.StatusNotFound)
			return
		}
		h.log.Error("ui revert fetch: %v", ferr)
		http.Error(w, "не удалось получить ревизию: "+ferr.Error(), http.StatusInternalServerError)
		return
	}
	title, content, tags, perr := parseRevertSnapshot(body)
	if perr != nil {
		http.Error(w, "не удалось распарсить ревизию: "+perr.Error(), http.StatusUnprocessableEntity)
		return
	}
	post.Title = title
	post.Content = content
	post.Tags = tags
	if err := h.service.UpdatePost(r.Context(), post); err != nil {
		h.log.Error("ui revert update: %v", err)
		http.Error(w, "ошибка сохранения: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if h.auditSvc != nil {
		_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
			Action:       core.AuditPostReverted,
			Outcome:      core.AuditOutcomeSuccess,
			ResourceType: "post",
			ResourceID:   id.String(),
			TenantID:     post.TenantID,
			Metadata:     map[string]any{"via": "webui", "hash": hash},
		})
	}
	h.publish("post.updated", map[string]any{"id": id.String(), "status": string(post.Status)})
	http.Redirect(w, r, "/ui/posts/"+id.String()+"?reverted=1", http.StatusSeeOther)
}

// parseRevertSnapshot извлекает Title/Content/Tags из raw bytes ревизионного
// файла поста (markdown с YAML frontmatter). Используется только для revert —
// lifecycle-поля игнорируются намеренно.
func parseRevertSnapshot(data []byte) (title, content string, tags []string, err error) {
	trimmed := bytes.TrimPrefix(data, []byte("---\n"))
	parts := bytes.SplitN(trimmed, []byte("\n---\n"), 2)
	if len(parts) < 2 {
		return "", "", nil, errors.New("invalid frontmatter format")
	}
	var fm struct {
		Title string   `yaml:"title"`
		Tags  []string `yaml:"tags"`
	}
	if err := yaml.Unmarshal(parts[0], &fm); err != nil {
		return "", "", nil, fmt.Errorf("yaml: %w", err)
	}
	if strings.TrimSpace(fm.Title) == "" {
		return "", "", nil, errors.New("missing title in revision")
	}
	return fm.Title, strings.TrimSpace(string(parts[1])), fm.Tags, nil
}
