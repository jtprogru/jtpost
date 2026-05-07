// Package webui — server-side rendered Web UI v2 на htmx + templ.
//
// Routes (mount под /ui/):
//   - GET /ui/                — dashboard (full page)
//   - GET /ui/posts           — htmx-partial: только tbody таблицы постов
//   - GET /ui/static/{file}   — embedded static (htmx.min.js, app.css)
package webui

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/jtprogru/jtpost/internal/adapters/webui/components"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/jtprogru/jtpost/internal/logger"
)

//go:embed static/*
var staticFS embed.FS

// Handler — UI handler. Обслуживает /ui/* path-prefix.
type Handler struct {
	service *core.PostService
	log     *logger.Logger
	mux     *http.ServeMux
}

// NewHandler создаёт UI handler с готовой подсетью routes.
func NewHandler(service *core.PostService, log *logger.Logger) *Handler {
	if log == nil {
		log = logger.NewDefault()
	}
	h := &Handler{service: service, log: log, mux: http.NewServeMux()}
	h.registerRoutes()
	return h
}

// ServeHTTP — единый entry-point под /ui/.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) registerRoutes() {
	h.mux.HandleFunc("/ui/", h.handleIndex)
	h.mux.HandleFunc("/ui/posts", h.handlePostsPartial)

	staticSub, _ := fs.Sub(staticFS, "static")
	h.mux.Handle("/ui/static/", http.StripPrefix("/ui/static/", http.FileServerFS(staticSub)))
}

func (h *Handler) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/ui/" && r.URL.Path != "/ui" {
		http.NotFound(w, r)
		return
	}
	props := components.DashboardProps{
		Search: r.URL.Query().Get("search"),
		Status: r.URL.Query().Get("status"),
	}
	posts, err := h.listPosts(r.Context(), props.Search, props.Status)
	if err != nil {
		h.log.Error("ui index list: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	props.Posts = posts
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.Dashboard(props).Render(r.Context(), w); err != nil {
		h.log.Error("ui render dashboard: %v", err)
	}
}

// handlePostsPartial — htmx-partial: только tbody, для in-place обновлений.
func (h *Handler) handlePostsPartial(w http.ResponseWriter, r *http.Request) {
	search := r.URL.Query().Get("search")
	status := r.URL.Query().Get("status")
	posts, err := h.listPosts(r.Context(), search, status)
	if err != nil {
		h.log.Error("ui partial list: %v", err)
		http.Error(w, "internal", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := components.PostsTable(posts).Render(r.Context(), w); err != nil {
		h.log.Error("ui render partial: %v", err)
	}
}

func (h *Handler) listPosts(ctx context.Context, search, status string) ([]*core.Post, error) {
	filter := core.PostFilter{}
	if t, ok := core.TenantFromContext(ctx); ok {
		filter.TenantID = t
	}
	if search != "" {
		filter.Search = search
	}
	if status != "" {
		filter.Statuses = []core.PostStatus{core.PostStatus(status)}
	}
	posts, err := h.service.ListPosts(ctx, filter)
	if err != nil {
		return nil, err
	}
	// Defensive: если репо не учитывает Statuses (mock/fs), фильтруем здесь.
	if status != "" {
		filtered := posts[:0]
		for _, p := range posts {
			if string(p.Status) == status {
				filtered = append(filtered, p)
			}
		}
		posts = filtered
	}
	if search != "" {
		needle := strings.ToLower(search)
		filtered := posts[:0]
		for _, p := range posts {
			if strings.Contains(strings.ToLower(p.Title), needle) || strings.Contains(strings.ToLower(p.Slug), needle) {
				filtered = append(filtered, p)
			}
		}
		posts = filtered
	}
	return posts, nil
}
