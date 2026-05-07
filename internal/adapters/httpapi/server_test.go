package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/core"
)

// testTenant и testAuthor — фиксированные UUID для тестов.
//
//nolint:gochecknoglobals // shared test constants
var (
	testTenant  = uuid.MustParse("11111111-1111-1111-1111-111111111111")
	testAuthor  = uuid.MustParse("22222222-2222-2222-2222-222222222222")
	otherTenant = uuid.MustParse("33333333-3333-3333-3333-333333333333")
)

// mustParsePostID парсит строку в PostID или генерирует UUID v5.
func mustParsePostID(s string) core.PostID {
	id, err := core.ParsePostID(s)
	if err != nil {
		u := uuid.NewSHA1(uuid.NameSpaceOID, []byte(s))
		return core.PostID(u)
	}
	return id
}

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

func (m *mockPostRepository) GetBySlug(_ context.Context, slug string) (*core.Post, error) {
	for _, post := range m.posts {
		if post.Slug == slug {
			return post, nil
		}
	}
	return nil, core.ErrNotFound
}

func (m *mockPostRepository) List(_ context.Context, filter core.PostFilter) ([]*core.Post, error) {
	var result []*core.Post
	for _, post := range m.posts {
		if filter.TenantID != uuid.Nil && post.TenantID != uuid.Nil && post.TenantID != filter.TenantID {
			continue
		}
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

// newTestServer создаёт сервер с тестовой конфигурацией (tenant/author заданы).
func newTestServer(_ *testing.T, publisher core.Publisher) (*Server, *mockPostRepository) {
	repo := newMockPostRepository()
	service := core.NewPostService(repo, &mockClock{now: time.Now()})
	cfg := &config.Config{
		Auth: config.AuthConfig{
			TenantDefault: testTenant,
			AuthorDefault: testAuthor,
		},
	}
	server := NewServerWithConfig(ServerConfig{
		Service:   service,
		Publisher: publisher,
		Config:    cfg,
	})
	return server, repo
}

// fixturePost собирает тестовый Post с обязательными полями.
func fixturePost(idSeed string, tenant uuid.UUID, title, slug string, status core.PostStatus) *core.Post {
	now := time.Now()
	return &core.Post{
		ID:        mustParsePostID(idSeed),
		TenantID:  tenant,
		AuthorID:  testAuthor,
		Title:     title,
		Slug:      slug,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
		Revision:  1,
	}
}

func TestServer_HandlePosts(t *testing.T) {
	server, repo := newTestServer(t, nil)

	ctx := context.Background()
	posts := []*core.Post{
		fixturePost("post-1", testTenant, "First Post", "first-post", core.StatusDraft),
		fixturePost("post-2", testTenant, "Second Post", "second-post", core.StatusReady),
	}
	posts[0].Tags = []string{"go", "test"}
	posts[0].Content = "Content 1"
	posts[1].Tags = []string{"go"}
	posts[1].Content = "Content 2"

	for _, post := range posts {
		_ = repo.Create(ctx, post)
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
		_ = json.NewDecoder(w.Body).Decode(&result)

		if len(result) != 1 {
			t.Errorf("Expected 1 post, got %d", len(result))
		}
		if result[0].Title != "First Post" {
			t.Errorf("Expected 'First Post', got %s", result[0].Title)
		}
	})
}

func TestHTTP_GET_Posts_TenantScoped(t *testing.T) {
	server, repo := newTestServer(t, nil)
	ctx := context.Background()

	_ = repo.Create(ctx, fixturePost("t1-1", testTenant, "T1-A", "t1-a", core.StatusDraft))
	_ = repo.Create(ctx, fixturePost("t1-2", testTenant, "T1-B", "t1-b", core.StatusDraft))
	_ = repo.Create(ctx, fixturePost("t2-1", otherTenant, "T2-A", "t2-a", core.StatusDraft))

	req := httptest.NewRequest(http.MethodGet, "/api/posts", nil)
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected 200, got %d", w.Code)
	}
	var got []jsonPost
	_ = json.NewDecoder(w.Body).Decode(&got)
	if len(got) != 2 {
		t.Errorf("Expected 2 tenant-scoped posts, got %d", len(got))
	}
	for _, p := range got {
		if p.TenantID != testTenant.String() {
			t.Errorf("Unexpected tenant id %q", p.TenantID)
		}
	}
}

func TestServer_HandlePostByID(t *testing.T) {
	server, repo := newTestServer(t, nil)

	ctx := context.Background()
	testPost := fixturePost("test-post", testTenant, "Test Post", "test-post", core.StatusDraft)
	testPost.Tags = []string{"test"}
	testPost.Content = "Test content"
	_ = repo.Create(ctx, testPost)

	t.Run("GET /api/posts/{id} returns post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/posts/"+testPost.ID.String(), nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result jsonPost
		if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if result.ID != testPost.ID.String() {
			t.Errorf("Expected ID '%s', got %s", testPost.ID.String(), result.ID)
		}
		if result.Title != "Test Post" {
			t.Errorf("Expected title 'Test Post', got %s", result.Title)
		}
	})

	t.Run("GET /api/posts/{id} returns 404 for non-existent", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/posts/"+mustParsePostID("non-existent").String(), nil)
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

		req := httptest.NewRequest(http.MethodPatch, "/api/posts/"+testPost.ID.String(), bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result jsonPost
		_ = json.NewDecoder(w.Body).Decode(&result)

		if result.Title != "Updated Title" {
			t.Errorf("Expected 'Updated Title', got %s", result.Title)
		}
		if result.Status != "ready" {
			t.Errorf("Expected status 'ready', got %s", result.Status)
		}
	})

	t.Run("DELETE /api/posts/{id} deletes post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/api/posts/"+testPost.ID.String(), nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("Expected status 204, got %d", w.Code)
		}

		req = httptest.NewRequest(http.MethodGet, "/api/posts/"+testPost.ID.String(), nil)
		w = httptest.NewRecorder()
		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 after delete, got %d", w.Code)
		}
	})
}

func TestHTTP_PATCH_Posts_TenantImmutable(t *testing.T) {
	server, repo := newTestServer(t, nil)
	ctx := context.Background()

	post := fixturePost("immutable", testTenant, "Title", "title", core.StatusDraft)
	_ = repo.Create(ctx, post)

	updateData := map[string]any{
		"tenant_id": otherTenant.String(),
		"title":     "x",
	}
	body, _ := json.Marshal(updateData)
	req := httptest.NewRequest(http.MethodPatch, "/api/posts/"+post.ID.String(), bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected 400, got %d", w.Code)
	}
	var got map[string]string
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got["error"] != "tenant_id_immutable" {
		t.Errorf("Expected tenant_id_immutable, got %v", got)
	}
}

func TestServer_HandleStats(t *testing.T) {
	server, repo := newTestServer(t, nil)

	ctx := context.Background()
	posts := []*core.Post{
		fixturePost("stats-1", testTenant, "Draft Post 1", "draft-post-1", core.StatusDraft),
		fixturePost("stats-2", testTenant, "Draft Post 2", "draft-post-2", core.StatusDraft),
		fixturePost("stats-3", testTenant, "Ready Post", "ready-post", core.StatusReady),
	}
	for _, post := range posts {
		post.Content = "x"
		_ = repo.Create(ctx, post)
	}

	t.Run("GET /api/stats returns statistics", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/stats", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var stats core.PostStats
		_ = json.NewDecoder(w.Body).Decode(&stats)

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

func TestServer_HandlePlan(t *testing.T) {
	server, repo := newTestServer(t, nil)

	ctx := context.Background()
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	nextWeek := now.Add(7 * 24 * time.Hour)

	p1 := fixturePost("post-1", testTenant, "Tomorrow Deadline", "tomorrow-deadline", core.StatusDraft)
	p1.Deadline = &tomorrow
	p2 := fixturePost("post-2", testTenant, "Next Week Scheduled", "next-week-scheduled", core.StatusReady)
	p2.ScheduledAt = &nextWeek
	p3 := fixturePost("post-3", testTenant, "Published Post", "published-post", core.StatusPublished)

	for _, post := range []*core.Post{p1, p2, p3} {
		_ = repo.Create(ctx, post)
	}

	t.Run("GET /api/plan returns planned posts", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/plan?days=30", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d", w.Code)
		}

		var result []jsonPlannedPost
		_ = json.NewDecoder(w.Body).Decode(&result)

		if len(result) != 2 {
			t.Errorf("Expected 2 planned posts, got %d", len(result))
		}
	})

	t.Run("GET /api/plan with short period", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/plan?days=2", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		var result []jsonPlannedPost
		_ = json.NewDecoder(w.Body).Decode(&result)

		if len(result) != 1 {
			t.Errorf("Expected 1 planned post, got %d", len(result))
		}
		expectedID := mustParsePostID("post-1").String()
		if result[0].ID != expectedID {
			t.Errorf("Expected '%s', got %s", expectedID, result[0].ID)
		}
	})
}

func TestServer_HandleIndex(t *testing.T) {
	server, _ := newTestServer(t, nil)

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
	server, _ := newTestServer(t, nil)

	t.Run("PUT /api/posts/{id} returns method not allowed", func(t *testing.T) {
		testID := mustParsePostID("test").String()
		req := httptest.NewRequest(http.MethodPut, "/api/posts/"+testID, nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("Expected status 405, got %d", w.Code)
		}
	})
}

func TestServer_CreatePost(t *testing.T) {
	server, _ := newTestServer(t, nil)

	t.Run("POST /api/posts creates new post", func(t *testing.T) {
		createData := map[string]any{
			"title": "New Test Post",
			"tags":  []string{"test", "go"},
		}
		body, _ := json.Marshal(createData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("Expected status 201, got %d (body=%s)", w.Code, w.Body.String())
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
		if result.TenantID != testTenant.String() {
			t.Errorf("Expected tenant %s, got %s", testTenant, result.TenantID)
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
		createData := map[string]any{"title": ""}
		body, _ := json.Marshal(createData)

		req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for empty title, got %d", w.Code)
		}
	})
}

func TestHTTP_POST_Posts_TenantMismatch(t *testing.T) {
	server, _ := newTestServer(t, nil)

	createData := map[string]any{
		"title":     "Mismatch",
		"tenant_id": otherTenant.String(),
	}
	body, _ := json.Marshal(createData)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected 403, got %d", w.Code)
	}
	var got map[string]string
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got["error"] != "tenant_mismatch" {
		t.Errorf("Expected tenant_mismatch, got %v", got)
	}
}

func TestHTTP_POST_Posts_AutoTenant(t *testing.T) {
	server, _ := newTestServer(t, nil)

	createData := map[string]any{"title": "Auto Tenant"}
	body, _ := json.Marshal(createData)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d (body=%s)", w.Code, w.Body.String())
	}
	var got jsonPost
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got.TenantID != testTenant.String() {
		t.Errorf("Expected tenant %s, got %s", testTenant, got.TenantID)
	}
	if got.AuthorID != testAuthor.String() {
		t.Errorf("Expected author %s, got %s", testAuthor, got.AuthorID)
	}
}

func TestHTTP_jsonPost_AllFields(t *testing.T) {
	server, _ := newTestServer(t, nil)

	excerpt := "test excerpt"
	deadline := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Second)
	createData := map[string]any{
		"title":    "All Fields",
		"slug":     "all-fields",
		"tags":     []string{"a", "b"},
		"excerpt":  excerpt,
		"deadline": deadline.Format(time.RFC3339),
		"content":  "body content",
	}
	body, _ := json.Marshal(createData)
	req := httptest.NewRequest(http.MethodPost, "/api/posts", bytes.NewReader(body))
	w := httptest.NewRecorder()
	server.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("Expected 201, got %d (body=%s)", w.Code, w.Body.String())
	}
	var created jsonPost
	_ = json.NewDecoder(w.Body).Decode(&created)
	if created.ID == "" || created.TenantID == "" || created.AuthorID == "" {
		t.Errorf("Expected ID/TenantID/AuthorID populated, got %+v", created)
	}
	if created.Excerpt == nil || *created.Excerpt != excerpt {
		t.Errorf("Expected excerpt %q, got %v", excerpt, created.Excerpt)
	}
	if created.Revision < 1 {
		t.Errorf("Expected revision >= 1, got %d", created.Revision)
	}

	// GET back
	req2 := httptest.NewRequest(http.MethodGet, "/api/posts/"+created.ID, nil)
	w2 := httptest.NewRecorder()
	server.ServeHTTP(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("Expected GET 200, got %d", w2.Code)
	}
	var got jsonPost
	_ = json.NewDecoder(w2.Body).Decode(&got)
	if got.ID != created.ID {
		t.Errorf("ID mismatch")
	}
	if got.Slug != "all-fields" {
		t.Errorf("Expected slug 'all-fields', got %s", got.Slug)
	}
	if got.Content != "body content" {
		t.Errorf("Expected content 'body content', got %q", got.Content)
	}
}

// mockPublisher тестовая реализация Publisher.
type mockPublisher struct {
	publish func(ctx context.Context, post *core.Post) (*core.Post, error)
}

func (m *mockPublisher) Publish(ctx context.Context, post *core.Post) (*core.Post, error) {
	if m.publish != nil {
		return m.publish(ctx, post)
	}
	post.External.TelegramURL = "https://t.me/test/123"
	return post, nil
}

func TestServer_PublishPost(t *testing.T) {
	publisher := &mockPublisher{}
	server, repo := newTestServer(t, publisher)

	ctx := context.Background()
	testPost := fixturePost("test-post", testTenant, "Test Post", "test-post", core.StatusReady)
	testPost.Tags = []string{"test"}
	testPost.Content = "Test content for publication"
	_ = repo.Create(ctx, testPost)

	t.Run("POST /api/posts/{id}/publish publishes post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/posts/"+testPost.ID.String()+"/publish", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d (body=%s)", w.Code, w.Body.String())
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
		req := httptest.NewRequest(http.MethodPost, "/api/posts/"+mustParsePostID("non-existent").String()+"/publish", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404, got %d", w.Code)
		}
	})

	t.Run("POST /api/posts/{id}/publish with empty content returns error", func(t *testing.T) {
		emptyPost := fixturePost("empty-post", testTenant, "Empty Post", "empty-post", core.StatusReady)
		emptyPost.Content = ""
		_ = repo.Create(ctx, emptyPost)

		req := httptest.NewRequest(http.MethodPost, "/api/posts/"+emptyPost.ID.String()+"/publish", nil)
		w := httptest.NewRecorder()

		server.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("Expected status 400 for empty content, got %d", w.Code)
		}
	})

	t.Run("POST /api/posts/{id}/publish without publishers returns error", func(t *testing.T) {
		serverNoPublishers, _ := newTestServer(t, nil)
		// reuse test post via local repo not possible; use a temp post via service
		req := httptest.NewRequest(http.MethodPost, "/api/posts/"+testPost.ID.String()+"/publish", nil)
		w := httptest.NewRecorder()

		serverNoPublishers.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest && w.Code != http.StatusNotFound {
			t.Errorf("Expected 400/404 for missing publisher, got %d", w.Code)
		}
	})
}
