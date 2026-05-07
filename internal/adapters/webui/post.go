package webui

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
)

// handlePostByID — GET → render edit page; POST → save.
func (h *Handler) handlePostByID(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/ui/posts/")
	if idStr == "" || strings.Contains(idStr, "/") {
		http.NotFound(w, r)
		return
	}
	id, err := core.ParsePostID(idStr)
	if err != nil {
		http.Error(w, "invalid post id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.renderPostEdit(w, r, id, "", r.URL.Query().Get("saved") == "1")
	case http.MethodPost:
		h.savePost(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) renderPostEdit(w http.ResponseWriter, r *http.Request, id core.PostID, errMsg string, saved bool) {
	post, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.log.Error("ui post get: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if errMsg != "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	props := components.PostEditProps{
		Post:      post,
		UserEmail: userEmailFromContext(r.Context()),
		Saved:     saved,
		Error:     errMsg,
	}
	if err := components.PostEdit(props).Render(r.Context(), w); err != nil {
		h.log.Error("ui post edit render: %v", err)
	}
}

func (h *Handler) savePost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	if err := r.ParseForm(); err != nil {
		h.renderPostEdit(w, r, id, "invalid form", false)
		return
	}
	post, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}

	title := strings.TrimSpace(r.PostForm.Get("title"))
	if title == "" {
		h.renderPostEdit(w, r, id, "title не может быть пустым", false)
		return
	}
	post.Title = title
	post.Content = r.PostForm.Get("content")

	tagsRaw := r.PostForm.Get("tags")
	if tagsRaw == "" {
		post.Tags = nil
	} else {
		parts := strings.Split(tagsRaw, ",")
		tags := parts[:0]
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				tags = append(tags, t)
			}
		}
		post.Tags = tags
	}

	statusRaw := r.PostForm.Get("status")
	newStatus := core.PostStatus(statusRaw)
	if newStatus != post.Status {
		updated, sErr := h.service.UpdateStatus(r.Context(), id, newStatus)
		if sErr != nil {
			h.renderPostEdit(w, r, id, "не удалось сменить статус: "+sErr.Error(), false)
			return
		}
		post = updated
		post.Title = title
		post.Content = r.PostForm.Get("content")
	}

	if err := h.service.UpdatePost(r.Context(), post); err != nil {
		h.log.Error("ui post update: %v", err)
		h.renderPostEdit(w, r, id, "ошибка сохранения: "+err.Error(), false)
		return
	}
	_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
		Action:       core.AuditPostUpdated,
		Outcome:      core.AuditOutcomeSuccess,
		ResourceType: "post",
		ResourceID:   id.String(),
		TenantID:     post.TenantID,
		Metadata:     map[string]any{"via": "webui"},
	})
	// PRG: redirect на тот же URL с ?saved=1 (избегаем re-submit при F5).
	http.Redirect(w, r, "/ui/posts/"+id.String()+"?saved=1", http.StatusSeeOther)
}

// handlePreview — POST /ui/preview: принимает form-field "content" и
// возвращает HTML-rendering. Используется htmx live-preview.
func (h *Handler) handlePreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.RenderedMarkdown(r.PostForm.Get("content")).Render(r.Context(), w); err != nil {
		h.log.Error("ui preview render: %v", err)
	}
}
