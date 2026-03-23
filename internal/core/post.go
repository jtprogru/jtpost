package core

import (
	"database/sql/driver"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// PostID уникальный идентификатор поста.
type PostID uuid.UUID

// String возвращает строковое представление PostID.
func (id PostID) String() string {
	return uuid.UUID(id).String()
}

// Value реализует driver.Valuer для PostID.
func (id PostID) Value() (driver.Value, error) {
	return id.String(), nil
}

// Scan реализует sql.Scanner для PostID.
func (id *PostID) Scan(value interface{}) error {
	if value == nil {
		return nil
	}
	s, ok := value.(string)
	if !ok {
		return fmt.Errorf("PostID.Scan: expected string, got %T", value)
	}
	parsed, err := ParsePostID(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// MarshalYAML реализует yaml.Marshaler для PostID.
func (id PostID) MarshalYAML() (interface{}, error) {
	return id.String(), nil
}

// UnmarshalYAML реализует yaml.Unmarshaler для PostID.
func (id *PostID) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	parsed, err := ParsePostID(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// MarshalJSON реализует json.Marshaler для PostID.
func (id PostID) MarshalJSON() ([]byte, error) {
	return []byte(`"` + id.String() + `"`), nil
}

// UnmarshalJSON реализует json.Unmarshaler для PostID.
func (id *PostID) UnmarshalJSON(data []byte) error {
	if len(data) < 2 {
		return nil
	}
	// Удаляем кавычки
	s := string(data[1 : len(data)-1])
	parsed, err := ParsePostID(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// GeneratePostID генерирует уникальный ID для поста используя UUID v7.
// Если генерация неудачна, использует текущее время для fallback.
func GeneratePostID(_ string, _ time.Time) PostID {
	// UUID v7 использует timestamp для генерации
	u, err := uuid.NewV7()
	if err != nil {
		// Fallback: генерируем случайный UUID
		u = uuid.New()
	}
	return PostID(u)
}

// ParsePostID парсит строку в PostID.
func ParsePostID(s string) (PostID, error) {
	u, err := uuid.Parse(s)
	if err != nil {
		return PostID{}, err
	}
	return PostID(u), nil
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
	Tags        []string   `yaml:"tags,omitempty" json:"tags,omitempty"`
	Deadline    *time.Time `yaml:"deadline,omitempty" json:"deadline,omitempty"`
	ScheduledAt *time.Time `yaml:"scheduled_at,omitempty" json:"scheduled_at,omitempty"`
	PublishedAt *time.Time `yaml:"published_at,omitempty" json:"published_at,omitempty"`
	Content     string     `yaml:"-" json:"content"`
	External    ExternalLinks `yaml:"external,omitempty" json:"external,omitempty"`
}

// PostFilter фильтры для поиска постов.
type PostFilter struct {
	Statuses []PostStatus
	Tags     []string
	Search   string
}
