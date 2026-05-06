package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/jtprogru/jtpost/internal/core"
)

func writeFile(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func TestLoad_FileMissing(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "missing.yaml"))
	if !errors.Is(err, core.ErrConfigNotFound) {
		t.Fatalf("ожидали ErrConfigNotFound, получили %v", err)
	}
}

func TestLoad_YAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".jtpost.yaml", `
posts_dir: my/posts
telegram:
  bot_token: token-from-yaml
  chat_id: "-100123"
sqlite:
  dsn: custom.db
`)

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PostsDir != "my/posts" {
		t.Errorf("PostsDir=%q", cfg.PostsDir)
	}
	if cfg.Telegram.BotToken != "token-from-yaml" {
		t.Errorf("BotToken=%q", cfg.Telegram.BotToken)
	}
	if cfg.Telegram.ChatID != "-100123" {
		t.Errorf("ChatID=%q", cfg.Telegram.ChatID)
	}
	if cfg.SQLite.DSN != "custom.db" {
		t.Errorf("DSN=%q", cfg.SQLite.DSN)
	}
	// Defaults применяются для незаданных полей.
	if cfg.TemplatesDir != "templates" {
		t.Errorf("TemplatesDir=%q (ожидали templates)", cfg.TemplatesDir)
	}
}

func TestLoad_EnvOverridesYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".jtpost.yaml", `
posts_dir: from-yaml
telegram:
  bot_token: yaml-token
`)
	t.Setenv("JTPOST_POSTS_DIR", "from-env")
	t.Setenv("JTPOST_TELEGRAM_BOT_TOKEN", "env-token")
	t.Setenv("JTPOST_TELEGRAM_CHAT_ID", "-100999")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.PostsDir != "from-env" {
		t.Errorf("PostsDir=%q (ожидали from-env)", cfg.PostsDir)
	}
	if cfg.Telegram.BotToken != "env-token" {
		t.Errorf("BotToken=%q (ожидали env-token)", cfg.Telegram.BotToken)
	}
	if cfg.Telegram.ChatID != "-100999" {
		t.Errorf("ChatID=%q", cfg.Telegram.ChatID)
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeFile(t, dir, ".jtpost.yaml", "::: not yaml :::\n  -\n -")
	_, err := Load(path)
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Fatalf("ожидали ErrConfigInvalid, получили %v", err)
	}
}

func TestLoadWithDefaults_FileMissing(t *testing.T) {
	t.Setenv("JTPOST_POSTS_DIR", "env-only")
	cfg, err := LoadWithDefaults(filepath.Join(t.TempDir(), "missing.yaml"))
	if err != nil {
		t.Fatalf("LoadWithDefaults: %v", err)
	}
	if cfg.PostsDir != "env-only" {
		t.Errorf("PostsDir=%q", cfg.PostsDir)
	}
	if cfg.SQLite.DSN != ".jtpost.db" {
		t.Errorf("DSN=%q (ожидали дефолт)", cfg.SQLite.DSN)
	}
}

func TestSaveAndLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".jtpost.yaml")

	orig := NewDefaultConfig()
	orig.Telegram.BotToken = "abc"
	orig.Telegram.ChatID = "-100"
	if err := orig.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.Telegram.BotToken != "abc" || loaded.Telegram.ChatID != "-100" {
		t.Errorf("round-trip mismatch: %+v", loaded.Telegram)
	}
	if loaded.PostsDir != orig.PostsDir {
		t.Errorf("PostsDir mismatch")
	}
}

func TestValidate(t *testing.T) {
	cfg := NewDefaultConfig()
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config invalid: %v", err)
	}
	cfg.PostsDir = ""
	if err := cfg.Validate(); !errors.Is(err, core.ErrPostsDirNotFound) {
		t.Errorf("ожидали ErrPostsDirNotFound, получили %v", err)
	}
}
