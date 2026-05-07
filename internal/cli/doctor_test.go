package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/jtprogru/jtpost/internal/adapters/config"
)

func TestCheckConfig_Missing(t *testing.T) {
	dir := t.TempDir()
	cfg, res := checkConfig(filepath.Join(dir, "nope.yaml"))
	if cfg != nil {
		t.Fatalf("ожидали nil cfg, получили %+v", cfg)
	}
	if res.level != levelFail {
		t.Fatalf("ожидали levelFail, получили %d (%s)", res.level, res.message)
	}
}

func TestCheckConfig_Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".jtpost.yaml")
	if err := os.WriteFile(path, []byte("posts_dir: content/posts\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, res := checkConfig(path)
	if cfg == nil {
		t.Fatal("ожидали загруженный cfg")
	}
	if res.level != levelOK {
		t.Fatalf("ожидали levelOK, получили %d", res.level)
	}
}

func TestCheckPostsDir(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		res := checkPostsDir(filepath.Join(t.TempDir(), "nope"))
		if res.level != levelFail {
			t.Fatalf("ожидали fail, получили %d", res.level)
		}
	})
	t.Run("ok", func(t *testing.T) {
		res := checkPostsDir(t.TempDir())
		if res.level != levelOK {
			t.Fatalf("ожидали ok, получили %d (%s)", res.level, res.message)
		}
	})
	t.Run("empty", func(t *testing.T) {
		res := checkPostsDir("")
		if res.level != levelFail {
			t.Fatalf("ожидали fail для пустой директории, получили %d", res.level)
		}
	})
}

// Тесты checkSQLite удалены: SQLite-проверка слилась с универсальной checkStorage,
// которая требует cfg + ctx. Покрытие через TestDoctor_Storage* (см. отдельные тесты при необходимости).

func TestCheckEditor(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	if res := checkEditor(); res.level != levelWarn {
		t.Fatalf("без env ожидали warn, получили %d", res.level)
	}
	t.Setenv("EDITOR", "nvim")
	if res := checkEditor(); res.level != levelOK {
		t.Fatalf("с EDITOR ожидали ok, получили %d", res.level)
	}
}

func TestCheckTelegram_NotConfigured(t *testing.T) {
	res := checkTelegram(context.Background(), config.TelegramConfig{})
	if res.level != levelWarn {
		t.Fatalf("без токена и chat_id ожидали warn, получили %d", res.level)
	}
}

func TestCheckTelegram_PartialConfig(t *testing.T) {
	res := checkTelegram(context.Background(), config.TelegramConfig{BotToken: "x"})
	if res.level != levelFail {
		t.Fatalf("без chat_id ожидали fail, получили %d", res.level)
	}
}

func TestRunDoctor_NoConfig(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	err := runDoctor(context.Background(), &buf, filepath.Join(dir, "nope.yaml"))
	if err == nil {
		t.Fatal("ожидали ошибку из-за отсутствующего конфига")
	}
	if !strings.Contains(buf.String(), "Конфигурация") {
		t.Fatalf("вывод не содержит проверку конфигурации: %s", buf.String())
	}
}

func TestRunDoctor_Healthy(t *testing.T) {
	dir := t.TempDir()
	postsDir := filepath.Join(dir, "posts")
	if err := os.Mkdir(postsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgPath := filepath.Join(dir, ".jtpost.yaml")
	cfgYAML := "posts_dir: " + postsDir + "\n"
	if err := os.WriteFile(cfgPath, []byte(cfgYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("EDITOR", "vim")

	var buf bytes.Buffer
	if err := runDoctor(context.Background(), &buf, cfgPath); err != nil {
		t.Fatalf("ожидали успех, получили ошибку: %v\nвывод: %s", err, buf.String())
	}
	if !strings.Contains(buf.String(), "Все проверки пройдены") {
		t.Fatalf("ожидали финальное сообщение об успехе, получили: %s", buf.String())
	}
}

func TestCheckGitRepo_NotARepo(t *testing.T) {
	cfg := config.NewDefaultConfig()
	cfg.PostsDir = t.TempDir()
	cfg.Storage.Git.Enabled = true
	results := checkGitRepo(cfg)
	if len(results) != 1 || results[0].level != levelFail {
		t.Fatalf("ожидали 1 fail, got %+v", results)
	}
}

func TestCheckGitRepo_CleanRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := initBareGitRepo(t, dir); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewDefaultConfig()
	cfg.PostsDir = dir
	cfg.Storage.Git.Enabled = true
	results := checkGitRepo(cfg)
	if len(results) < 1 || results[0].level != levelOK {
		t.Fatalf("ожидали clean ok, got %+v", results)
	}
	if !strings.Contains(results[0].message, "clean") {
		t.Errorf("message=%q, want contains 'clean'", results[0].message)
	}
}

func TestCheckGitRepo_DirtyRepo(t *testing.T) {
	dir := t.TempDir()
	if _, err := initBareGitRepo(t, dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "untracked.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.NewDefaultConfig()
	cfg.PostsDir = dir
	cfg.Storage.Git.Enabled = true
	results := checkGitRepo(cfg)
	if len(results) < 1 || results[0].level != levelWarn {
		t.Fatalf("ожидали dirty warn, got %+v", results)
	}
	if !strings.Contains(results[0].message, "dirty") {
		t.Errorf("message=%q, want contains 'dirty'", results[0].message)
	}
}

// initBareGitRepo инициализирует non-bare git-репо в dir для doctor-тестов.
func initBareGitRepo(t *testing.T, dir string) (string, error) {
	t.Helper()
	if _, err := git.PlainInit(dir, false); err != nil {
		return "", err
	}
	return dir, nil
}
