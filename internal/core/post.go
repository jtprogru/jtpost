// Package core содержит доменную модель проекта jtpost.
package core

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
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
	s := string(data[1 : len(data)-1])
	parsed, err := ParsePostID(s)
	if err != nil {
		return err
	}
	*id = parsed
	return nil
}

// GeneratePostID генерирует уникальный ID для поста используя UUID v7.
func GeneratePostID(_ string, _ time.Time) PostID {
	u, err := uuid.NewV7()
	if err != nil {
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

// AttachmentType тип медиа-вложения.
type AttachmentType string

// Допустимые типы вложений.
const (
	AttachmentTypePhoto    AttachmentType = "photo"
	AttachmentTypeVideo    AttachmentType = "video"
	AttachmentTypeDocument AttachmentType = "document"
	AttachmentTypeAudio    AttachmentType = "audio"
)

// Validate проверяет, что значение AttachmentType допустимо.
func (t AttachmentType) Validate() error {
	switch t {
	case AttachmentTypePhoto, AttachmentTypeVideo, AttachmentTypeDocument, AttachmentTypeAudio:
		return nil
	default:
		return fmt.Errorf("%w: invalid attachment type %q", ErrValidation, string(t))
	}
}

// Attachment медиа-вложение, прикрепляемое к Post.
type Attachment struct {
	ID       uuid.UUID      `yaml:"id" json:"id"`
	Type     AttachmentType `yaml:"type" json:"type"`
	Path     string         `yaml:"path,omitempty" json:"path,omitempty"`
	URL      string         `yaml:"url,omitempty" json:"url,omitempty"`
	Caption  string         `yaml:"caption,omitempty" json:"caption,omitempty"`
	MimeType string         `yaml:"mime_type,omitempty" json:"mime_type,omitempty"`
	Size     int64          `yaml:"size,omitempty" json:"size,omitempty"`
}

// AbsolutePath возвращает абсолютный путь к файлу вложения относительно postsDir.
// Защищается от traversal-атак: возвращает ErrValidation, если путь выводит за пределы postsDir.
func (a Attachment) AbsolutePath(postsDir string) (string, error) {
	absRoot, err := filepath.Abs(postsDir)
	if err != nil {
		return "", errors.Join(ErrValidation, err)
	}
	joined := filepath.Join(absRoot, a.Path)
	cleaned := filepath.Clean(joined)
	if !strings.HasPrefix(cleaned, absRoot+string(filepath.Separator)) && cleaned != absRoot {
		return "", fmt.Errorf("%w: unsafe attachment path", ErrValidation)
	}
	return cleaned, nil
}

// PublishAttempt запись попытки публикации (успех/провал).
type PublishAttempt struct {
	ID              uuid.UUID       `yaml:"id" json:"id"`
	At              time.Time       `yaml:"at" json:"at"`
	Target          string          `yaml:"target" json:"target"`
	Status          string          `yaml:"status" json:"status"`
	MessageID       string          `yaml:"message_id,omitempty" json:"message_id,omitempty"`
	ResponsePayload json.RawMessage `yaml:"response_payload,omitempty" json:"response_payload,omitempty"`
	Error           string          `yaml:"error,omitempty" json:"error,omitempty"`
	RetryAttempt    int             `yaml:"retry_attempt" json:"retry_attempt"`
	Duration        time.Duration   `yaml:"duration" json:"duration"`
}

// ExternalLinks ссылки на опубликованные посты на внешних платформах.
type ExternalLinks struct {
	TelegramURL string `yaml:"telegram_url,omitempty" json:"telegram_url,omitempty"`
}

// Post представляет пост в системе.
type Post struct {
	// Обязательные поля
	ID        PostID     `yaml:"id" json:"id"`
	TenantID  uuid.UUID  `yaml:"tenant_id" json:"tenant_id"`
	AuthorID  uuid.UUID  `yaml:"author_id" json:"author_id"`
	Title     string     `yaml:"title" json:"title"`
	Slug      string     `yaml:"slug" json:"slug"`
	Status    PostStatus `yaml:"status" json:"status"`
	CreatedAt time.Time  `yaml:"created_at" json:"created_at"`
	UpdatedAt time.Time  `yaml:"updated_at" json:"updated_at"`
	Revision  int        `yaml:"revision" json:"revision"`

	// Опциональные поля
	Tags           []string         `yaml:"tags,omitempty" json:"tags,omitempty"`
	Deadline       *time.Time       `yaml:"deadline,omitempty" json:"deadline,omitempty"`
	ScheduledAt    *time.Time       `yaml:"scheduled_at,omitempty" json:"scheduled_at,omitempty"`
	PublishedAt    *time.Time       `yaml:"published_at,omitempty" json:"published_at,omitempty"`
	Excerpt        *string          `yaml:"excerpt,omitempty" json:"excerpt,omitempty"`
	CoverImage     *Attachment      `yaml:"cover_image,omitempty" json:"cover_image,omitempty"`
	Attachments    []Attachment     `yaml:"attachments,omitempty" json:"attachments,omitempty"`
	PublishHistory []PublishAttempt `yaml:"publish_history,omitempty" json:"publish_history,omitempty"`
	RevisionSHA    *string          `yaml:"revision_sha,omitempty" json:"revision_sha,omitempty"`

	// Контент и внешние ссылки
	Content  string        `yaml:"-" json:"content"`
	External ExternalLinks `yaml:"external,omitempty" json:"external,omitempty"`
}

// TenantShortID возвращает первые 8 hex-символов TenantID без дефисов.
func (p Post) TenantShortID() string {
	return tenantShortID(p.TenantID)
}

// tenantShortID — общий хелпер для Post.TenantShortID и PostFilter.TenantShortID.
func tenantShortID(id uuid.UUID) string {
	s := strings.ReplaceAll(id.String(), "-", "")
	if len(s) < 8 {
		return s
	}
	return s[:8]
}

// PostFilter фильтры для поиска постов.
type PostFilter struct {
	// TenantID обязателен для всех операций List.
	TenantID uuid.UUID
	// AuthorID опционален — если задан, фильтрует только посты автора.
	AuthorID *uuid.UUID
	Statuses []PostStatus
	Tags     []string
	Search   string
	// SortBy одно из allowedSortKeys.
	SortBy string
	// SortOrder "asc" или "desc". Default "asc".
	SortOrder string
	// Limit ограничение количества возвращаемых постов. 0 = no limit.
	Limit int
	// Offset смещение для пагинации.
	Offset int
}

// TenantShortID возвращает первые 8 hex-символов TenantID без дефисов.
func (f PostFilter) TenantShortID() string {
	return tenantShortID(f.TenantID)
}

// allowedSortKeys набор валидных ключей сортировки PostFilter.SortBy.
//
//nolint:gochecknoglobals // immutable lookup table
var allowedSortKeys = map[string]struct{}{
	"created_at":   {},
	"updated_at":   {},
	"deadline":     {},
	"scheduled_at": {},
	"title":        {},
	"status":       {},
}

// IsValidSortKey проверяет, что s — валидный ключ сортировки.
func IsValidSortKey(s string) bool {
	_, ok := allowedSortKeys[s]
	return ok
}
