// Package config загружает конфигурацию jtpost из YAML, переменных окружения и
// флагов с приоритетом flags > env > yaml > defaults. Загрузка реализована
// через viper.
package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"text/template"
	"time"

	"github.com/go-viper/mapstructure/v2"
	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

// EnvPrefix префикс переменных окружения. Например JTPOST_POSTS_DIR,
// JTPOST_TELEGRAM_BOT_TOKEN, JTPOST_SQLITE_DSN.
const EnvPrefix = "JTPOST"

// Config конфигурация проекта jtpost.
type Config struct {
	PostsDir     string         `yaml:"posts_dir,omitempty" json:"posts_dir,omitempty" mapstructure:"posts_dir"`
	TemplatesDir string         `yaml:"templates_dir,omitempty" json:"templates_dir,omitempty" mapstructure:"templates_dir"`
	Telegram     TelegramConfig `yaml:"telegram,omitempty" json:"telegram,omitempty" mapstructure:"telegram"`
	SQLite       SQLiteConfig   `yaml:"sqlite,omitempty" json:"sqlite,omitempty" mapstructure:"sqlite"`
	Storage      StorageConfig  `yaml:"storage,omitempty" json:"storage,omitempty" mapstructure:"storage"`
	Auth         AuthConfig     `yaml:"auth,omitempty" json:"auth,omitempty" mapstructure:"auth"`
	Worker       WorkerConfig   `yaml:"worker,omitempty" json:"worker,omitempty" mapstructure:"worker"`
	Server       ServerConfig   `yaml:"server,omitempty" json:"server,omitempty" mapstructure:"server"`
	Defaults     DefaultConfig  `yaml:"defaults,omitempty" json:"defaults,omitempty" mapstructure:"defaults"`
}

// SQLiteConfig настройки SQLite хранилища.
type SQLiteConfig struct {
	DSN string `yaml:"dsn,omitempty" json:"dsn,omitempty" mapstructure:"dsn"`
}

// TelegramConfig настройки Telegram.
type TelegramConfig struct {
	BotToken string `yaml:"bot_token,omitempty" json:"bot_token,omitempty" mapstructure:"bot_token"`
	ChatID   string `yaml:"chat_id,omitempty" json:"chat_id,omitempty" mapstructure:"chat_id"`
}

// DefaultConfig настройки по умолчанию.
type DefaultConfig struct {
	Status    string     `yaml:"status,omitempty" json:"status,omitempty" mapstructure:"status"`
	Platforms []string   `yaml:"platforms,omitempty" json:"platforms,omitempty" mapstructure:"platforms"`
	Deadline  *time.Time `yaml:"deadline,omitempty" json:"deadline,omitempty" mapstructure:"deadline"`
}

// StorageConfig настройки слоя хранения.
type StorageConfig struct {
	Type     string           `yaml:"type" mapstructure:"type"`
	Git      GitStorageConfig `yaml:"git" mapstructure:"git"`
	SQLite   SQLiteConfig     `yaml:"sqlite" mapstructure:"sqlite"`
	Postgres PostgresConfig   `yaml:"postgres" mapstructure:"postgres"`
}

// GitStorageConfig настройки git-хранилища постов.
type GitStorageConfig struct {
	Enabled        bool   `yaml:"enabled" mapstructure:"enabled"`
	AutoCommit     bool   `yaml:"auto_commit" mapstructure:"auto_commit"`
	AutoPush       bool   `yaml:"auto_push" mapstructure:"auto_push"`
	Remote         string `yaml:"remote" mapstructure:"remote"`
	Branch         string `yaml:"branch" mapstructure:"branch"`
	CommitTemplate string `yaml:"commit_template" mapstructure:"commit_template"`
}

// PostgresConfig настройки PostgreSQL хранилища.
type PostgresConfig struct {
	DSN             string        `yaml:"dsn" mapstructure:"dsn"`
	MaxOpenConns    int           `yaml:"max_open_conns" mapstructure:"max_open_conns"`
	MaxIdleConns    int           `yaml:"max_idle_conns" mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `yaml:"conn_max_lifetime" mapstructure:"conn_max_lifetime"`
}

// AuthConfig настройки аутентификации.
type AuthConfig struct {
	Type          string        `yaml:"type" mapstructure:"type"`
	Secret        string        `yaml:"secret" mapstructure:"secret"`
	TenantDefault uuid.UUID     `yaml:"tenant_default" mapstructure:"tenant_default"`
	AuthorDefault uuid.UUID     `yaml:"author_default" mapstructure:"author_default"`
	OAuth         OAuthConfig   `yaml:"oauth" mapstructure:"oauth"`
	TokenTTL      time.Duration `yaml:"token_ttl" mapstructure:"token_ttl"`
	BCryptCost    int           `yaml:"bcrypt_cost,omitempty" mapstructure:"bcrypt_cost"`
	SessionTTL    time.Duration `yaml:"session_ttl,omitempty" mapstructure:"session_ttl"`
}

// OAuthConfig настройки OAuth провайдера.
type OAuthConfig struct {
	Provider     string `yaml:"provider" mapstructure:"provider"`
	ClientID     string `yaml:"client_id" mapstructure:"client_id"`
	ClientSecret string `yaml:"client_secret" mapstructure:"client_secret"`
	RedirectURL  string `yaml:"redirect_url" mapstructure:"redirect_url"`
}

// WorkerConfig настройки фонового воркера.
type WorkerConfig struct {
	Enabled      bool          `yaml:"enabled" mapstructure:"enabled"`
	Interval     time.Duration `yaml:"interval" mapstructure:"interval"`
	MaxRetries   int           `yaml:"max_retries" mapstructure:"max_retries"`
	RetryBackoff time.Duration `yaml:"retry_backoff" mapstructure:"retry_backoff"`
}

// ServerConfig настройки HTTP-сервера.
type ServerConfig struct {
	Addr         string        `yaml:"addr" mapstructure:"addr"`
	Port         int           `yaml:"port" mapstructure:"port"`
	BaseURL      string        `yaml:"base_url" mapstructure:"base_url"`
	ReadTimeout  time.Duration `yaml:"read_timeout" mapstructure:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout" mapstructure:"write_timeout"`
	CookieSecure bool          `yaml:"cookie_secure" mapstructure:"cookie_secure"`
	CookieDomain string        `yaml:"cookie_domain,omitempty" mapstructure:"cookie_domain"`
}

// NewDefaultConfig возвращает конфигурацию по умолчанию.
func NewDefaultConfig() *Config {
	return &Config{
		PostsDir:     "content/posts",
		TemplatesDir: "templates",
		SQLite: SQLiteConfig{
			DSN: ".jtpost.db",
		},
		Storage: StorageConfig{
			Type: "fs",
			Git: GitStorageConfig{
				AutoCommit:     true,
				Branch:         "main",
				CommitTemplate: "chore: {{.Operation}} post {{.Slug}}",
			},
			SQLite: SQLiteConfig{
				DSN: ".jtpost.db",
			},
			Postgres: PostgresConfig{
				MaxOpenConns:    10,
				MaxIdleConns:    5,
				ConnMaxLifetime: 30 * time.Minute,
			},
		},
		Auth: AuthConfig{
			Type:       "none",
			TokenTTL:   24 * time.Hour,
			BCryptCost: 10,
			SessionTTL: 24 * time.Hour,
		},
		Worker: WorkerConfig{
			Interval:     time.Minute,
			MaxRetries:   3,
			RetryBackoff: 30 * time.Second,
		},
		Server: ServerConfig{
			Addr:         "localhost",
			Port:         8080,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			CookieSecure: true,
		},
		Defaults: DefaultConfig{
			Status:    string(core.StatusIdea),
			Platforms: []string{"telegram"},
		},
	}
}

// Load загружает конфигурацию из файла с наложением переменных окружения.
// Приоритет: env > yaml > defaults.
//
// Если файл path не существует, возвращается core.ErrConfigNotFound (для
// сохранения обратной совместимости со старыми вызовами). Переменные
// окружения в этом случае не применяются — вызывающий код сам решит, что
// делать (обычно — создать дефолтную конфигурацию).
func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, core.ErrConfigNotFound
		}
		return nil, err
	}
	return loadFromFile(path)
}

// LoadWithDefaults загружает конфигурацию аналогично Load, но при отсутствии
// файла возвращает значения по умолчанию (с применённым env). Используется
// командами, для которых наличие файла не обязательно.
func LoadWithDefaults(path string) (*Config, error) {
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return loadFromFile("")
		}
		return nil, err
	}
	return loadFromFile(path)
}

// uuidDecodeHook парсит uuid.UUID из строкового значения (env/yaml).
func uuidDecodeHook() mapstructure.DecodeHookFuncType {
	return func(_, to reflect.Type, data interface{}) (interface{}, error) {
		if to != reflect.TypeOf(uuid.UUID{}) {
			return data, nil
		}
		s, ok := data.(string)
		if !ok {
			return data, nil
		}
		if s == "" {
			return uuid.UUID{}, nil
		}
		return uuid.Parse(s)
	}
}

func loadFromFile(path string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix(EnvPrefix)
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	def := NewDefaultConfig()
	v.SetDefault("posts_dir", def.PostsDir)
	v.SetDefault("templates_dir", def.TemplatesDir)
	v.SetDefault("sqlite.dsn", def.SQLite.DSN)
	v.SetDefault("defaults.status", def.Defaults.Status)
	v.SetDefault("defaults.platforms", def.Defaults.Platforms)

	v.SetDefault("storage.type", def.Storage.Type)
	v.SetDefault("storage.git.enabled", def.Storage.Git.Enabled)
	v.SetDefault("storage.git.auto_commit", def.Storage.Git.AutoCommit)
	v.SetDefault("storage.git.auto_push", def.Storage.Git.AutoPush)
	v.SetDefault("storage.git.remote", def.Storage.Git.Remote)
	v.SetDefault("storage.git.branch", def.Storage.Git.Branch)
	v.SetDefault("storage.git.commit_template", def.Storage.Git.CommitTemplate)
	v.SetDefault("storage.sqlite.dsn", def.Storage.SQLite.DSN)
	v.SetDefault("storage.postgres.dsn", def.Storage.Postgres.DSN)
	v.SetDefault("storage.postgres.max_open_conns", def.Storage.Postgres.MaxOpenConns)
	v.SetDefault("storage.postgres.max_idle_conns", def.Storage.Postgres.MaxIdleConns)
	v.SetDefault("storage.postgres.conn_max_lifetime", def.Storage.Postgres.ConnMaxLifetime)

	v.SetDefault("auth.type", def.Auth.Type)
	v.SetDefault("auth.token_ttl", def.Auth.TokenTTL)
	v.SetDefault("auth.bcrypt_cost", def.Auth.BCryptCost)
	v.SetDefault("auth.session_ttl", def.Auth.SessionTTL)
	v.SetDefault("server.cookie_secure", def.Server.CookieSecure)

	v.SetDefault("worker.enabled", def.Worker.Enabled)
	v.SetDefault("worker.interval", def.Worker.Interval)
	v.SetDefault("worker.max_retries", def.Worker.MaxRetries)
	v.SetDefault("worker.retry_backoff", def.Worker.RetryBackoff)

	v.SetDefault("server.addr", def.Server.Addr)
	v.SetDefault("server.port", def.Server.Port)
	v.SetDefault("server.read_timeout", def.Server.ReadTimeout)
	v.SetDefault("server.write_timeout", def.Server.WriteTimeout)

	// Явный bind для вложенных env-переменных — AutomaticEnv не обходит
	// неизвестные ключи без хинта.
	for _, key := range []string{
		"posts_dir", "templates_dir",
		"telegram.bot_token", "telegram.chat_id",
		"sqlite.dsn",
		"defaults.status", "defaults.platforms",
		"storage.type",
		"storage.git.enabled", "storage.git.auto_commit", "storage.git.auto_push",
		"storage.git.remote", "storage.git.branch", "storage.git.commit_template",
		"storage.sqlite.dsn",
		"storage.postgres.dsn", "storage.postgres.max_open_conns",
		"storage.postgres.max_idle_conns", "storage.postgres.conn_max_lifetime",
		"auth.type", "auth.secret", "auth.tenant_default", "auth.author_default",
		"auth.token_ttl", "auth.bcrypt_cost", "auth.session_ttl",
		"server.cookie_secure", "server.cookie_domain",
		"auth.oauth.provider", "auth.oauth.client_id",
		"auth.oauth.client_secret", "auth.oauth.redirect_url",
		"worker.enabled", "worker.interval", "worker.max_retries", "worker.retry_backoff",
		"server.addr", "server.port", "server.base_url",
		"server.read_timeout", "server.write_timeout",
	} {
		_ = v.BindEnv(key)
	}

	if path != "" {
		v.SetConfigFile(path)
		v.SetConfigType("yaml")
		if err := v.ReadInConfig(); err != nil {
			return nil, core.ErrConfigInvalid
		}
	}

	var cfg Config
	decodeOpt := viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(
		mapstructure.StringToTimeDurationHookFunc(),
		uuidDecodeHook(),
	))
	if err := v.Unmarshal(&cfg, decodeOpt); err != nil {
		return nil, core.ErrConfigInvalid
	}

	return &cfg, nil
}

// Save сохраняет конфигурацию в YAML-файл.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Validate проверяет валидность конфигурации.
func (c *Config) Validate() error {
	if c.PostsDir == "" {
		return core.ErrPostsDirNotFound
	}
	if c.Storage.Type != "" &&
		c.Storage.Type != "fs" &&
		c.Storage.Type != "sqlite" &&
		c.Storage.Type != "postgres" {
		return fmt.Errorf("%w: invalid storage.type %q", core.ErrConfigInvalid, c.Storage.Type)
	}
	if c.Storage.Type == "sqlite" && c.SQLiteDSN() == "" {
		return fmt.Errorf("%w: storage.sqlite.dsn required", core.ErrConfigInvalid)
	}
	if c.Storage.Type == "postgres" && c.Storage.Postgres.DSN == "" {
		return fmt.Errorf("%w: storage.postgres.dsn required", core.ErrConfigInvalid)
	}
	if c.Storage.Postgres.MaxOpenConns < 0 || c.Storage.Postgres.MaxIdleConns < 0 {
		return fmt.Errorf("%w: storage.postgres pool sizes must be non-negative", core.ErrConfigInvalid)
	}
	if c.Storage.Git.Enabled {
		if c.Storage.Git.AutoPush && c.Storage.Git.Remote == "" {
			return fmt.Errorf("%w: storage.git.auto_push=true requires storage.git.remote", core.ErrConfigInvalid)
		}
		if c.Storage.Git.CommitTemplate != "" {
			if _, err := template.New("commit").Parse(c.Storage.Git.CommitTemplate); err != nil {
				return fmt.Errorf("%w: storage.git.commit_template invalid: %w", core.ErrConfigInvalid, err)
			}
		}
		if c.Storage.Git.Branch == "" {
			c.Storage.Git.Branch = "main"
		}
	}
	if c.Auth.TenantDefault == uuid.Nil {
		return fmt.Errorf("%w: auth.tenant_default required", core.ErrConfigInvalid)
	}
	if c.Auth.AuthorDefault == uuid.Nil {
		return fmt.Errorf("%w: auth.author_default required", core.ErrConfigInvalid)
	}
	if c.Auth.Type != "" && c.Auth.Type != "none" && c.Auth.Type != "token" {
		return fmt.Errorf("%w: invalid auth.type %q (must be none|token in F4a)", core.ErrConfigInvalid, c.Auth.Type)
	}
	if c.Auth.Type == "token" {
		if c.Storage.Type == "fs" {
			return fmt.Errorf("%w: auth.type=token requires storage.type=sqlite or postgres", core.ErrConfigInvalid)
		}
		if c.Auth.BCryptCost < 4 || c.Auth.BCryptCost > 14 {
			return fmt.Errorf("%w: auth.bcrypt_cost must be in [4, 14]", core.ErrConfigInvalid)
		}
		if c.Auth.SessionTTL > 0 && (c.Auth.SessionTTL < 5*time.Minute || c.Auth.SessionTTL > 720*time.Hour) {
			return fmt.Errorf("%w: auth.session_ttl must be in [5m, 720h]", core.ErrConfigInvalid)
		}
	}
	return nil
}

// SQLiteDSN возвращает DSN для SQLite с приоритетом storage.sqlite.dsn,
// fallback на legacy верхнеуровневый sqlite.dsn.
func (c *Config) SQLiteDSN() string {
	if c.Storage.SQLite.DSN != "" {
		return c.Storage.SQLite.DSN
	}
	return c.SQLite.DSN
}
