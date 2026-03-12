package httpapi

import (
	_ "embed"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

//go:embed templates/index.html
var indexTemplate string

// Server HTTP сервер для API.
type Server struct {
	service    *core.PostService
	publishers map[core.Platform]core.Publisher
	mux        *http.ServeMux
}

// NewServer создаёт новый HTTP сервер.
func NewServer(service *core.PostService, publishers map[core.Platform]core.Publisher) *Server {
	s := &Server{
		service:    service,
		publishers: publishers,
		mux:        http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// ServeHTTP реализует http.Handler.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

// registerRoutes регистрирует HTTP обработчики.
func (s *Server) registerRoutes() {
	s.mux.HandleFunc("/api/posts", s.handlePosts)
	s.mux.HandleFunc("/api/posts/", s.handlePostByID)
	s.mux.HandleFunc("/api/stats", s.handleStats)
	s.mux.HandleFunc("/api/plan", s.handlePlan)
	s.mux.HandleFunc("/api/platforms", s.handlePlatforms)
	s.mux.HandleFunc("/api/tags", s.handleTags)
	s.mux.HandleFunc("/", s.handleIndex)
}

// handleIndex обрабатывает запросы на корень.
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Отдаём HTML страницу из шаблона
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

	// Парсим фильтры из query параметров
	filter := core.PostFilter{}

	statuses := r.URL.Query()["status"]
	for _, st := range statuses {
		filter.Statuses = append(filter.Statuses, core.PostStatus(st))
	}

	platforms := r.URL.Query()["platform"]
	for _, pl := range platforms {
		filter.Platforms = append(filter.Platforms, core.Platform(pl))
	}

	tags := r.URL.Query()["tag"]
	filter.Tags = tags

	search := r.URL.Query().Get("search")
	if search != "" {
		filter.Search = search
	}

	posts, err := s.service.ListPosts(ctx, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Сортировка
	sortField := r.URL.Query().Get("sort")
	sortOrder := r.URL.Query().Get("order")
	if sortField != "" {
		sortPosts(posts, sortField, sortOrder)
	}

	// Конвертируем в JSON-совместимый формат
	jsonPosts := make([]jsonPost, len(posts))
	for i, post := range posts {
		jsonPosts[i] = toJSONPost(post)
	}

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
		case "platforms":
			less = strings.Join(platformsToString(posts[i].Platforms), ",") <
				strings.Join(platformsToString(posts[j].Platforms), ",")
		case "tags":
			less = strings.Join(posts[i].Tags, ",") < strings.Join(posts[j].Tags, ",")
		case "deadline":
			if posts[i].Deadline == nil && posts[j].Deadline == nil {
				less = false
			} else if posts[i].Deadline == nil {
				less = false
			} else if posts[j].Deadline == nil {
				less = true
			} else {
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

	// Пузырьковая сортировка
	for i := 0; i < len(posts)-1; i++ {
		for j := 0; j < len(posts)-i-1; j++ {
			if !sortFunc(j, j+1) {
				posts[j], posts[j+1] = posts[j+1], posts[j]
			}
		}
	}
}

// platformsToString конвертирует []Platform в []string.
func platformsToString(platforms []core.Platform) []string {
	result := make([]string, len(platforms))
	for i, p := range platforms {
		result[i] = string(p)
	}
	return result
}

// createPost обрабатывает POST /api/posts.
func (s *Server) createPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var input struct {
		Title     string   `json:"title"`
		Slug      string   `json:"slug,omitempty"`
		Platforms []string `json:"platforms,omitempty"`
		Tags      []string `json:"tags,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Конвертируем платформы
	platforms := make([]core.Platform, len(input.Platforms))
	for i, p := range input.Platforms {
		platforms[i] = core.Platform(p)
	}

	post, err := s.service.CreatePost(ctx, core.CreatePostInput{
		Title:     input.Title,
		Slug:      input.Slug,
		Platforms: platforms,
		Tags:      input.Tags,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(toJSONPost(post))
}

// handlePostByID обрабатывает GET/PATCH/DELETE /api/posts/{id}.
func (s *Server) handlePostByID(w http.ResponseWriter, r *http.Request) {
	// Извлекаем ID из пути
	path := strings.TrimPrefix(r.URL.Path, "/api/posts/")

	// Проверяем, не является ли это запросом на публикацию
	postID, ok := strings.CutSuffix(path, "/publish")
	if ok {
		s.publishPost(w, r, core.PostID(postID))
		return
	}

	id := core.PostID(postID)

	if id == "" {
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

	post, err := s.service.GetByID(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toJSONPost(post))
}

// updatePost обновляет пост.
func (s *Server) updatePost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	ctx := r.Context()

	var input struct {
		Title     *string    `json:"title,omitempty"`
		Status    *string    `json:"status,omitempty"`
		Tags      []string   `json:"tags,omitempty"`
		Deadline  *time.Time `json:"deadline,omitempty"`
		Scheduled *time.Time `json:"scheduled_at,omitempty"`
		Content   *string    `json:"content,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	post, err := s.service.GetByID(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	// Обновляем поля
	if input.Title != nil {
		post.Title = *input.Title
	}
	if input.Status != nil {
		newStatus := core.PostStatus(*input.Status)
		updatedPost, err := s.service.UpdateStatus(ctx, id, newStatus)
		if err != nil {
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

	if err := s.service.UpdatePost(ctx, post); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toJSONPost(post))
}

// deletePost удаляет пост.
func (s *Server) deletePost(w http.ResponseWriter, r *http.Request, id core.PostID) {
	ctx := r.Context()

	if err := s.service.DeletePost(ctx, id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleStats обрабатывает GET /api/stats.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	stats, err := s.service.GetStats(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

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

	// Парсим days из query параметра
	daysStr := r.URL.Query().Get("days")
	days := 30 // по умолчанию
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	posts, err := s.service.ListPosts(ctx, core.PostFilter{})
	if err != nil {
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

	// Сортируем по дате
	sortPlannedByDate(planned)

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
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Slug        string        `json:"slug"`
	Status      string        `json:"status"`
	Platforms   []string      `json:"platforms,omitempty"`
	Tags        []string      `json:"tags,omitempty"`
	Deadline    *time.Time    `json:"deadline,omitempty"`
	ScheduledAt *time.Time    `json:"scheduled_at,omitempty"`
	PublishedAt *time.Time    `json:"published_at,omitempty"`
	Content     string        `json:"content"`
	External    ExternalLinks `json:"external"`
}

// ExternalLinks ссылки на опубликованные посты.
type ExternalLinks struct {
	TelegramURL string `json:"telegram_url,omitempty"`
}

// toJSONPost конвертирует Post в jsonPost.
func toJSONPost(post *core.Post) jsonPost {
	platforms := make([]string, len(post.Platforms))
	for i, p := range post.Platforms {
		platforms[i] = string(p)
	}

	return jsonPost{
		ID:          string(post.ID),
		Title:       post.Title,
		Slug:        post.Slug,
		Status:      string(post.Status),
		Platforms:   platforms,
		Tags:        post.Tags,
		Deadline:    post.Deadline,
		ScheduledAt: post.ScheduledAt,
		PublishedAt: post.PublishedAt,
		Content:     post.Content,
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

	var input struct {
		Platforms []string `json:"platforms"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if len(input.Platforms) == 0 {
		http.Error(w, "platforms required", http.StatusBadRequest)
		return
	}

	// Конвертируем платформы
	platforms := make([]core.Platform, len(input.Platforms))
	for i, p := range input.Platforms {
		platforms[i] = core.Platform(p)
	}

	post, err := s.service.PublishPost(ctx, id, core.PublishPostInput{
		Platforms: platforms,
	}, s.publishers)
	if err != nil {
		// Обрабатываем разные типы ошибок
		switch {
		case errors.Is(err, core.ErrNotFound):
			http.Error(w, err.Error(), http.StatusNotFound)
		case errors.Is(err, core.ErrValidation):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case errors.Is(err, core.ErrInvalidStatus):
			http.Error(w, err.Error(), http.StatusBadRequest)
		case strings.Contains(err.Error(), "publisher для платформы"):
			http.Error(w, err.Error(), http.StatusBadRequest)
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(toJSONPost(post))
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
		ID:       string(post.ID),
		Title:    post.Title,
		Slug:     post.Slug,
		Status:   string(post.Status),
		Date:     date,
		DateType: dateType,
	}
}

// handlePlatforms обрабатывает GET /api/platforms.
func (s *Server) handlePlatforms(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Возвращаем список доступных платформ
	platforms := []string{"telegram"}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string][]string{"platforms": platforms})
}

// handleTags обрабатывает GET /api/tags.
func (s *Server) handleTags(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()

	// Получаем все посты для извлечения тегов
	posts, err := s.service.ListPosts(ctx, core.PostFilter{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Собираем уникальные теги
	tagSet := make(map[string]bool)
	for _, post := range posts {
		for _, tag := range post.Tags {
			tagSet[tag] = true
		}
	}

	// Конвертируем в срез
	tags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		tags = append(tags, tag)
	}

	// Сортируем теги по алфавиту
	for i := 0; i < len(tags)-1; i++ {
		for j := i + 1; j < len(tags); j++ {
			if tags[i] > tags[j] {
				tags[i], tags[j] = tags[j], tags[i]
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string][]string{"tags": tags})
}
