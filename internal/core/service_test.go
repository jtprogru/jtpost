package core

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

// mustParsePostID парсит строку в PostID или паникует при ошибке.
// Используется только в тестах.
func mustParsePostID(s string) PostID {
	id, err := ParsePostID(s)
	if err != nil {
		// Fallback: генерируем UUID из строки используя SHA256
		u := uuid.NewSHA1(uuid.NameSpaceOID, []byte(s))
		return PostID(u)
	}
	return id
}

func TestPostService_CreatePost(t *testing.T) {
	repo := &mockRepository{
		posts: make(map[PostID]*Post),
	}
	clock := SystemClock{}
	service := NewPostService(repo, clock)

	tests := []struct {
		name      string
		input     CreatePostInput
		wantErr   bool
		errType   error
		checkPost func(*Post) bool
	}{
		{
			name: "valid post creation",
			input: CreatePostInput{
				Title: "Test Post",
				Tags:  []string{"test", "go"},
			},
			wantErr: false,
			checkPost: func(p *Post) bool {
				return p.Title == "Test Post" &&
					p.Status == StatusIdea &&
					len(p.Tags) == 2
			},
		},
		{
			name: "empty title",
			input: CreatePostInput{
				Title: "",
			},
			wantErr: true,
			errType: ErrEmptyTitle,
		},
		{
			name: "auto-generate slug",
			input: CreatePostInput{
				Title: "Test Post Without Slug",
			},
			wantErr: false,
			checkPost: func(p *Post) bool {
				return p.Slug == "test-post-without-slug"
			},
		},
		{
			name: "custom slug",
			input: CreatePostInput{
				Title: "Test Post",
				Slug:  "custom-slug",
			},
			wantErr: false,
			checkPost: func(p *Post) bool {
				return p.Slug == "custom-slug"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			post, err := service.CreatePost(context.Background(), tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("CreatePost() expected error, got nil")
				} else if tt.errType != nil && !errors.Is(err, tt.errType) {
					t.Errorf("CreatePost() expected error %v, got %v", tt.errType, err)
				}
			} else {
				if err != nil {
					t.Errorf("CreatePost() unexpected error: %v", err)
				}
				if tt.checkPost != nil && !tt.checkPost(post) {
					t.Errorf("CreatePost() post validation failed")
				}
			}
		})
	}
}

func TestPostService_UpdateStatus(t *testing.T) {
	repo := &mockRepository{
		posts: make(map[PostID]*Post),
	}
	clock := SystemClock{}
	service := NewPostService(repo, clock)

	// Создаём тестовый пост
	initialPost := &Post{
		ID:        mustParsePostID("test-id"),
		Title:     "Test",
		Slug:      "test",
		Status:    StatusDraft,
	}
	repo.posts[initialPost.ID] = initialPost

	tests := []struct {
		name      string
		id        PostID
		newStatus PostStatus
		wantErr   bool
		checkErr  func(error) bool
	}{
		{"draft to ready", mustParsePostID("test-id"), StatusReady, false, nil},
		{"draft to published", mustParsePostID("test-id"), StatusPublished, false, nil},
		{"ready to draft", mustParsePostID("test-id"), StatusDraft, true, func(err error) bool {
			return err != nil
		}},
		{"not found", mustParsePostID("non-existent"), StatusReady, true, func(err error) bool {
			return errors.Is(err, ErrNotFound)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := service.UpdateStatus(context.Background(), tt.id, tt.newStatus)

			if tt.wantErr {
				if err == nil {
					t.Errorf("UpdateStatus() expected error, got nil")
				} else if tt.checkErr != nil && !tt.checkErr(err) {
					t.Errorf("UpdateStatus() unexpected error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("UpdateStatus() unexpected error: %v", err)
				}
			}
		})
	}
}

func TestPostService_GenerateSlug(t *testing.T) {
	repo := &mockRepository{posts: make(map[PostID]*Post)}
	service := NewPostService(repo, SystemClock{})

	tests := []struct {
		title    string
		expected string
	}{
		{"Hello World", "hello-world"},
		{"Test 123", "test-123"},
		{"Multiple   Spaces", "multiple-spaces"},
		{"Special!@#Chars", "special-chars"}, // спецсимволы заменяются на дефис
		{"  Trimmed  ", "trimmed"},
		{"Cyrillic Тест", "cyrillic-test"}, // кириллица транслитерируется
		{"Привет Мир", "privet-mir"},
		{"Golang урок", "golang-urok"},
	}

	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			result := service.generateSlug(tt.title)
			if result != tt.expected {
				t.Errorf("generateSlug(%q) = %q, expected %q", tt.title, result, tt.expected)
			}
		})
	}
}

func TestPostService_DeletePost(t *testing.T) {
	repo := &mockRepository{
		posts: make(map[PostID]*Post),
	}
	clock := SystemClock{}
	service := NewPostService(repo, clock)

	// Создаём тестовый пост
	initialPost := &Post{
		ID:        mustParsePostID("test-id"),
		Title:     "Test",
		Slug:      "test",
		Status:    StatusDraft,
	}
	repo.posts[initialPost.ID] = initialPost

	tests := []struct {
		name      string
		id        PostID
		wantErr   bool
		checkErr  func(error) bool
		checkPost func() bool
	}{
		{
			name:    "delete existing post",
			id:      mustParsePostID("test-id"),
			wantErr: false,
			checkPost: func() bool {
				_, exists := repo.posts[mustParsePostID("test-id")]
				return !exists // пост должен быть удалён
			},
		},
		{
			name:    "delete non-existent post",
			id:      mustParsePostID("non-existent"),
			wantErr: true,
			checkErr: func(err error) bool {
				return errors.Is(err, ErrNotFound)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.DeletePost(context.Background(), tt.id)

			if tt.wantErr {
				if err == nil {
					t.Errorf("DeletePost() expected error, got nil")
				} else if tt.checkErr != nil && !tt.checkErr(err) {
					t.Errorf("DeletePost() unexpected error: %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("DeletePost() unexpected error: %v", err)
				}
				if tt.checkPost != nil && !tt.checkPost() {
					t.Errorf("DeletePost() post was not deleted")
				}
			}
		})
	}
}

// mockRepository — тестовая реализация PostRepository.
type mockRepository struct {
	posts   map[PostID]*Post
	listErr error
}

func (m *mockRepository) GetByID(ctx context.Context, id PostID) (*Post, error) {
	post, ok := m.posts[id]
	if !ok {
		return nil, ErrNotFound
	}
	return post, nil
}

func (m *mockRepository) GetBySlug(ctx context.Context, slug string) (*Post, error) {
	for _, post := range m.posts {
		if post.Slug == slug {
			return post, nil
		}
	}
	return nil, ErrNotFound
}

func (m *mockRepository) List(ctx context.Context, filter PostFilter) ([]*Post, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	var posts []*Post
	for _, post := range m.posts {
		if matchesFilter(post, filter) {
			posts = append(posts, post)
		}
	}
	return posts, nil
}

func (m *mockRepository) Create(ctx context.Context, post *Post) error {
	if _, ok := m.posts[post.ID]; ok {
		return ErrAlreadyExists
	}
	m.posts[post.ID] = post
	return nil
}

func (m *mockRepository) Update(ctx context.Context, post *Post) error {
	if _, ok := m.posts[post.ID]; !ok {
		return ErrNotFound
	}
	m.posts[post.ID] = post
	return nil
}

func (m *mockRepository) Delete(ctx context.Context, id PostID) error {
	if _, ok := m.posts[id]; !ok {
		return ErrNotFound
	}
	delete(m.posts, id)
	return nil
}

// matchesFilter — вспомогательная функция для фильтрации.
func matchesFilter(post *Post, filter PostFilter) bool {
	if len(filter.Statuses) > 0 {
		found := false
		for _, s := range filter.Statuses {
			if post.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func TestPostService_GetStats(t *testing.T) {
	repo := &mockRepository{
		posts: make(map[PostID]*Post),
	}
	clock := SystemClock{}
	service := NewPostService(repo, clock)

	// Создаём тестовые посты
	testPosts := []*Post{
		{
			ID:        mustParsePostID("post-1"),
			Title:     "Draft Post",
			Slug:      "draft-post",
			Status:    StatusDraft,
			Tags:      []string{"go", "tutorial"},
		},
		{
			ID:        mustParsePostID("post-2"),
			Title:     "Ready Post",
			Slug:      "ready-post",
			Status:    StatusReady,
			Tags:      []string{"go", "news"},
		},
		{
			ID:        mustParsePostID("post-3"),
			Title:     "Published Post",
			Slug:      "published-post",
			Status:    StatusPublished,
			Tags:      []string{"tutorial"},
		},
		{
			ID:        mustParsePostID("post-4"),
			Title:     "Another Draft",
			Slug:      "another-draft",
			Status:    StatusDraft,
			Tags:      []string{"go", "cli"},
		},
	}

	for _, post := range testPosts {
		repo.posts[post.ID] = post
	}

	t.Run("returns correct statistics", func(t *testing.T) {
		stats, err := service.GetStats(context.Background())
		if err != nil {
			t.Fatalf("GetStats() unexpected error: %v", err)
		}

		if stats.Total != 4 {
			t.Errorf("GetStats() Total = %d, want 4", stats.Total)
		}

		// Проверка по статусам
		expectedByStatus := map[PostStatus]int{
			StatusDraft:     2,
			StatusReady:     1,
			StatusPublished: 1,
		}
		for status, expected := range expectedByStatus {
			if stats.ByStatus[status] != expected {
				t.Errorf("GetStats() ByStatus[%s] = %d, want %d", status, stats.ByStatus[status], expected)
			}
		}

		// Проверка по тегам
		expectedByTag := map[string]int{
			"go":       3,
			"tutorial": 2,
			"news":     1,
			"cli":      1,
		}
		for tag, expected := range expectedByTag {
			if stats.ByTag[tag] != expected {
				t.Errorf("GetStats() ByTag[%s] = %d, want %d", tag, stats.ByTag[tag], expected)
			}
		}
	})

	t.Run("empty repository", func(t *testing.T) {
		emptyRepo := &mockRepository{
			posts: make(map[PostID]*Post),
		}
		emptyService := NewPostService(emptyRepo, clock)

		stats, err := emptyService.GetStats(context.Background())
		if err != nil {
			t.Fatalf("GetStats() unexpected error: %v", err)
		}

		if stats.Total != 0 {
			t.Errorf("GetStats() Total = %d, want 0", stats.Total)
		}

		if len(stats.ByStatus) != 0 {
			t.Errorf("GetStats() ByStatus should be empty")
		}

		if len(stats.ByTag) != 0 {
			t.Errorf("GetStats() ByTag should be empty")
		}
	})

	t.Run("repository error", func(t *testing.T) {
		errorRepo := &mockRepository{
			posts:   make(map[PostID]*Post),
			listErr: errors.New("list error"),
		}
		errorService := NewPostService(errorRepo, clock)

		_, err := errorService.GetStats(context.Background())
		if err == nil {
			t.Errorf("GetStats() expected error, got nil")
		}
	})
}

func TestPostService_GetNextPost(t *testing.T) {
	repo := &mockRepository{
		posts: make(map[PostID]*Post),
	}
	clock := SystemClock{}
	service := NewPostService(repo, clock)

	now := clock.Now()
	
	// Создаём временные метки для тестов
	pastDeadline := now.Add(-24 * time.Hour)   // Просроченный дедлайн
	futureDeadline := now.Add(48 * time.Hour)  // Будущий дедлайн
	pastScheduled := now.Add(-12 * time.Hour)  // Просроченное scheduled

	// Создаём тестовые посты с разными приоритетами
	testPosts := []*Post{
		{
			// Пост с просроченным дедлайном — highest priority
			ID:        mustParsePostID("post-overdue"),
			Title:     "Overdue Post",
			Slug:      "overdue-post",
			Status:    StatusDraft,
			Deadline:  &pastDeadline,
		},
		{
			// Пост с будущим дедлайном
			ID:        mustParsePostID("post-with-deadline"),
			Title:     "Deadline Post",
			Slug:      "deadline-post",
			Status:    StatusIdea,
			Deadline:  &futureDeadline,
		},
		{
			// Пост со scheduled_at (просроченным)
			ID:          mustParsePostID("post-scheduled-past"),
			Title:       "Past Scheduled Post",
			Slug:        "past-scheduled-post",
			Status:      StatusReady,
			ScheduledAt: &pastScheduled,
		},
		{
			// Пост без дедлайна, статус ready
			ID:        mustParsePostID("post-ready"),
			Title:     "Ready Post",
			Slug:      "ready-post",
			Status:    StatusReady,
		},
		{
			// Пост без дедлайна, статус draft
			ID:        mustParsePostID("post-draft"),
			Title:     "Draft Post",
			Slug:      "draft-post",
			Status:    StatusDraft,
		},
		{
			// Пост без дедлайна, статус idea
			ID:        mustParsePostID("post-idea"),
			Title:     "Idea Post",
			Slug:      "idea-post",
			Status:    StatusIdea,
		},
		{
			// Опубликованный пост — должен быть исключён
			ID:        mustParsePostID("post-published"),
			Title:     "Published Post",
			Slug:      "published-post",
			Status:    StatusPublished,
		},
	}

	for _, post := range testPosts {
		repo.posts[post.ID] = post
	}

	t.Run("returns post with overdue deadline", func(t *testing.T) {
		nextPost, err := service.GetNextPost(context.Background())
		if err != nil {
			t.Fatalf("GetNextPost() unexpected error: %v", err)
		}

		if nextPost == nil {
			t.Fatalf("GetNextPost() expected post, got nil")
		}

		// Должен вернуться пост с просроченным дедлайном
		expectedID := mustParsePostID("post-overdue")
		if nextPost.ID != expectedID {
			t.Errorf("GetNextPost() ID = %s, expected post-overdue", nextPost.ID)
		}
	})

	t.Run("empty repository returns nil", func(t *testing.T) {
		emptyRepo := &mockRepository{
			posts: make(map[PostID]*Post),
		}
		emptyService := NewPostService(emptyRepo, clock)

		nextPost, err := emptyService.GetNextPost(context.Background())
		if err != nil {
			t.Fatalf("GetNextPost() unexpected error: %v", err)
		}

		if nextPost != nil {
			t.Errorf("GetNextPost() expected nil for empty repository, got %v", nextPost)
		}
	})

	t.Run("only published posts returns nil", func(t *testing.T) {
		publishedOnlyRepo := &mockRepository{
			posts: map[PostID]*Post{
				mustParsePostID("pub1"): {ID: mustParsePostID("pub1"), Title: "Pub 1", Slug: "pub-1", Status: StatusPublished},
				mustParsePostID("pub2"): {ID: mustParsePostID("pub2"), Title: "Pub 2", Slug: "pub-2", Status: StatusScheduled},
			},
		}
		publishedService := NewPostService(publishedOnlyRepo, clock)

		nextPost, err := publishedService.GetNextPost(context.Background())
		if err != nil {
			t.Fatalf("GetNextPost() unexpected error: %v", err)
		}

		if nextPost != nil {
			t.Errorf("GetNextPost() expected nil for only published posts, got %v", nextPost)
		}
	})

	t.Run("repository error", func(t *testing.T) {
		errorRepo := &mockRepository{
			posts:   make(map[PostID]*Post),
			listErr: errors.New("list error"),
		}
		errorService := NewPostService(errorRepo, clock)

		_, err := errorService.GetNextPost(context.Background())
		if err == nil {
			t.Errorf("GetNextPost() expected error, got nil")
		}
	})
}
