package cli

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
)

func TestDeletePost(t *testing.T) {
	// Создаём временную директорию для тестов
	tempDir := t.TempDir()

	// Создаём тестовый пост
	repo, err := fsrepo.NewFileSystemRepository(tempDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	testPost := &core.Post{
		ID:        "test-delete-id",
		Title:     "Test Delete Post",
		Slug:      "test-delete-post",
		Status:    core.StatusDraft,
		Platforms: []core.Platform{core.PlatformTelegram},
		Content:   "Test content",
	}

	ctx := context.Background()
	if err := repo.Create(ctx, testPost); err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}

	// Создаём сервис
	service := core.NewPostService(repo, core.SystemClock{})

	t.Run("delete existing post", func(t *testing.T) {
		err := service.DeletePost(ctx, testPost.ID)
		if err != nil {
			t.Errorf("DeletePost() unexpected error: %v", err)
		}

		// Проверяем, что пост удалён
		_, err = repo.GetByID(ctx, testPost.ID)
		if err == nil {
			t.Errorf("GetByID() after delete expected error, got nil")
		}
	})

	t.Run("delete non-existent post", func(t *testing.T) {
		err := service.DeletePost(ctx, "non-existent-id")
		if err == nil {
			t.Errorf("DeletePost() expected error, got nil")
		}
	})
}

func TestDeleteCommandIntegration(t *testing.T) {
	// Создаём временную директорию для тестов
	tempDir := t.TempDir()

	// Создаём конфиг
	cfg := config.NewDefaultConfig()
	cfg.PostsDir = tempDir
	configPath := filepath.Join(tempDir, ".jtpost.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Создаём репозиторий
	repo, err := fsrepo.NewFileSystemRepository(tempDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	testPost := &core.Post{
		ID:        "test-integration-id",
		Title:     "Test Integration Post",
		Slug:      "test-integration-post",
		Status:    core.StatusDraft,
		Platforms: []core.Platform{core.PlatformTelegram},
		Content:   "Test content",
	}

	ctx := context.Background()
	if err := repo.Create(ctx, testPost); err != nil {
		t.Fatalf("Failed to create test post: %v", err)
	}

	// Загружаем конфигурацию
	loadedCfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Создаём репозиторий из конфига
	repo2, err := fsrepo.NewFileSystemRepository(loadedCfg.PostsDir)
	if err != nil {
		t.Fatalf("Failed to create repository from config: %v", err)
	}

	service := core.NewPostService(repo2, core.SystemClock{})

	// Проверяем, что пост существует
	post, err := service.GetByID(ctx, testPost.ID)
	if err != nil {
		t.Fatalf("GetByID() expected post, got error: %v", err)
	}
	if post.Title != testPost.Title {
		t.Errorf("GetByID() title = %v, expected %v", post.Title, testPost.Title)
	}
}
