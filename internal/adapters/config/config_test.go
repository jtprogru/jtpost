package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

// validUUIDs возвращает пару валидных UUID для тестов Validate.
func validUUIDs(t *testing.T) (uuid.UUID, uuid.UUID) {
	t.Helper()
	tenant, err := uuid.Parse("01900000-0000-7000-8000-000000000001")
	if err != nil {
		t.Fatalf("parse tenant uuid: %v", err)
	}
	author, err := uuid.Parse("01900000-0000-7000-8000-000000000002")
	if err != nil {
		t.Fatalf("parse author uuid: %v", err)
	}
	return tenant, author
}

func writeFile(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, ".jtpost.yaml")
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
	path := writeFile(t, dir, `
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
	path := writeFile(t, dir, `
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
	path := writeFile(t, dir, "::: not yaml :::\n  -\n -")
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
	tenant, author := validUUIDs(t)
	cfg := NewDefaultConfig()
	cfg.Auth.TenantDefault = tenant
	cfg.Auth.AuthorDefault = author
	if err := cfg.Validate(); err != nil {
		t.Errorf("default config invalid: %v", err)
	}
	cfg.PostsDir = ""
	if err := cfg.Validate(); !errors.Is(err, core.ErrPostsDirNotFound) {
		t.Errorf("ожидали ErrPostsDirNotFound, получили %v", err)
	}
}

func TestConfig_LoadDefaults(t *testing.T) {
	cfg, err := LoadWithDefaults("")
	if err != nil {
		t.Fatalf("LoadWithDefaults: %v", err)
	}
	if cfg.Storage.Type != "fs" {
		t.Errorf("Storage.Type=%q, want fs", cfg.Storage.Type)
	}
	if cfg.Server.Port != 8080 {
		t.Errorf("Server.Port=%d, want 8080", cfg.Server.Port)
	}
	if cfg.Server.Addr != "localhost" {
		t.Errorf("Server.Addr=%q, want localhost", cfg.Server.Addr)
	}
	if cfg.Worker.Interval != time.Minute {
		t.Errorf("Worker.Interval=%v, want 1m", cfg.Worker.Interval)
	}
	if cfg.Worker.MaxRetries != 3 {
		t.Errorf("Worker.MaxRetries=%d, want 3", cfg.Worker.MaxRetries)
	}
	if cfg.Worker.RetryBackoff != 30*time.Second {
		t.Errorf("Worker.RetryBackoff=%v, want 30s", cfg.Worker.RetryBackoff)
	}
	if cfg.Storage.Postgres.MaxOpenConns != 10 {
		t.Errorf("Postgres.MaxOpenConns=%d, want 10", cfg.Storage.Postgres.MaxOpenConns)
	}
	if cfg.Storage.Postgres.MaxIdleConns != 5 {
		t.Errorf("Postgres.MaxIdleConns=%d, want 5", cfg.Storage.Postgres.MaxIdleConns)
	}
	if cfg.Storage.Postgres.ConnMaxLifetime != 30*time.Minute {
		t.Errorf("Postgres.ConnMaxLifetime=%v, want 30m", cfg.Storage.Postgres.ConnMaxLifetime)
	}
	if cfg.Storage.Git.Branch != "main" {
		t.Errorf("Storage.Git.Branch=%q, want main", cfg.Storage.Git.Branch)
	}
	if !cfg.Storage.Git.AutoCommit {
		t.Errorf("Storage.Git.AutoCommit=false, want true")
	}
	if cfg.Auth.Type != "none" {
		t.Errorf("Auth.Type=%q, want none", cfg.Auth.Type)
	}
	if cfg.Auth.TokenTTL != 24*time.Hour {
		t.Errorf("Auth.TokenTTL=%v, want 24h", cfg.Auth.TokenTTL)
	}
}

func TestConfig_LoadFromYAML(t *testing.T) {
	dir := t.TempDir()
	body := `
posts_dir: my/posts
storage:
  type: postgres
  git:
    enabled: true
    auto_commit: false
    auto_push: true
    remote: origin
    branch: develop
    commit_template: "post: {{.Slug}}"
  sqlite:
    dsn: storage.db
  postgres:
    dsn: postgres://user:pass@localhost/db
    max_open_conns: 20
    max_idle_conns: 8
    conn_max_lifetime: 1h
auth:
  type: oauth
  secret: s3cret
  tenant_default: "01900000-0000-7000-8000-000000000011"
  author_default: "01900000-0000-7000-8000-000000000022"
  token_ttl: 12h
  oauth:
    provider: github
    client_id: cid
    client_secret: csec
    redirect_url: https://example.com/cb
worker:
  enabled: true
  interval: 2m
  max_retries: 7
  retry_backoff: 45s
server:
  addr: 0.0.0.0
  port: 9090
  base_url: https://app.example.com
  read_timeout: 20s
  write_timeout: 25s
`
	path := writeFile(t, dir, body)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Storage.Type != "postgres" {
		t.Errorf("Storage.Type=%q", cfg.Storage.Type)
	}
	if !cfg.Storage.Git.Enabled || cfg.Storage.Git.AutoCommit || !cfg.Storage.Git.AutoPush {
		t.Errorf("Storage.Git=%+v", cfg.Storage.Git)
	}
	if cfg.Storage.Git.Remote != "origin" || cfg.Storage.Git.Branch != "develop" {
		t.Errorf("Storage.Git remote/branch wrong: %+v", cfg.Storage.Git)
	}
	if cfg.Storage.Git.CommitTemplate != "post: {{.Slug}}" {
		t.Errorf("Storage.Git.CommitTemplate=%q", cfg.Storage.Git.CommitTemplate)
	}
	if cfg.Storage.SQLite.DSN != "storage.db" {
		t.Errorf("Storage.SQLite.DSN=%q", cfg.Storage.SQLite.DSN)
	}
	if cfg.Storage.Postgres.DSN != "postgres://user:pass@localhost/db" {
		t.Errorf("Storage.Postgres.DSN=%q", cfg.Storage.Postgres.DSN)
	}
	if cfg.Storage.Postgres.MaxOpenConns != 20 || cfg.Storage.Postgres.MaxIdleConns != 8 {
		t.Errorf("Storage.Postgres conns: %+v", cfg.Storage.Postgres)
	}
	if cfg.Storage.Postgres.ConnMaxLifetime != time.Hour {
		t.Errorf("Storage.Postgres.ConnMaxLifetime=%v", cfg.Storage.Postgres.ConnMaxLifetime)
	}
	if cfg.Auth.Type != "oauth" || cfg.Auth.Secret != "s3cret" {
		t.Errorf("Auth=%+v", cfg.Auth)
	}
	wantTenant := uuid.MustParse("01900000-0000-7000-8000-000000000011")
	wantAuthor := uuid.MustParse("01900000-0000-7000-8000-000000000022")
	if cfg.Auth.TenantDefault != wantTenant {
		t.Errorf("Auth.TenantDefault=%v", cfg.Auth.TenantDefault)
	}
	if cfg.Auth.AuthorDefault != wantAuthor {
		t.Errorf("Auth.AuthorDefault=%v", cfg.Auth.AuthorDefault)
	}
	if cfg.Auth.TokenTTL != 12*time.Hour {
		t.Errorf("Auth.TokenTTL=%v", cfg.Auth.TokenTTL)
	}
	if cfg.Auth.OAuth.Provider != "github" || cfg.Auth.OAuth.ClientID != "cid" {
		t.Errorf("Auth.OAuth=%+v", cfg.Auth.OAuth)
	}
	if cfg.Auth.OAuth.ClientSecret != "csec" || cfg.Auth.OAuth.RedirectURL != "https://example.com/cb" {
		t.Errorf("Auth.OAuth=%+v", cfg.Auth.OAuth)
	}
	if !cfg.Worker.Enabled || cfg.Worker.Interval != 2*time.Minute {
		t.Errorf("Worker=%+v", cfg.Worker)
	}
	if cfg.Worker.MaxRetries != 7 || cfg.Worker.RetryBackoff != 45*time.Second {
		t.Errorf("Worker=%+v", cfg.Worker)
	}
	if cfg.Server.Addr != "0.0.0.0" || cfg.Server.Port != 9090 {
		t.Errorf("Server=%+v", cfg.Server)
	}
	if cfg.Server.BaseURL != "https://app.example.com" {
		t.Errorf("Server.BaseURL=%q", cfg.Server.BaseURL)
	}
	if cfg.Server.ReadTimeout != 20*time.Second || cfg.Server.WriteTimeout != 25*time.Second {
		t.Errorf("Server timeouts: %+v", cfg.Server)
	}
}

func TestConfig_EnvOverride(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
		envVal string
		check  func(t *testing.T, cfg *Config)
	}{
		{
			name:   "storage_type",
			envKey: "JTPOST_STORAGE_TYPE",
			envVal: "postgres",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if c.Storage.Type != "postgres" {
					t.Errorf("Storage.Type=%q", c.Storage.Type)
				}
			},
		},
		{
			name:   "storage_git_enabled",
			envKey: "JTPOST_STORAGE_GIT_ENABLED",
			envVal: "true",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if !c.Storage.Git.Enabled {
					t.Errorf("Storage.Git.Enabled=false")
				}
			},
		},
		{
			name:   "auth_type",
			envKey: "JTPOST_AUTH_TYPE",
			envVal: "oauth",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if c.Auth.Type != "oauth" {
					t.Errorf("Auth.Type=%q", c.Auth.Type)
				}
			},
		},
		{
			name:   "auth_tenant_default",
			envKey: "JTPOST_AUTH_TENANT_DEFAULT",
			envVal: "01900000-0000-7000-8000-000000000001",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				want := uuid.MustParse("01900000-0000-7000-8000-000000000001")
				if c.Auth.TenantDefault != want {
					t.Errorf("Auth.TenantDefault=%v", c.Auth.TenantDefault)
				}
			},
		},
		{
			name:   "auth_author_default",
			envKey: "JTPOST_AUTH_AUTHOR_DEFAULT",
			envVal: "01900000-0000-7000-8000-000000000002",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				want := uuid.MustParse("01900000-0000-7000-8000-000000000002")
				if c.Auth.AuthorDefault != want {
					t.Errorf("Auth.AuthorDefault=%v", c.Auth.AuthorDefault)
				}
			},
		},
		{
			name:   "worker_interval",
			envKey: "JTPOST_WORKER_INTERVAL",
			envVal: "5m",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if c.Worker.Interval != 5*time.Minute {
					t.Errorf("Worker.Interval=%v", c.Worker.Interval)
				}
			},
		},
		{
			name:   "server_port",
			envKey: "JTPOST_SERVER_PORT",
			envVal: "9000",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if c.Server.Port != 9000 {
					t.Errorf("Server.Port=%d", c.Server.Port)
				}
			},
		},
		{
			name:   "server_base_url",
			envKey: "JTPOST_SERVER_BASE_URL",
			envVal: "https://example.com",
			check: func(t *testing.T, c *Config) {
				t.Helper()
				if c.Server.BaseURL != "https://example.com" {
					t.Errorf("Server.BaseURL=%q", c.Server.BaseURL)
				}
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(tc.envKey, tc.envVal)
			cfg, err := LoadWithDefaults("")
			if err != nil {
				t.Fatalf("LoadWithDefaults: %v", err)
			}
			tc.check(t, cfg)
		})
	}
}

func TestConfig_Validate_RejectsZeroTenant(t *testing.T) {
	_, author := validUUIDs(t)
	cfg := NewDefaultConfig()
	cfg.Auth.TenantDefault = uuid.Nil
	cfg.Auth.AuthorDefault = author
	err := cfg.Validate()
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Fatalf("ожидали ErrConfigInvalid, получили %v", err)
	}
}

func TestConfig_Validate_RejectsZeroAuthor(t *testing.T) {
	tenant, _ := validUUIDs(t)
	cfg := NewDefaultConfig()
	cfg.Auth.TenantDefault = tenant
	cfg.Auth.AuthorDefault = uuid.Nil
	err := cfg.Validate()
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Fatalf("ожидали ErrConfigInvalid, получили %v", err)
	}
}

func TestConfig_Validate_InvalidStorageType(t *testing.T) {
	tenant, author := validUUIDs(t)
	cfg := NewDefaultConfig()
	cfg.Auth.TenantDefault = tenant
	cfg.Auth.AuthorDefault = author
	cfg.Storage.Type = "mysql"
	err := cfg.Validate()
	if !errors.Is(err, core.ErrConfigInvalid) {
		t.Fatalf("ожидали ErrConfigInvalid, получили %v", err)
	}
}

func TestConfig_Validate_AcceptsAllStorageTypes(t *testing.T) {
	tenant, author := validUUIDs(t)
	for _, st := range []string{"fs", "sqlite", "postgres"} {
		t.Run(st, func(t *testing.T) {
			cfg := NewDefaultConfig()
			cfg.PostsDir = "content/posts"
			cfg.Auth.TenantDefault = tenant
			cfg.Auth.AuthorDefault = author
			cfg.Storage.Type = st
			if st == "postgres" {
				cfg.Storage.Postgres.DSN = "postgres://localhost/jtpost"
			}
			if err := cfg.Validate(); err != nil {
				t.Errorf("Validate(%s): %v", st, err)
			}
		})
	}
}

func TestConfig_Validate_StorageDSN(t *testing.T) {
	tenant, author := validUUIDs(t)
	tt := []struct {
		name    string
		mutate  func(*Config)
		wantErr bool
	}{
		{
			name: "fs_no_dsn_required",
			mutate: func(c *Config) {
				c.Storage.Type = "fs"
				c.Storage.SQLite.DSN = ""
				c.Storage.Postgres.DSN = ""
			},
			wantErr: false,
		},
		{
			name: "sqlite_empty_dsn_fails",
			mutate: func(c *Config) {
				c.Storage.Type = "sqlite"
				c.Storage.SQLite.DSN = ""
				c.SQLite.DSN = ""
			},
			wantErr: true,
		},
		{
			name: "sqlite_storage_dsn_ok",
			mutate: func(c *Config) {
				c.Storage.Type = "sqlite"
				c.Storage.SQLite.DSN = "/tmp/x.db"
				c.SQLite.DSN = ""
			},
			wantErr: false,
		},
		{
			name: "sqlite_legacy_dsn_fallback_ok",
			mutate: func(c *Config) {
				c.Storage.Type = "sqlite"
				c.Storage.SQLite.DSN = ""
				c.SQLite.DSN = "/tmp/legacy.db"
			},
			wantErr: false,
		},
		{
			name: "postgres_empty_dsn_fails",
			mutate: func(c *Config) {
				c.Storage.Type = "postgres"
				c.Storage.Postgres.DSN = ""
			},
			wantErr: true,
		},
		{
			name: "postgres_dsn_ok",
			mutate: func(c *Config) {
				c.Storage.Type = "postgres"
				c.Storage.Postgres.DSN = "postgres://u:p@h/db"
			},
			wantErr: false,
		},
		{
			name: "postgres_negative_max_open_fails",
			mutate: func(c *Config) {
				c.Storage.Type = "postgres"
				c.Storage.Postgres.DSN = "postgres://h/db"
				c.Storage.Postgres.MaxOpenConns = -1
			},
			wantErr: true,
		},
		{
			name: "postgres_negative_max_idle_fails",
			mutate: func(c *Config) {
				c.Storage.Type = "postgres"
				c.Storage.Postgres.DSN = "postgres://h/db"
				c.Storage.Postgres.MaxIdleConns = -2
			},
			wantErr: true,
		},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			cfg := NewDefaultConfig()
			cfg.Auth.TenantDefault = tenant
			cfg.Auth.AuthorDefault = author
			tc.mutate(cfg)
			err := cfg.Validate()
			if tc.wantErr && !errors.Is(err, core.ErrConfigInvalid) {
				t.Fatalf("want ErrConfigInvalid, got %v", err)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("want nil, got %v", err)
			}
		})
	}
}

func TestConfig_SQLiteDSN_Priority(t *testing.T) {
	cfg := &Config{}
	cfg.Storage.SQLite.DSN = "/from/storage.db"
	cfg.SQLite.DSN = "/from/legacy.db"
	if got := cfg.SQLiteDSN(); got != "/from/storage.db" {
		t.Errorf("storage.sqlite.dsn must win, got %q", got)
	}
	cfg.Storage.SQLite.DSN = ""
	if got := cfg.SQLiteDSN(); got != "/from/legacy.db" {
		t.Errorf("legacy fallback failed, got %q", got)
	}
	cfg.SQLite.DSN = ""
	if got := cfg.SQLiteDSN(); got != "" {
		t.Errorf("both empty must yield empty, got %q", got)
	}
}

func TestConfig_PreservesDefaultsPlatforms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".jtpost.yaml")
	cfg := NewDefaultConfig()
	cfg.Defaults.Platforms = []string{"telegram"}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded.Defaults.Platforms) != 1 || loaded.Defaults.Platforms[0] != "telegram" {
		t.Errorf("Defaults.Platforms=%v", loaded.Defaults.Platforms)
	}
}

func TestConfig_DurationParsing(t *testing.T) {
	t.Setenv("JTPOST_WORKER_INTERVAL", "2m30s")
	cfg, err := LoadWithDefaults("")
	if err != nil {
		t.Fatalf("LoadWithDefaults: %v", err)
	}
	want := 2*time.Minute + 30*time.Second
	if cfg.Worker.Interval != want {
		t.Errorf("Worker.Interval=%v, want %v", cfg.Worker.Interval, want)
	}
}
