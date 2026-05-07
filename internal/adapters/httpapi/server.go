package httpapi

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/jtprogru/jtpost/internal/logger"
)

//go:embed templates/index.html
var indexTemplate string

// Server HTTP сервер для API.
type Server struct {
	service   *core.PostService
	publisher core.Publisher
	authSvc   *core.AuthService  // nil if auth.type != token
	oauthSvc  *core.OAuthService // nil if no providers configured
	outbox    core.OutboxRepository
	mux       *http.ServeMux
	log       *logger.Logger
	cfg       *config.Config
}

// ServerConfig конфигурация HTTP сервера.
type ServerConfig struct {
	Service      *core.PostService
	Publisher    core.Publisher
	AuthService  *core.AuthService
	OAuthService *core.OAuthService
	Outbox       core.OutboxRepository
	Logger       *logger.Logger
	Config       *config.Config
}

// NewServer создаёт новый HTTP сервер.
func NewServer(service *core.PostService, publisher core.Publisher) *Server {
	return NewServerWithConfig(ServerConfig{
		Service:   service,
		Publisher: publisher,
		Logger:    logger.NewDefault(),
	})
}

// NewServerWithConfig создаёт HTTP сервер с конфигурацией.
func NewServerWithConfig(cfg ServerConfig) *Server {
	log := cfg.Logger
	if log == nil {
		log = logger.NewDefault()
	}

	s := &Server{
		service:   cfg.Service,
		publisher: cfg.Publisher,
		authSvc:   cfg.AuthService,
		oauthSvc:  cfg.OAuthService,
		outbox:    cfg.Outbox,
		mux:       http.NewServeMux(),
		log:       log,
		cfg:       cfg.Config,
	}
	s.registerRoutes()
	return s
}

// ServeHTTP реализует http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// Logger возвращает логгер сервера.
func (s *Server) Logger() *logger.Logger {
	return s.log
}

// apply оборачивает HandlerFunc в TenantFromConfigMiddleware (если конфиг задан).
func (s *Server) apply(h http.HandlerFunc) http.Handler {
	var handler http.Handler = h
	if s.cfg != nil {
		handler = TenantFromConfigMiddleware(s.cfg)(handler)
	}
	return handler
}

// registerRoutes регистрирует HTTP обработчики.
// Все API-routes регистрируются под двумя префиксами: legacy `/api/` и новый
// `/api/v1/` (F5). Aliases ведут на тот же handler — backward-compat.
func (s *Server) registerRoutes() {
	s.bothPrefixes("/api/posts", s.apply(s.handlePosts))
	s.bothPrefixes("/api/posts/", s.apply(s.handlePostByID))
	s.bothPrefixes("/api/stats", s.apply(s.handleStats))
	s.bothPrefixes("/api/plan", s.apply(s.handlePlan))
	s.bothPrefixes("/api/tags", s.apply(s.handleTags))
	if s.authSvc != nil && s.cfg != nil {
		s.bothPrefixesFunc("/api/auth/login", LoginHandler(s.authSvc, s.cfg))
		s.bothPrefixesFunc("/api/auth/logout", LogoutHandler(s.authSvc, s.cfg))
		s.bothPrefixesFunc("/api/auth/csrf", CSRFHandler(s.authSvc))
	}
	if s.oauthSvc != nil && s.authSvc != nil && s.cfg != nil {
		oauthHandler := NewOAuthHandler(s.oauthSvc, s.authSvc, s.cfg)
		s.bothPrefixes("/api/auth/oauth/", oauthHandler)
	}
	s.mux.HandleFunc("/", s.handleIndex)
}

// bothPrefixes регистрирует handler под legacy `/api/...` и новым
// `/api/v1/...` префиксами (F5).
func (s *Server) bothPrefixes(legacyPath string, h http.Handler) {
	s.mux.Handle(legacyPath, h)
	v1Path := "/api/v1" + strings.TrimPrefix(legacyPath, "/api")
	s.mux.Handle(v1Path, h)
}

func (s *Server) bothPrefixesFunc(legacyPath string, h http.HandlerFunc) {
	s.bothPrefixes(legacyPath, h)
}

// writeJSONError возвращает ошибку в JSON виде с заданным кодом.
func writeJSONError(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// handleIndex обрабатывает запросы на корень.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(indexTemplate))
}

// handlePosts обрабатывает GET/POST /api/posts.
func (s *Server) handlePosts(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		s.listPosts(w, r)
	case http.MethodPost:
		s.createPost(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// listPosts обрабатывает GET /api/posts.
func (s *Server) listPosts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	filter := core.PostFilter{}
	if t, ok := core.TenantFromContext(ctx); ok {
		filter.TenantID = t
	}

	statuses := r.URL.Query()["status"]
	for _, st := range statuses {
		filter.Statuses = append(filter.Statuses, core.PostStatus(st))
	}

	tags := r.URL.Query()["tag"]
	filter.Tags = tags

	search := r.URL.Query().Get("search")
	if search != "" {
		filter.Search = search
	}

	s.log.Debug("ListPosts filter: tenant=%s, statuses=%v, tags=%v, search=%s",
		filter.TenantID, filter.Statuses, filter.Tags, filter.Search)

	posts, err := s.service.ListPosts(ctx, filter)
	if err != nil {
		s.log.Error("ListPosts error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Если контекст принёс tenant — фильтруем посты по нему (mock-репозиторий
	// в тестах может не учитывать filter.TenantID).
	if filter.TenantID != uuid.Nil {
		filtered := posts[:0]
		for _, p := range posts {
			if p.TenantID == filter.TenantID || p.TenantID == uuid.Nil {
				filtered = append(filtered, p)
			}
		}
		posts = filtered
	}

	sortField := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")
	if sortField != "" {
		sortPosts(posts, sortField, sortOrder)
	}

	jsonPosts := make([]jsonPost, len(posts))
	for i, post := range posts {
		jsonPosts[i] = toJSONPost(post)
	}

	s.log.Debug("ListPosts returned %d posts", len(posts))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonPosts)
}

// sortPosts сортирует посты по указанному полю.
func sortPosts(posts []*core.Post, field, order string) {
	ascending := order != "desc"

	sortFunc := func(i, j int) bool {
		var less bool
		switch field {
		case "status":
			statusOrder := map[core.PostStatus]int{
				core.StatusIdea: 0, core.StatusDraft: 1, core.StatusReady: 2,
				core.StatusScheduled: 3, core.StatusPublished: 4,
			}
			less = statusOrder[posts[i].Status] < statusOrder[posts[j].Status]
		case "tags":
			less = strings.Join(posts[i].Tags, ",") < strings.Join(posts[j].Tags, ",")
		case "deadline":
			switch {
			case posts[i].Deadline == nil && posts[j].Deadline == nil:
				less = false
			case posts[i].Deadline == nil:
				less = false
			case posts[j].Deadline == nil:
				less = true
			default:
				less = posts[i].Deadline.Before(*posts[j].Deadline)
			}
		case "title":
			less = posts[i].Title < posts[j].Title
		default:
			less = true
		}

		if !ascending {
			return !less
		}
		return less
	}

	for range len(posts) - 1 {
		for j := range len(posts) - 1 {
			if !sortFunc(j, j+1) {
				posts[j], posts[j+1] = posts[j+1], posts[j]
			}
		}
	}
}

// createPostInput тело POST /api/posts.
type createPostInput struct {
	TenantID *string    `json:"tenant_id,omitempty"`
	AuthorID *string    `json:"author_id,omitempty"`
	Title    string     `json:"title"`
	Slug     string     `json:"slug,omitempty"`
	Tags     []string   `json:"tags,omitempty"`
	Excerpt  *string    `json:"excerpt,omitempty"`
	Deadline *time.Time `json:"deadline,omitempty"`
	Content  *string    `json:"content,omitempty"`
}

// createPost обрабатывает POST /api/posts.
func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if r.Body == nil {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}

	var input createPostInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.log.Warn("CreatePost decode error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctxTenant, _ := core.TenantFromContext(ctx)
	ctxAuthor, _ := core.AuthorFromContext(ctx)

	tenantID := ctxTenant
	if input.TenantID != nil && *input.TenantID != "" {
		parsed, err := uuid.Parse(*input.TenantID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_tenant_id")
			return
		}
		if ctxTenant != uuid.Nil && parsed != ctxTenant {
			writeJSONError(w, http.StatusForbidden, "tenant_mismatch")
			return
		}
		tenantID = parsed
	}

	authorID := ctxAuthor
	if input.AuthorID != nil && *input.AuthorID != "" {
		parsed, err := uuid.Parse(*input.AuthorID)
		if err != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_author_id")
			return
		}
		authorID = parsed
	}

	s.log.Info("CreatePost title=%q, tags=%v, tenant=%s", input.Title, input.Tags, tenantID)

	post, err := s.service.CreatePost(ctx, core.CreatePostInput{
		TenantID: tenantID,
		AuthorID: authorID,
		Title:    input.Title,
		Slug:     input.Slug,
		Tags:     input.Tags,
		Excerpt:  input.Excerpt,
	})
	if err != nil {
		s.log.Error("CreatePost error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if input.Content != nil || input.Deadline != nil {
		if input.Content != nil {
			post.Content = *input.Content
		}
		if input.Deadline != nil {
			post.Deadline = input.Deadline
		}
		if err := s.service.UpdatePost(ctx, post); err != nil {
			s.log.Error("CreatePost update content error: %v", err)
		}
	}

	s.log.Info("CreatePost success id=%s", post.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toJSONPost(post))
}

// handlePostByID обрабатывает GET/PATCH/DELETE /api/posts/{id}.
func (s *Server) handlePostByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/posts/")

	postID, ok := strings.CutSuffix(path, "/publish")
	if ok {
		parsedID, err := core.ParsePostID(postID)
		if err != nil {
			http.Error(w, "Invalid post ID format", http.StatusBadRequest)
			return
		}
		s.publishPost(w, r, parsedID)
		return
	}
	if queueID, ok := strings.CutSuffix(path, "/queue"); ok {
		parsedID, err := core.ParsePostID(queueID)
		if err != nil {
			http.Error(w, "Invalid post ID format", http.StatusBadRequest)
			return
		}
		s.queuePost(w, r, parsedID)
		return
	}

	id, err := core.ParsePostID(postID)
	if err != nil {
		http.Error(w, "Invalid post ID format", http.StatusBadRequest)
		return
	}

	if id == (core.PostID{}) {
		http.Error(w, "Post ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.getPost(w, r, id)
	case http.MethodPatch:
		s.updatePost(w, r, id)
	case http.MethodDelete:
		s.deletePost(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getPost возвращает пост по ID.
func (s *Server) getPost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	ctx := r.Context()

	s.log.Debug("GetPost id=%s", id)

	post, err := s.service.GetByID(ctx, id)
	if err != nil {
		s.log.Warn("GetPost error: %v, id=%s", err, id)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	if ctxTenant, ok := core.TenantFromContext(ctx); ok && post.TenantID != uuid.Nil && post.TenantID != ctxTenant {
		writeJSONError(w, http.StatusForbidden, "tenant_mismatch")
		return
	}

	s.log.Debug("GetPost success id=%s", id)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toJSONPost(post))
}

// updatePost обновляет пост.
func (s *Server) updatePost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	ctx := r.Context()

	s.log.Debug("UpdatePost id=%s", id)

	var input struct {
		TenantID  *string    `json:"tenant_id,omitempty"`
		Title     *string    `json:"title,omitempty"`
		Status    *string    `json:"status,omitempty"`
		Tags      []string   `json:"tags,omitempty"`
		Deadline  *time.Time `json:"deadline,omitempty"`
		Scheduled *time.Time `json:"scheduled_at,omitempty"`
		Content   *string    `json:"content,omitempty"`
		Excerpt   *string    `json:"excerpt,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		s.log.Warn("UpdatePost decode error: %v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	post, err := s.service.GetByID(ctx, id)
	if err != nil {
		s.log.Warn("UpdatePost not found: %v", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// REQ-8.5: tenant_id immutability.
	if input.TenantID != nil && *input.TenantID != "" {
		parsed, perr := uuid.Parse(*input.TenantID)
		if perr != nil {
			writeJSONError(w, http.StatusBadRequest, "invalid_tenant_id")
			return
		}
		if parsed != post.TenantID {
			writeJSONError(w, http.StatusBadRequest, "tenant_id_immutable")
			return
		}
	}

	// REQ-8.4: tenant scope check.
	if ctxTenant, ok := core.TenantFromContext(ctx); ok && post.TenantID != uuid.Nil && post.TenantID != ctxTenant {
		writeJSONError(w, http.StatusForbidden, "tenant_mismatch")
		return
	}

	if input.Title != nil {
		post.Title = *input.Title
	}
	if input.Status != nil {
		newStatus := core.PostStatus(*input.Status)
		s.log.Info("UpdatePost status id=%s, new=%s", id, newStatus)
		updatedPost, err := s.service.UpdateStatus(ctx, id, newStatus)
		if err != nil {
			s.log.Error("UpdatePost status error: %v", err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		post = updatedPost
	}
	if input.Tags != nil {
		post.Tags = input.Tags
	}
	if input.Deadline != nil {
		post.Deadline = input.Deadline
	}
	if input.Scheduled != nil {
		post.ScheduledAt = input.Scheduled
	}
	if input.Content != nil {
		post.Content = *input.Content
	}
	if input.Excerpt != nil {
		post.Excerpt = input.Excerpt
	}

	if err := s.service.UpdatePost(ctx, post); err != nil {
		s.log.Error("UpdatePost error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.log.Info("UpdatePost success id=%s", id)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toJSONPost(post))
}

// deletePost удаляет пост.
func (s *Server) deletePost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	ctx := r.Context()

	s.log.Info("DeletePost id=%s", id)

	if err := s.service.DeletePost(ctx, id); err != nil {
		s.log.Error("DeletePost error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.log.Info("DeletePost success id=%s", id)
	w.WriteHeader(http.StatusNoContent)
}

// handleStats обрабатывает GET /api/stats.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	tenantID, _ := core.TenantFromContext(ctx)

	s.log.Debug("HandleStats request tenant=%s", tenantID)

	stats, err := s.service.GetStats(ctx, tenantID)
	if err != nil {
		s.log.Error("HandleStats error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	s.log.Debug("HandleStats success")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(stats)
}

// handlePlan обрабатывает GET /api/plan.
func (s *Server) handlePlan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	daysStr := r.URL.Query().Get("days")
	days := 30
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	s.log.Debug("HandlePlan days=%d", days)

	filter := core.PostFilter{}
	if t, ok := core.TenantFromContext(ctx); ok {
		filter.TenantID = t
	}

	posts, err := s.service.ListPosts(ctx, filter)
	if err != nil {
		s.log.Error("HandlePlan error: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	now := time.Now()
	deadline := now.AddDate(0, 0, days)

	var planned []jsonPlannedPost
	for _, post := range posts {
		if post.Status == core.StatusPublished {
			continue
		}

		var date *time.Time
		var dateType string

		if post.ScheduledAt != nil {
			date = post.ScheduledAt
			dateType = "schedule"
		} else if post.Deadline != nil {
			date = post.Deadline
			dateType = "deadline"
		}

		if date != nil && !date.After(deadline) {
			planned = append(planned, toJSONPlannedPost(post, *date, dateType))
		}
	}

	sortPlannedByDate(planned)

	s.log.Debug("HandlePlan returned %d planned posts", len(planned))
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(planned)
}

// sortPlannedByDate сортирует запланированные посты по дате.
func sortPlannedByDate(posts []jsonPlannedPost) {
	for range posts {
		for j := 1; j < len(posts); j++ {
			if posts[j].Date.Before(posts[j-1].Date) {
				posts[j], posts[j-1] = posts[j-1], posts[j]
			}
		}
	}
}

// jsonPost JSON-представление поста.
type jsonPost struct {
	ID             string                `json:"id"`
	TenantID       string                `json:"tenant_id"`
	AuthorID       string                `json:"author_id"`
	Title          string                `json:"title"`
	Slug           string                `json:"slug"`
	Status         string                `json:"status"`
	CreatedAt      time.Time             `json:"created_at"`
	UpdatedAt      time.Time             `json:"updated_at"`
	Revision       int                   `json:"revision"`
	Tags           []string              `json:"tags,omitempty"`
	Deadline       *time.Time            `json:"deadline,omitempty"`
	ScheduledAt    *time.Time            `json:"scheduled_at,omitempty"`
	PublishedAt    *time.Time            `json:"published_at,omitempty"`
	Excerpt        *string               `json:"excerpt,omitempty"`
	CoverImage     *core.Attachment      `json:"cover_image,omitempty"`
	Attachments    []core.Attachment     `json:"attachments,omitempty"`
	PublishHistory []core.PublishAttempt `json:"publish_history,omitempty"`
	RevisionSHA    *string               `json:"revision_sha,omitempty"`
	Content        string                `json:"content"`
	External       ExternalLinks         `json:"external"`
}

// ExternalLinks ссылки на опубликованные посты.
type ExternalLinks struct {
	TelegramURL string `json:"telegram_url,omitempty"`
}

// toJSONPost конвертирует Post в jsonPost.
func toJSONPost(post *core.Post) jsonPost {
	return jsonPost{
		ID:             post.ID.String(),
		TenantID:       post.TenantID.String(),
		AuthorID:       post.AuthorID.String(),
		Title:          post.Title,
		Slug:           post.Slug,
		Status:         string(post.Status),
		CreatedAt:      post.CreatedAt,
		UpdatedAt:      post.UpdatedAt,
		Revision:       post.Revision,
		Tags:           post.Tags,
		Deadline:       post.Deadline,
		ScheduledAt:    post.ScheduledAt,
		PublishedAt:    post.PublishedAt,
		Excerpt:        post.Excerpt,
		CoverImage:     post.CoverImage,
		Attachments:    post.Attachments,
		PublishHistory: post.PublishHistory,
		RevisionSHA:    post.RevisionSHA,
		Content:        post.Content,
		External: ExternalLinks{
			TelegramURL: post.External.TelegramURL,
		},
	}
}

// publishPost обрабатывает POST /api/posts/{id}/publish.
func (s *Server) publishPost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	s.log.Info("PublishPost id=%s", id)

	if s.publisher == nil {
		http.Error(w, "publisher not configured", http.StatusBadRequest)
		return
	}

	post, err := s.service.PublishPost(ctx, id, core.PublishPostInput{}, s.publisher)
	if err != nil {
		s.log.Error("PublishPost error: %v, id=%s", err, id)

		switch {
		case errors.Is(err, core.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, core.ErrValidation):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidStatus):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	s.log.Info("PublishPost success id=%s", id)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toJSONPost(post))
}

// queuePost обрабатывает POST /api/posts/{id}/queue — кладёт пост в outbox.
func (s *Server) queuePost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if s.outbox == nil {
		http.Error(w, "outbox not configured", http.StatusServiceUnavailable)
		return
	}
	ctx := r.Context()
	post, err := s.service.GetByID(ctx, id)
	if err != nil {
		if errors.Is(err, core.ErrNotFound) {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	entry, err := core.EnqueueForPublish(ctx, s.outbox, post, time.Time{})
	if err != nil {
		s.log.Error("queuePost enqueue: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.log.Info("queuePost success post=%s entry=%s", id, entry.ID)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"entry_id":        entry.ID,
		"post_id":         entry.PostID,
		"status":          entry.Status,
		"next_attempt_at": entry.NextAttemptAt,
	})
}

// jsonPlannedPost JSON-представление запланированного поста.
type jsonPlannedPost struct {
	ID       string    `json:"id"`
	Title    string    `json:"title"`
	Slug     string    `json:"slug"`
	Status   string    `json:"status"`
	Date     time.Time `json:"date"`
	DateType string    `json:"date_type"`
}

// toJSONPlannedPost конвертирует Post в jsonPlannedPost.
func toJSONPlannedPost(post *core.Post, date time.Time, dateType string) jsonPlannedPost {
	return jsonPlannedPost{
		ID:       post.ID.String(),
		Title:    post.Title,
		Slug:     post.Slug,
		Status:   string(post.Status),
		Date:     date,
		DateType: dateType,
	}
}

// handleTags обрабатывает GET /api/tags.
func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	filter := core.PostFilter{}
	if t, ok := core.TenantFromContext(ctx); ok {
		filter.TenantID = t
	}

	posts, err := s.service.ListPosts(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tagSet := make(map[string]bool)
	for _, post := range posts {
		for _, tag := range post.Tags {
			tagSet[tag] = true
		}
	}

	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	for i := range len(tags) - 1 {
		for j := i + 1; j < len(tags); j++ {
			if tags[i] > tags[j] {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string][]string{"tags": tags})
}
