package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

// mockPostRepository тестовая реализация репозитория.
type mockPostRepository struct {
	posts map[core.PostID]*core.Post
}

func newMockPostRepository() *mockPostRepository {
	return &mockPostRepository{
		posts: make(map[core.PostID]*core.Post),
	}
}

func (m *mockPostRepository) Create(_ context.Context, post *core.Post) error {
	if _, exists := m.posts[post.ID]; exists {
		return core.ErrAlreadyExists
	}
	m.posts[post.ID] = post
	return nil
}

func (m *mockPostRepository) GetByID(_ context.Context, id core.PostID) (*core.Post, error) {
	post, exists := m.posts[id]
	if !exists {
		return nil, core.ErrNotFound
	}
	return post, nil
}

func (m *mockPostRepository) List(_ context.Context, filter core.PostFilter) ([]*core.Post, error) {
	var result []*core.Post
	for _, post := range m.posts {
		// Простая фильтрация для тестов
		if filter.Search != "" && !contains(post.Title, filter.Search) {
			continue
		}
		result = append(result, post)
	}
	return result, nil
}

func (m *mockPostRepository) Update(_ context.Context, post *core.Post) error {
	m.posts[post.ID] = post
	return nil
}

func (m *mockPostRepository) Delete(_ context.Context, id core.PostID) error {
	delete(m.posts, id)
	return nil
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// mockClock тестовые часы.
type mockClock struct {
	now time.Time
}

func (m *mockClock) Now() time.Time {
	return m.now
}

func TestServer_HandlePosts(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	server := NewServer(service, nil)

	// Создаём тестовые посты
	ctx := context.Background()
	posts := []*core.Post{
		{
			ID:        "post-1",
			Title:     "First Post",
			Slug:      "first-post",
			Status:    core.StatusDraft,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"go", "test"},
			Content:   "Content 1",
		},
		{
			ID:        "post-2",
			Title:     "Second Post",
			Slug:      "second-post",
			Status:    core.StatusReady,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"go"},
			Content:   "Content 2",
		},
	}

	for _, post := range posts {
		repo.Create(ctx, post)
	}

	t.Run("GET /api/posts returns all posts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result []jsonPost
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if len(result) != 2 {
			t.Errorf("Expected 2 posts, got %d", len(result))
		}
	})

	t.Run("GET /api/posts with search filter", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/posts?search=First", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		var result []jsonPost
		json.NewDecoder(w.Body).Decode(&result)

		if len(result) != 1 {
			t.Errorf("Expected 1 post, got %d", len(result))
		}
		if result[0].Title != "First Post" {
			t.Errorf("Expected 'First Post', got %s", result[0].Title)
		}
	})
}

func TestServer_HandlePostByID(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	server := NewServer(service, nil)

	ctx := context.Background()
	testPost := &core.Post{
		ID:        "test-post",
		Title:     "Test Post",
		Slug:      "test-post",
		Status:    core.StatusDraft,
		Platforms: []core.Platform{core.PlatformTelegram},
		Tags:      []string{"test"},
		Content:   "Test content",
	}
	repo.Create(ctx, testPost)

	t.Run("GET /api/posts/{id} returns post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/posts/test-post", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result jsonPost
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.ID != "test-post" {
			t.Errorf("Expected ID 'test-post', got %s", result.ID)
		}
		if result.Title != "Test Post" {
			t.Errorf("Expected title 'Test Post', got %s", result.Title)
		}
	})

	t.Run("GET /api/posts/{id} returns 404 for non-existent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/posts/non-existent", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("PATCH /api/posts/{id} updates post", func(t *testing.T) {
		updateData := map[string]any{
			"title":  "Updated Title",
			"status": "ready",
			"tags":   []string{"test", "updated"},
		}
		body, _ := json.Marshal(updateData)

		req := httptest.NewRequest(http.MethodPatch, "/api/posts/test-post", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result jsonPost
		json.NewDecoder(w.Body).Decode(&result)

		if result.Title != "Updated Title" {
			t.Errorf("Expected 'Updated Title', got %s", result.Title)
		}
		if result.Status != "ready" {
			t.Errorf("Expected status 'ready', got %s", result.Status)
		}
	})

	t.Run("DELETE /api/posts/{id} deletes post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/posts/test-post", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", w.Code)
		}

		// Проверяем, что пост удалён
		req = httptest.NewRequest(http.MethodGet, "/api/posts/test-post", nil)
		w = httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 after delete, got %d", w.Code)
		}
	})
}

func TestServer_HandleStats(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	server := NewServer(service, nil)

	ctx := context.Background()
	posts := []*core.Post{
		{ID: "1", Title: "Post 1", Slug: "post-1", Status: core.StatusDraft, Platforms: []core.Platform{core.PlatformTelegram}, Tags: []string{"go"}},
		{ID: "2", Title: "Post 2", Slug: "post-2", Status: core.StatusDraft, Platforms: []core.Platform{core.PlatformTelegram}, Tags: []string{"test"}},
		{ID: "3", Title: "Post 3", Slug: "post-3", Status: core.StatusReady, Platforms: []core.Platform{core.PlatformTelegram}, Tags: []string{"go"}},
	}

	for _, post := range posts {
		repo.Create(ctx, post)
	}

	t.Run("GET /api/stats returns statistics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var stats core.PostStats
		json.NewDecoder(w.Body).Decode(&stats)

		if stats.Total != 3 {
			t.Errorf("Expected total 3, got %d", stats.Total)
		}
		if stats.ByStatus[core.StatusDraft] != 2 {
			t.Errorf("Expected 2 draft posts, got %d", stats.ByStatus[core.StatusDraft])
		}
		if stats.ByStatus[core.StatusReady] != 1 {
			t.Errorf("Expected 1 ready post, got %d", stats.ByStatus[core.StatusReady])
		}
	})
}

func TestServer_HandleNext(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	server := NewServer(service, nil)

	t.Run("GET /api/next returns 404 when no posts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/next", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("GET /api/next returns recommended post", func(t *testing.T) {
		ctx := context.Background()
		now := time.Now()
		deadline := now.Add(-24 * time.Hour) // Просроченный дедлайн

		post := &core.Post{
			ID:       "urgent-post",
			Title:    "Urgent Post",
			Slug:     "urgent-post",
			Status:   core.StatusDraft,
			Deadline: &deadline,
		}
		repo.Create(ctx, post)

		req := httptest.NewRequest(http.MethodGet, "/api/next", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result jsonPost
		json.NewDecoder(w.Body).Decode(&result)

		if result.ID != "urgent-post" {
			t.Errorf("Expected 'urgent-post', got %s", result.ID)
		}
	})
}

func TestServer_HandlePlan(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	server := NewServer(service, nil)

	ctx := context.Background()
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	nextWeek := now.Add(7 * 24 * time.Hour)

	posts := []*core.Post{
		{
			ID:       "post-1",
			Title:    "Tomorrow Deadline",
			Slug:     "tomorrow-deadline",
			Status:   core.StatusDraft,
			Deadline: &tomorrow,
		},
		{
			ID:          "post-2",
			Title:       "Next Week Scheduled",
			Slug:        "next-week-scheduled",
			Status:      core.StatusReady,
			ScheduledAt: &nextWeek,
		},
		{
			ID:     "post-3",
			Title:  "Published Post",
			Slug:   "published-post",
			Status: core.StatusPublished,
		},
	}

	for _, post := range posts {
		repo.Create(ctx, post)
	}

	t.Run("GET /api/plan returns planned posts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/plan?days=30", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result []jsonPlannedPost
		json.NewDecoder(w.Body).Decode(&result)

		// Должно быть 2 поста (опубликованный исключён)
		if len(result) != 2 {
			t.Errorf("Expected 2 planned posts, got %d", len(result))
		}
	})

	t.Run("GET /api/plan with short period", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/plan?days=2", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		var result []jsonPlannedPost
		json.NewDecoder(w.Body).Decode(&result)

		// Должен попасть только пост с завтрашним дедлайном
		if len(result) != 1 {
			t.Errorf("Expected 1 planned post, got %d", len(result))
		}
		if result[0].ID != "post-1" {
			t.Errorf("Expected 'post-1', got %s", result[0].ID)
		}
	})
}

func TestServer_HandleIndex(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	server := NewServer(service, nil)

	t.Run("GET / returns HTML page", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		contentType := w.Header().Get("Content-Type")
		if !contains(contentType, "text/html") {
			t.Errorf("Expected text/html content type, got %s", contentType)
		}

		if !contains(w.Body.String(), "jtpost") {
			t.Errorf("Expected 'jtpost' in HTML response")
		}
	})

	t.Run("GET /nonexistent returns 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})
}

func TestServer_MethodNotAllowed(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	server := NewServer(service, nil)

	t.Run("PUT /api/posts/{id} returns method not allowed", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPut, "/api/posts/test", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
}

func TestServer_CreatePost(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	server := NewServer(service, nil)

	t.Run("POST /api/posts creates new post", func(t *testing.T) {
		createData := map[string]any{
			"title":     "New Test Post",
			"platforms": []string{"telegram"},
			"tags":      []string{"test", "go"},
		}
		body, _ := json.Marshal(createData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
		}

		var result jsonPost
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.Title != "New Test Post" {
			t.Errorf("Expected 'New Test Post', got %s", result.Title)
		}
		if result.Status != "idea" {
			t.Errorf("Expected status 'idea', got %s", result.Status)
		}
		if len(result.Tags) != 2 {
			t.Errorf("Expected 2 tags, got %d", len(result.Tags))
		}
	})

	t.Run("POST /api/posts with empty body returns bad request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/posts", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for empty body, got %d", w.Code)
		}
	})

	t.Run("POST /api/posts with empty title returns bad request", func(t *testing.T) {
		createData := map[string]any{
			"title": "",
		}
		body, _ := json.Marshal(createData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for empty title, got %d", w.Code)
		}
	})

	t.Run("POST /api/posts with default platform", func(t *testing.T) {
		createData := map[string]any{
			"title": "Post with default platform",
			"tags":  []string{"test"},
		}
		body, _ := json.Marshal(createData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d", w.Code)
		}

		var result jsonPost
		json.NewDecoder(w.Body).Decode(&result)

		if len(result.Platforms) != 1 {
			t.Errorf("Expected 1 platform (default), got %d", len(result.Platforms))
		}
		if result.Platforms[0] != "telegram" {
			t.Errorf("Expected default platform 'telegram', got %s", result.Platforms[0])
		}
	})
}

// mockPublisher тестовая реализация Publisher.
type mockPublisher struct {
	platform core.Platform
	publish  func(ctx context.Context, post *core.Post) (*core.Post, error)
}

func (m *mockPublisher) Platform() core.Platform {
	return m.platform
}

func (m *mockPublisher) Publish(ctx context.Context, post *core.Post) (*core.Post, error) {
	if m.publish != nil {
		return m.publish(ctx, post)
	}
	// Имитация успешной публикации
	post.External.TelegramURL = "https://t.me/test/123"
	return post, nil
}

func TestServer_PublishPost(t *testing.T) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})

	// Создаём mock publisher
	publishers := map[core.Platform]core.Publisher{
		core.PlatformTelegram: &mockPublisher{platform: core.PlatformTelegram},
	}

	server := NewServer(service, publishers)

	ctx := context.Background()
	testPost := &core.Post{
		ID:        "test-post",
		Title:     "Test Post",
		Slug:      "test-post",
		Status:    core.StatusReady,
		Platforms: []core.Platform{core.PlatformTelegram},
		Tags:      []string{"test"},
		Content:   "Test content for publication",
	}
	repo.Create(ctx, testPost)

	t.Run("POST /api/posts/{id}/publish publishes post", func(t *testing.T) {
		publishData := map[string]any{
			"platforms": []string{"telegram"},
		}
		body, _ := json.Marshal(publishData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts/test-post/publish", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result jsonPost
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.Status != "published" {
			t.Errorf("Expected status 'published', got %s", result.Status)
		}
		if result.External.TelegramURL == "" {
			t.Error("Expected telegram_url to be set")
		}
	})

	t.Run("POST /api/posts/{id}/publish with non-existent post returns 404", func(t *testing.T) {
		publishData := map[string]any{
			"platforms": []string{"telegram"},
		}
		body, _ := json.Marshal(publishData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts/non-existent/publish", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("POST /api/posts/{id}/publish with empty content returns error", func(t *testing.T) {
		emptyPost := &core.Post{
			ID:        "empty-post",
			Title:     "Empty Post",
			Slug:      "empty-post",
			Status:    core.StatusReady,
			Platforms: []core.Platform{core.PlatformTelegram},
			Content:   "",
		}
		repo.Create(ctx, emptyPost)

		publishData := map[string]any{
			"platforms": []string{"telegram"},
		}
		body, _ := json.Marshal(publishData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts/empty-post/publish", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for empty content, got %d", w.Code)
		}
	})

	t.Run("POST /api/posts/{id}/publish without publishers returns error", func(t *testing.T) {
		serverNoPublishers := NewServer(service, nil)

		publishData := map[string]any{
			"platforms": []string{"telegram"},
		}
		body, _ := json.Marshal(publishData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts/test-post/publish", bytes.NewReader(body))
		w := httptest.NewRecorder()

		serverNoPublishers.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for missing publishers, got %d", w.Code)
		}
	})
}
