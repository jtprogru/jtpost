// Package config загружает конфигурацию jtpost из YAML, переменных окружения и
// флагов с приоритетом flags > env > yaml > defaults. Загрузка реализована
// через viper.
package config

import (
	"errors"
	"os"
	"strings"
	"time"

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

	// Явный bind для вложенных env-переменных — AutomaticEnv не обходит
	// неизвестные ключи без хинта.
	for _, key := range []string{
		"posts_dir", "templates_dir",
		"telegram.bot_token", "telegram.chat_id",
		"sqlite.dsn",
		"defaults.status", "defaults.platforms",
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
	if err := v.Unmarshal(&cfg); err != nil {
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
	return nil
}
