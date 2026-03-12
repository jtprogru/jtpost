package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
)

func TestSQLitePostRepository_CreateAndGetByID(t *testing.T) {
	// Создаём временный файл БД
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.db"

	repo, err := NewSQLitePostRepository(Config{DSN: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Создаём тестовый пост
	deadline := time.Now().Add(24 * time.Hour)
	post := &core.Post{
		ID:       "test-id-1",
		Title:    "Тестовый пост",
		Slug:     "test-post",
		Status:   core.StatusDraft,
		Platforms: []core.Platform{core.PlatformTelegram},
		Tags:     []string{"go", "test"},
		Deadline: &deadline,
		Content:  "Содержимое поста",
	}

	// Создаём
	if err := repo.Create(ctx, post); err != nil {
		t.Fatal(err)
	}

	// Получаем
	got, err := repo.GetByID(ctx, post.ID)
	if err != nil {
		t.Fatal(err)
	}

	// Проверяем
	if got.Title != post.Title {
		t.Errorf("Title = %v, want %v", got.Title, post.Title)
	}
	if got.Slug != post.Slug {
		t.Errorf("Slug = %v, want %v", got.Slug, post.Slug)
	}
	if got.Status != post.Status {
		t.Errorf("Status = %v, want %v", got.Status, post.Status)
	}
	if len(got.Tags) != len(post.Tags) {
		t.Errorf("Tags length = %v, want %v", len(got.Tags), len(post.Tags))
	}
}

func TestSQLitePostRepository_GetBySlug(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.db"

	repo, err := NewSQLitePostRepository(Config{DSN: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()

	post := &core.Post{
		ID:      "test-id-2",
		Title:   "Пост для slug",
		Slug:    "my-slug",
		Status:  core.StatusReady,
		Content: "Текст",
	}

	if err := repo.Create(ctx, post); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetBySlug(ctx, post.Slug)
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != post.ID {
		t.Errorf("ID = %v, want %v", got.ID, post.ID)
	}
}

func TestSQLitePostRepository_List(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.db"

	repo, err := NewSQLitePostRepository(Config{DSN: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Создаём несколько постов
	posts := []*core.Post{
		{ID: "1", Title: "Post 1", Slug: "post-1", Status: core.StatusDraft, Tags: []string{"go"}, Content: "Content 1"},
		{ID: "2", Title: "Post 2", Slug: "post-2", Status: core.StatusReady, Tags: []string{"telegram"}, Content: "Content 2"},
		{ID: "3", Title: "Post 3", Slug: "post-3", Status: core.StatusDraft, Tags: []string{"go", "cli"}, Content: "Content 3"},
	}

	for _, p := range posts {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("no filter", func(t *testing.T) {
		got, err := repo.List(ctx, core.PostFilter{})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 3 {
			t.Errorf("len(posts) = %v, want 3", len(got))
		}
	})

	t.Run("filter by status", func(t *testing.T) {
		got, err := repo.List(ctx, core.PostFilter{Statuses: []core.PostStatus{core.StatusDraft}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("len(posts) = %v, want 2", len(got))
		}
	})

	t.Run("filter by tag", func(t *testing.T) {
		got, err := repo.List(ctx, core.PostFilter{Tags: []string{"go"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Errorf("len(posts) = %v, want 2", len(got))
		}
	})

	t.Run("filter by search", func(t *testing.T) {
		got, err := repo.List(ctx, core.PostFilter{Search: "Post 1"})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 1 {
			t.Errorf("len(posts) = %v, want 1", len(got))
		}
	})
}

func TestSQLitePostRepository_Update(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.db"

	repo, err := NewSQLitePostRepository(Config{DSN: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()

	post := &core.Post{
		ID:      "update-id",
		Title:   "Old Title",
		Slug:    "old-slug",
		Status:  core.StatusDraft,
		Content: "Old content",
	}

	if err := repo.Create(ctx, post); err != nil {
		t.Fatal(err)
	}

	// Обновляем
	post.Title = "New Title"
	post.Status = core.StatusReady
	post.Content = "New content"

	if err := repo.Update(ctx, post); err != nil {
		t.Fatal(err)
	}

	got, err := repo.GetByID(ctx, post.ID)
	if err != nil {
		t.Fatal(err)
	}

	if got.Title != "New Title" {
		t.Errorf("Title = %v, want New Title", got.Title)
	}
	if got.Status != core.StatusReady {
		t.Errorf("Status = %v, want ready", got.Status)
	}
}

func TestSQLitePostRepository_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.db"

	repo, err := NewSQLitePostRepository(Config{DSN: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()

	post := &core.Post{
		ID:      "delete-id",
		Title:   "ToDelete",
		Slug:    "to-delete",
		Status:  core.StatusDraft,
		Content: "Content",
	}

	if err := repo.Create(ctx, post); err != nil {
		t.Fatal(err)
	}

	if err := repo.Delete(ctx, post.ID); err != nil {
		t.Fatal(err)
	}

	_, err = repo.GetByID(ctx, post.ID)
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestSQLitePostRepository_ImportPosts(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.db"

	repo, err := NewSQLitePostRepository(Config{DSN: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Создаём посты для импорта
	posts := []*core.Post{
		{ID: "imp-1", Title: "Import 1", Slug: "import-1", Status: core.StatusDraft, Content: "Content 1"},
		{ID: "imp-2", Title: "Import 2", Slug: "import-2", Status: core.StatusReady, Content: "Content 2"},
	}

	if err := repo.ImportPosts(ctx, posts); err != nil {
		t.Fatal(err)
	}

	// Проверяем количество
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %v, want 2", count)
	}

	// Проверяем данные
	got, err := repo.GetByID(ctx, "imp-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Import 1" {
		t.Errorf("Title = %v, want Import 1", got.Title)
	}
}

func TestSQLitePostRepository_Count(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.db"

	repo, err := NewSQLitePostRepository(Config{DSN: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()

	// Пустая БД
	count, err := repo.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("count = %v, want 0", count)
	}

	// Добавляем посты
	posts := []*core.Post{
		{ID: "c1", Title: "C1", Slug: "c1", Status: core.StatusDraft, Content: "C"},
		{ID: "c2", Title: "C2", Slug: "c2", Status: core.StatusDraft, Content: "C"},
	}

	for _, p := range posts {
		if err := repo.Create(ctx, p); err != nil {
			t.Fatal(err)
		}
	}

	count, err = repo.Count(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Errorf("count = %v, want 2", count)
	}
}

func TestSQLitePostRepository_GetByID_NotFound(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/test.db"

	repo, err := NewSQLitePostRepository(Config{DSN: tmpFile})
	if err != nil {
		t.Fatal(err)
	}
	defer repo.Close()

	ctx := context.Background()

	_, err = repo.GetByID(ctx, "nonexistent")
	if !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}
