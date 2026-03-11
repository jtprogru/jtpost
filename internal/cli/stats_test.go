package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jtprogru/jtpost/internal/adapters/config"
	"github.com/jtprogru/jtpost/internal/adapters/fsrepo"
	"github.com/jtprogru/jtpost/internal/core"
)

func TestStatsCommand(t *testing.T) {
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

	testPosts := []*core.Post{
		{
			ID:        "post-1",
			Title:     "Draft Post 1",
			Slug:      "draft-post-1",
			Status:    core.StatusDraft,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"go", "tutorial"},
			Content:   "Content 1",
		},
		{
			ID:        "post-2",
			Title:     "Draft Post 2",
			Slug:      "draft-post-2",
			Status:    core.StatusDraft,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"go", "cli"},
			Content:   "Content 2",
		},
		{
			ID:        "post-3",
			Title:     "Ready Post",
			Slug:      "ready-post",
			Status:    core.StatusReady,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"go", "news"},
			Content:   "Content 3",
		},
		{
			ID:        "post-4",
			Title:     "Published Post",
			Slug:      "published-post",
			Status:    core.StatusPublished,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"tutorial"},
			Content:   "Content 4",
		},
	}

	ctx := context.Background()
	for _, post := range testPosts {
		if err := repo.Create(ctx, post); err != nil {
			t.Fatalf("Failed to create test post: %v", err)
		}
	}

	t.Run("stats command table format", func(t *testing.T) {
		// Перехватываем stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		// Создаём корневую команду с аргументами
		rootCmd.SetArgs([]string{"stats", "-c", configPath})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("stats command failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Проверяем наличие заголовка
		if !strings.Contains(output, "Статистика постов") {
			t.Errorf("Expected 'Статистика постов' in output, got: %s", output)
		}

		// Проверяем общее количество
		if !strings.Contains(output, "Всего постов: 4") {
			t.Errorf("Expected 'Всего постов: 4' in output, got: %s", output)
		}

		// Проверяем статусы
		if !strings.Contains(output, "draft") {
			t.Errorf("Expected 'draft' status in output")
		}
		if !strings.Contains(output, "ready") {
			t.Errorf("Expected 'ready' status in output")
		}
		if !strings.Contains(output, "published") {
			t.Errorf("Expected 'published' status in output")
		}

		// Проверяем платформы
		if !strings.Contains(output, "telegram") {
			t.Errorf("Expected 'telegram' platform in output")
		}

		// Проверяем теги
		if !strings.Contains(output, "go") {
			t.Errorf("Expected 'go' tag in output")
		}
		if !strings.Contains(output, "tutorial") {
			t.Errorf("Expected 'tutorial' tag in output")
		}
	})

	t.Run("stats command json format", func(t *testing.T) {
		// Перехватываем stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		// Сбрасываем аргументы и устанавливаем новые
		rootCmd.SetArgs([]string{"stats", "-c", configPath, "-f", "json"})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("stats command with JSON format failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Проверяем JSON структуру
		if !strings.Contains(output, `"total": 4`) {
			t.Errorf("Expected '\"total\": 4' in JSON output, got: %s", output)
		}
		if !strings.Contains(output, `"by_status"`) {
			t.Errorf("Expected 'by_status' in JSON output")
		}
		if !strings.Contains(output, `"by_platform"`) {
			t.Errorf("Expected 'by_platform' in JSON output")
		}
		if !strings.Contains(output, `"by_tag"`) {
			t.Errorf("Expected 'by_tag' in JSON output")
		}
	})

	t.Run("stats command empty repository", func(t *testing.T) {
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

		// Сбрасываем аргументы и устанавливаем новые (явно указываем table формат)
		rootCmd.SetArgs([]string{"stats", "-c", emptyConfigPath, "-f", "table"})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("stats command with empty repository failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Всего постов: 0") {
			t.Errorf("Expected 'Всего постов: 0' in output, got: %s", output)
		}
	})
}

func TestStatsOutputFormats(t *testing.T) {
	t.Run("printStatsTable empty stats", func(t *testing.T) {
		stats := &core.PostStats{
			Total:      0,
			ByStatus:   make(map[core.PostStatus]int),
			ByPlatform: make(map[core.Platform]int),
			ByTag:      make(map[string]int),
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printStatsTable(stats)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Статистика постов") {
			t.Errorf("Expected header in output")
		}
		if !strings.Contains(output, "Всего постов: 0") {
			t.Errorf("Expected 'Всего постов: 0' in output")
		}
	})

	t.Run("printStatsTable with data", func(t *testing.T) {
		stats := &core.PostStats{
			Total: 5,
			ByStatus: map[core.PostStatus]int{
				core.StatusDraft:     2,
				core.StatusReady:     2,
				core.StatusPublished: 1,
			},
			ByPlatform: map[core.Platform]int{
				core.PlatformTelegram: 5,
			},
			ByTag: map[string]int{
				"go":  3,
				"cli": 2,
			},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printStatsTable(stats)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Всего постов: 5") {
			t.Errorf("Expected 'Всего постов: 5' in output")
		}
		if !strings.Contains(output, "draft") {
			t.Errorf("Expected 'draft' in output")
		}
		if !strings.Contains(output, "telegram") {
			t.Errorf("Expected 'telegram' in output")
		}
	})
}

func TestStatsJSONOutput(t *testing.T) {
	stats := &core.PostStats{
		Total: 3,
		ByStatus: map[core.PostStatus]int{
			core.StatusDraft: 2,
			core.StatusReady: 1,
		},
		ByPlatform: map[core.Platform]int{
			core.PlatformTelegram: 3,
		},
		ByTag: map[string]int{
			"go":  2,
			"cli": 1,
		},
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := printStatsJSON(stats)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("printStatsJSON failed: %v", err)
	}

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, `"total": 3`) {
		t.Errorf("Expected '\"total\": 3' in JSON output")
	}
	if !strings.Contains(output, `"by_status"`) {
		t.Errorf("Expected 'by_status' in JSON output")
	}
}
