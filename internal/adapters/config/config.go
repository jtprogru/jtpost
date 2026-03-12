package config

import (
	"os"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
	"gopkg.in/yaml.v3"
)

// Config конфигурация проекта jtpost.
type Config struct {
	// PostsDir директория с постами
	PostsDir string `yaml:"posts_dir,omitempty" json:"posts_dir,omitempty"`
	// TemplatesDir директория с шаблонами
	TemplatesDir string `yaml:"templates_dir,omitempty" json:"templates_dir,omitempty"`
	// Telegram настройки Telegram
	Telegram TelegramConfig `yaml:"telegram,omitempty" json:"telegram,omitempty"`
	// SQLite настройки SQLite хранилища
	SQLite SQLiteConfig `yaml:"sqlite,omitempty" json:"sqlite,omitempty"`
	// Defaults настройки по умолчанию
	Defaults DefaultConfig `yaml:"defaults,omitempty" json:"defaults,omitempty"`
}

// SQLiteConfig настройки SQLite хранилища.
type SQLiteConfig struct {
	// DSN строка подключения к базе данных (путь к файлу .db)
	DSN string `yaml:"dsn,omitempty" json:"dsn,omitempty"`
}

// TelegramConfig настройки Telegram.
type TelegramConfig struct {
	BotToken string `yaml:"bot_token,omitempty" json:"bot_token,omitempty"`
	ChatID   string `yaml:"chat_id,omitempty" json:"chat_id,omitempty"`
}

// DefaultConfig настройки по умолчанию.
type DefaultConfig struct {
	Status    string       `yaml:"status,omitempty" json:"status,omitempty"`
	Platforms []string     `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	Deadline  *time.Time   `yaml:"deadline,omitempty" json:"deadline,omitempty"`
}

// NewDefaultConfig возвращает конфигурацию по умолчанию.
func NewDefaultConfig() *Config {
	return &Config{
		PostsDir:     "content/posts",
		TemplatesDir: "templates",
		SQLite: SQLiteConfig{
			DSN: ".jtpost.db",
		},
		Defaults: DefaultConfig{
			Status:    string(core.StatusIdea),
			Platforms: []string{"telegram"},
		},
	}
}

// Load загружает конфигурацию из файла.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrConfigNotFound
		}
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, core.ErrConfigInvalid
	}

	// Применяем значения по умолчанию если не указаны
	if cfg.PostsDir == "" {
		cfg.PostsDir = NewDefaultConfig().PostsDir
	}
	if cfg.TemplatesDir == "" {
		cfg.TemplatesDir = NewDefaultConfig().TemplatesDir
	}
	if cfg.SQLite.DSN == "" {
		cfg.SQLite.DSN = NewDefaultConfig().SQLite.DSN
	}

	return &cfg, nil
}

// Save сохраняет конфигурацию в файл.
func (c *Config) Save(path string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0o644)
}

// Validate проверяает валидность конфигурации.
func (c *Config) Validate() error {
	if c.PostsDir == "" {
		return core.ErrPostsDirNotFound
	}
	return nil
}
