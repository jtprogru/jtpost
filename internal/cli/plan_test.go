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

func TestPlanCommand(t *testing.T) {
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
	tomorrow := now.Add(24 * time.Hour)
	nextWeek := now.Add(7 * 24 * time.Hour)
	nextMonth := now.Add(30 * 24 * time.Hour)

	testPosts := []*core.Post{
		{
			ID:          "post-1",
			Title:       "Tomorrow Deadline",
			Slug:        "tomorrow-deadline",
			Status:      core.StatusDraft,
			Platforms:   []core.Platform{core.PlatformTelegram},
			Tags:        []string{"go", "tutorial"},
			Content:     "Content 1",
			Deadline:    &tomorrow,
		},
		{
			ID:          "post-2",
			Title:       "Next Week Scheduled",
			Slug:        "next-week-scheduled",
			Status:      core.StatusReady,
			Platforms:   []core.Platform{core.PlatformTelegram},
			Tags:        []string{"go", "cli"},
			Content:     "Content 2",
			ScheduledAt: &nextWeek,
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
		{
			ID:        "post-4",
			Title:     "No Date Post",
			Slug:      "no-date-post",
			Status:    core.StatusDraft,
			Platforms: []core.Platform{core.PlatformTelegram},
			Tags:      []string{"draft"},
			Content:   "Content 4",
		},
		{
			ID:          "post-5",
			Title:       "Next Month Deadline",
			Slug:        "next-month-deadline",
			Status:      core.StatusIdea,
			Platforms:   []core.Platform{core.PlatformTelegram},
			Tags:        []string{"idea"},
			Content:     "Content 5",
			Deadline:    &nextMonth,
		},
	}

	ctx := context.Background()
	for _, post := range testPosts {
		if err := repo.Create(ctx, post); err != nil {
			t.Fatalf("Failed to create test post: %v", err)
		}
	}

	t.Run("plan command default days", func(t *testing.T) {
		// Перехватываем stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		rootCmd.SetArgs([]string{"plan", "-c", configPath})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("plan command failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Проверяем наличие заголовка таблицы
		if !strings.Contains(output, "DATE") {
			t.Errorf("Expected 'DATE' header in output, got: %s", output)
		}
		if !strings.Contains(output, "TYPE") {
			t.Errorf("Expected 'TYPE' header in output")
		}

		// Проверяем, что опубликованный пост не попал в план
		if strings.Contains(output, "Published Post") {
			t.Errorf("Published post should not be in plan")
		}

		// Проверяем, что пост без даты не попал в план
		if strings.Contains(output, "No Date Post") {
			t.Errorf("Post without date should not be in plan")
		}

		// Проверяем наличие постов с дедлайнами
		if !strings.Contains(output, "Tomorrow Deadline") {
			t.Errorf("Expected 'Tomorrow Deadline' in plan")
		}
		if !strings.Contains(output, "Next Week Scheduled") {
			t.Errorf("Expected 'Next Week Scheduled' in plan")
		}
	})

	t.Run("plan command custom days", func(t *testing.T) {
		// Перехватываем stdout
		r, w, _ := os.Pipe()
		oldStdout := os.Stdout
		os.Stdout = w

		// План только на 2 дня — должен попасть только завтрашний дедлайн
		rootCmd.SetArgs([]string{"plan", "-c", configPath, "-d", "2"})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("plan command with custom days failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		// Проверяем, что попал только пост с завтрашним дедлайном
		if !strings.Contains(output, "Tomorrow Deadline") {
			t.Errorf("Expected 'Tomorrow Deadline' in plan")
		}
		if strings.Contains(output, "Next Week Scheduled") {
			t.Errorf("Next week post should not be in 2-day plan")
		}
	})

	t.Run("plan command empty repository", func(t *testing.T) {
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

		rootCmd.SetArgs([]string{"plan", "-c", emptyConfigPath})

		if err := rootCmd.Execute(); err != nil {
			w.Close()
			os.Stdout = oldStdout
			t.Errorf("plan command with empty repository failed: %v", err)
		}

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Нет запланированных постов") {
			t.Errorf("Expected 'Нет запланированных постов' in output, got: %s", output)
		}
	})
}

func TestPlanOutput(t *testing.T) {
	now := time.Now()

	t.Run("printPlan empty", func(t *testing.T) {
		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printPlan([]*plannedPost{})

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Нет запланированных постов") {
			t.Errorf("Expected 'Нет запланированных постов' in output")
		}
	})

	t.Run("printPlan with posts", func(t *testing.T) {
		deadline := now.Add(24 * time.Hour)
		posts := []*plannedPost{
			{
				Post: &core.Post{
					ID:        "test-1",
					Title:     "Test Post 1",
					Slug:      "test-post-1",
					Status:    core.StatusDraft,
					Platforms: []core.Platform{core.PlatformTelegram},
				},
				Date:     deadline,
				DateType: "deadline",
			},
			{
				Post: &core.Post{
					ID:        "test-2",
					Title:     "Test Post 2",
					Slug:      "test-post-2",
					Status:    core.StatusReady,
					Platforms: []core.Platform{core.PlatformTelegram},
				},
				Date:     deadline,
				DateType: "schedule",
			},
		}

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		printPlan(posts)

		w.Close()
		os.Stdout = oldStdout

		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		output := buf.String()

		if !strings.Contains(output, "Test Post 1") {
			t.Errorf("Expected 'Test Post 1' in output")
		}
		if !strings.Contains(output, "Test Post 2") {
			t.Errorf("Expected 'Test Post 2' in output")
		}
		if !strings.Contains(output, "Всего постов в плане: 2") {
			t.Errorf("Expected total count in output")
		}
	})
}

func TestSortByDate(t *testing.T) {
	now := time.Now()
	date1 := now.Add(24 * time.Hour)
	date2 := now.Add(48 * time.Hour)
	date3 := now.Add(12 * time.Hour)

	posts := []*plannedPost{
		{Post: &core.Post{ID: "1"}, Date: date1},
		{Post: &core.Post{ID: "2"}, Date: date2},
		{Post: &core.Post{ID: "3"}, Date: date3},
	}

	sortByDate(posts)

	// Проверяем, что отсортировалось по возрастанию даты
	if posts[0].Date != date3 {
		t.Errorf("Expected first post to have earliest date")
	}
	if posts[1].Date != date1 {
		t.Errorf("Expected second post to have middle date")
	}
	if posts[2].Date != date2 {
		t.Errorf("Expected third post to have latest date")
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{"short string", "Hello", 10, "Hello"},
		{"exact length", "Hello", 5, "Hello"},
		{"long string", "Hello World", 8, "Hello..."},
		{"empty string", "", 5, ""},
		{"zero max len", "Hello", 0, "..."},
		{"small max len", "Hello", 3, "..."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, result, tt.expected)
			}
		})
	}
}
