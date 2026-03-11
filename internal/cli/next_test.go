package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
)

func TestNextCommand(t *testing.T) {
	// Создаём временную директорию для тестов
	tempDir := t.TempDir()

	// Создаём конфиг
	cfg := config.NewDefaultConfig()
	cfg.PostsDir = tempDir
	configPath := filepath.Join(tempDir, ".jtpost.yaml")
	if err := cfg.Save(configPath); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Создаём репозиторий с тестовыми постами
	repo, err := fsrepo.NewFileSystemRepository(tempDir)
	if err != nil {
		t.Fatalf("Failed to create repository: %v", err)
	}

	now := time.Now()
	pastDeadline := now.Add(-24 * time.Hour)

	testPosts := []*core.Post{
		{
			ID:        "post-1",
			Title:     "Overdue Post",
			Slug:      "overdue-post",
			Status:    core.StatusDraft,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"go", "tutorial"},
			Content:   "Content 1",
			Deadline:  &pastDeadline,
		},
		{
			ID:        "post-2",
			Title:     "Draft Post",
			Slug:      "draft-post",
			Status:    core.StatusDraft,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"go"},
			Content:   "Content 2",
		},
		{
			ID:        "post-3",
			Title:     "Published Post",
			Slug:      "published-post",
			Status:    core.StatusPublished,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"news"},
			Content:   "Content 3",
		},
	}

	ctx := context.Background()
	for _, post := range testPosts {
		if err := repo.Create(ctx, post); err != nil {
			t.Fatalf("Failed to create test post: %v", err)
		}
	}

	t.Run("next command full format", func(t *testing.T) {
		// Перехватываем stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		rootCmd.SetArgs([]string{"next", "-c", configPath, "-f", "full"})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("next command failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Проверяем наличие заголовка
		if !strings.Contains(output, "Рекомендуемый пост для работы") {
			t.Errorf("Expected header in output, got: %s", output)
		}

		// Проверяем, что вернулся пост с просроченным дедлайном
		if !strings.Contains(output, "Overdue Post") {
			t.Errorf("Expected 'Overdue Post' in output, got: %s", output)
		}
		if !strings.Contains(output, "overdue-post") {
			t.Errorf("Expected 'overdue-post' slug in output")
		}
	})

	t.Run("next command plain format", func(t *testing.T) {
		// Перехватываем stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		rootCmd.SetArgs([]string{"next", "-c", configPath, "-f", "plain"})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("next command failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Проверяем, что вернулся только slug
		expected := "overdue-post"
		if !strings.Contains(output, expected) {
			t.Errorf("Expected '%s' in output, got: %s", expected, output)
		}
	})

	t.Run("next command json format", func(t *testing.T) {
		// Перехватываем stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		rootCmd.SetArgs([]string{"next", "-c", configPath, "-f", "json"})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("next command failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Проверяем JSON структуру
		if !strings.Contains(output, `"slug": "overdue-post"`) {
			t.Errorf("Expected slug in JSON output, got: %s", output)
		}
		if !strings.Contains(output, `"title": "Overdue Post"`) {
			t.Errorf("Expected title in JSON output, got: %s", output)
		}
	})

	t.Run("next command empty repository", func(t *testing.T) {
		// Создаём пустую директорию
		emptyDir := t.TempDir()
		emptyCfg := config.NewDefaultConfig()
		emptyCfg.PostsDir = emptyDir
		emptyConfigPath := filepath.Join(emptyDir, ".jtpost.yaml")
		if err := emptyCfg.Save(emptyConfigPath); err != nil {
			t.Fatalf("Failed to save config: %v", err)
		}

		// Перехватываем stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		rootCmd.SetArgs([]string{"next", "-c", emptyConfigPath})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("next command failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Нет постов для рекомендации") {
			t.Errorf("Expected 'Нет постов для рекомендации' in output, got: %s", output)
		}
	})
}

func TestNextOutputFormats(t *testing.T) {
	now := time.Now()

	t.Run("printNextFull", func(t *testing.T) {
		post := &core.Post{
			ID:        "test-id",
			Title:     "Test Post",
			Slug:      "test-post",
			Status:    core.StatusDraft,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"go", "test"},
			Deadline:  &now,
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printNextFull(post)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Test Post") {
			t.Errorf("Expected title in output")
		}
		if !strings.Contains(output, "test-post") {
			t.Errorf("Expected slug in output")
		}
		if !strings.Contains(output, "draft") {
			t.Errorf("Expected status in output")
		}
	})

	t.Run("printNextJSON", func(t *testing.T) {
		post := &core.Post{
			ID:        "test-id",
			Title:     "Test Post",
			Slug:      "test-post",
			Status:    core.StatusDraft,
			Platforms: []core.Platform{core.PlatformTelegram},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := printNextJSON(post)

		w.Close()
		os.Stdout = oldStdout

		if err != nil {
			t.Fatalf("printNextJSON failed: %v", err)
		}

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, `"slug": "test-post"`) {
			t.Errorf("Expected slug in JSON output")
		}
	})
}
