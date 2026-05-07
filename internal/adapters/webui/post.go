package webui

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
)

// handlePostByID — GET → render edit page; POST → save.
// Также роутит /ui/posts/{id}/delete и /ui/posts/new.
func (h *Handler) handlePostByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/ui/posts/")
	if rest == "" {
		http.NotFound(w, r)
		return
	}
	if rest == "new" {
		h.handlePostNew(w, r)
		return
	}
	// /ui/posts/{id}/delete
	if id, ok := strings.CutSuffix(rest, "/delete"); ok {
		parsed, err := core.ParsePostID(id)
		if err != nil {
			http.Error(w, "invalid post id", http.StatusBadRequest)
			return
		}
		h.deletePost(w, r, parsed)
		return
	}
	if strings.Contains(rest, "/") {
		http.NotFound(w, r)
		return
	}
	id, err := core.ParsePostID(rest)
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

// handlePostNew — GET render form; POST create + redirect на edit page.
func (h *Handler) handlePostNew(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderPostNew(w, r, components.PostNewProps{UserEmail: userEmailFromContext(r.Context())})
	case http.MethodPost:
		h.createPost(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) renderPostNew(w http.ResponseWriter, r *http.Request, p components.PostNewProps) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if p.Error != "" {
		w.WriteHeader(http.StatusUnprocessableEntity)
	}
	if err := components.PostNew(p).Render(r.Context(), w); err != nil {
		h.log.Error("ui post new render: %v", err)
	}
}

func (h *Handler) createPost(w http.ResponseWriter, r *http.Request) {
	if h.cfg == nil {
		http.Error(w, "config not loaded", http.StatusServiceUnavailable)
		return
	}
	if err := r.ParseForm(); err != nil {
		h.renderPostNew(w, r, components.PostNewProps{Error: "invalid form"})
		return
	}
	title := strings.TrimSpace(r.PostForm.Get("title"))
	slug := strings.TrimSpace(r.PostForm.Get("slug"))
	tagsRaw := r.PostForm.Get("tags")
	if title == "" {
		h.renderPostNew(w, r, components.PostNewProps{
			Tags:      tagsRaw,
			Slug:      slug,
			UserEmail: userEmailFromContext(r.Context()),
			Error:     "title обязателен",
		})
		return
	}

	// TenantID/AuthorID — из ctx (auth) или дефолтов в cfg.
	tenantID := h.cfg.Auth.TenantDefault
	authorID := h.cfg.Auth.AuthorDefault
	if t, ok := core.TenantFromContext(r.Context()); ok && t != (uuid.UUID{}) {
		tenantID = t
	}
	if u, ok := core.UserFromContext(r.Context()); ok && u != nil {
		authorID = u.ID
	}

	var tags []string
	if tagsRaw != "" {
		for p := range strings.SplitSeq(tagsRaw, ",") {
			if t := strings.TrimSpace(p); t != "" {
				tags = append(tags, t)
			}
		}
	}

	post, err := h.service.CreatePost(r.Context(), core.CreatePostInput{
		TenantID: tenantID,
		AuthorID: authorID,
		Title:    title,
		Slug:     slug,
		Tags:     tags,
	})
	if err != nil {
		h.renderPostNew(w, r, components.PostNewProps{
			Title:     title,
			Tags:      tagsRaw,
			Slug:      slug,
			UserEmail: userEmailFromContext(r.Context()),
			Error:     "не удалось создать: " + err.Error(),
		})
		return
	}
	_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
		Action:       core.AuditPostCreated,
		Outcome:      core.AuditOutcomeSuccess,
		ResourceType: "post",
		ResourceID:   post.ID.String(),
		TenantID:     post.TenantID,
		Metadata:     map[string]any{"via": "webui"},
	})
	h.publish("post.created", map[string]any{"id": post.ID.String(), "title": post.Title})
	http.Redirect(w, r, "/ui/posts/"+post.ID.String()+"?saved=1", http.StatusSeeOther)
}

func (h *Handler) deletePost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	post, err := h.service.GetByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.NotFound(w, r)
			return
		}
		h.log.Error("ui delete get: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	if err := h.service.DeletePost(r.Context(), id); err != nil {
		h.log.Error("ui delete: %v", err)
		http.Error(w, "delete failed: "+err.Error(), http.StatusInternalServerError)
		return
	}
	_ = h.auditSvc.Log(r.Context(), core.AuditEntry{
		Action:       core.AuditPostDeleted,
		Outcome:      core.AuditOutcomeSuccess,
		ResourceType: "post",
		ResourceID:   id.String(),
		TenantID:     post.TenantID,
		Metadata:     map[string]any{"via": "webui", "title": post.Title},
	})
	h.publish("post.deleted", map[string]any{"id": id.String()})
	http.Redirect(w, r, "/ui/", http.StatusSeeOther)
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
	h.publish("post.updated", map[string]any{"id": id.String(), "status": string(post.Status)})
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
