package core

import (
	"fmt"
	"time"
)

// PostID уникальный идентификатор поста.
type PostID string

// GeneratePostID генерирует уникальный ID для поста.
func GeneratePostID(slug string, t time.Time) PostID {
	return PostID(fmt.Sprintf("%d-%s", t.UnixNano(), slug))
}

// ExternalLinks ссылки на опубликованные посты на внешних платформах.
type ExternalLinks struct {
	TelegramURL string `yaml:"telegram_url,omitempty" json:"telegram_url,omitempty"`
}

// Post представляет пост в системе.
type Post struct {
	ID          PostID     `yaml:"id" json:"id"`
	Title       string     `yaml:"title" json:"title"`
	Slug        string     `yaml:"slug" json:"slug"`
	Status      PostStatus `yaml:"status" json:"status"`
	Platforms   []Platform `yaml:"platforms,omitempty" json:"platforms,omitempty"`
	Tags        []string   `yaml:"tags,omitempty" json:"tags,omitempty"`
	Deadline    *time.Time `yaml:"deadline,omitempty" json:"deadline,omitempty"`
	ScheduledAt *time.Time `yaml:"scheduled_at,omitempty" json:"scheduled_at,omitempty"`
	PublishedAt *time.Time `yaml:"published_at,omitempty" json:"published_at,omitempty"`
	Content     string     `yaml:"-" json:"content"`
	External    ExternalLinks `yaml:"external,omitempty" json:"external,omitempty"`
}

// PostFilter фильтры для поиска постов.
type PostFilter struct {
	Statuses  []PostStatus
	Platforms []Platform
	Tags      []string
	Search    string
}
