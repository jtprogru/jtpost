package fsrepo

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
	"gopkg.in/yaml.v3"
)

// FrontmatterType представляет тип frontmatter.
type FrontmatterType int

const (
	// FrontmatterNone отсутствует.
	FrontmatterNone FrontmatterType = iota
	// FrontmatterYAML YAML формат.
	FrontmatterYAML
	// FrontmatterTOML TOML формат (пока не поддерживается).
	FrontmatterTOML
)

// FrontmatterResult результат парсинга frontmatter.
type FrontmatterResult struct {
	Type       FrontmatterType
	HasFrontmatter bool
	Content    string
	Metadata   map[string]interface{}
	RawFrontmatter string // Исходный текст frontmatter
}

// yamlDelimiter регулярное выражение для YAML frontmatter.
var yamlDelimiter = regexp.MustCompile(`^---\s*$`)

// ParseFrontmatter парсит frontmatter из Markdown контента.
// Поддерживает YAML и отсутствие frontmatter.
func ParseFrontmatter(content string) (*FrontmatterResult, error) {
	lines := strings.Split(content, "\n")
	
	// Проверяем, начинается ли с ---
	if len(lines) == 0 || !yamlDelimiter.MatchString(strings.TrimSpace(lines[0])) {
		// Нет frontmatter
		return &FrontmatterResult{
			Type:       FrontmatterNone,
			HasFrontmatter: false,
			Content:    content,
			Metadata:   make(map[string]interface{}),
			RawFrontmatter: "",
		}, nil
	}

	// Ищем закрывающий ---
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if yamlDelimiter.MatchString(strings.TrimSpace(lines[i])) {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		// Непарный frontmatter, считаем что это контент
		return &FrontmatterResult{
			Type:       FrontmatterNone,
			HasFrontmatter: false,
			Content:    content,
			Metadata:   make(map[string]interface{}),
			RawFrontmatter: "",
		}, nil
	}

	// Извлекаем frontmatter
	frontmatterLines := lines[1:endIndex]
	frontmatterStr := strings.Join(frontmatterLines, "\n")
	
	// Извлекаем контент после frontmatter
	contentLines := lines[endIndex+1:]
	contentStr := strings.Join(contentLines, "\n")

	// Парсим YAML
	metadata, err := parseYAMLToMap(frontmatterStr)
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга YAML frontmatter: %w", err)
	}

	return &FrontmatterResult{
		Type:       FrontmatterYAML,
		HasFrontmatter: true,
		Content:    strings.TrimSpace(contentStr),
		Metadata:   metadata,
		RawFrontmatter: frontmatterStr,
	}, nil
}

// parseYAMLToMap парсит YAML строку в map.
func parseYAMLToMap(yamlStr string) (map[string]interface{}, error) {
	var result map[string]interface{}
	decoder := yaml.NewDecoder(strings.NewReader(yamlStr))
	if err := decoder.Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

// NormalizeFrontmatter нормализует frontmatter до стандарта jtpost.
// Добавляет отсутствующие поля, обновляет форматы.
func NormalizeFrontmatter(result *FrontmatterResult, slug string) (*core.Post, error) {
	post := &core.Post{
		Slug:     slug,
		Content:  result.Content,
		External: core.ExternalLinks{},
	}

	// Устанавливаем значения по умолчанию
	post.Status = core.StatusIdea
	post.Platforms = []core.Platform{core.PlatformTelegram}
	post.Tags = []string{}

	// Если нет frontmatter, возвращаем пост с дефолтными значениями
	if !result.HasFrontmatter {
		return post, nil
	}

	metadata := result.Metadata

	// Извлекаем ID
	if id, ok := metadata["id"].(string); ok && id != "" {
		post.ID = core.PostID(id)
	}

	// Извлекаем title
	if title, ok := metadata["title"].(string); ok {
		post.Title = title
	}

	// Извлекаем slug (перезаписываем если есть в frontmatter)
	if slug, ok := metadata["slug"].(string); ok && slug != "" {
		post.Slug = slug
	}

	// Извлекаем status
	if status, ok := metadata["status"].(string); ok {
		post.Status = core.PostStatus(status)
		// Валидируем статус
		if !IsValidPostStatus(post.Status) {
			post.Status = core.StatusDraft // По умолчанию draft для импортируемых
		}
	}

	// Извлекаем platforms
	if platformsRaw, ok := metadata["platforms"]; ok {
		switch v := platformsRaw.(type) {
		case []interface{}:
			post.Platforms = make([]core.Platform, len(v))
			for i, p := range v {
				if pStr, ok := p.(string); ok {
					post.Platforms[i] = core.Platform(pStr)
				}
			}
		case string:
			// Если строка (например, "telegram"), оборачиваем в слайс
			post.Platforms = []core.Platform{core.Platform(v)}
		}
	}

	// Если платформы пустые, устанавливаем telegram по умолчанию
	if len(post.Platforms) == 0 {
		post.Platforms = []core.Platform{core.PlatformTelegram}
	}

	// Извлекаем tags
	if tagsRaw, ok := metadata["tags"]; ok {
		switch v := tagsRaw.(type) {
		case []interface{}:
			post.Tags = make([]string, len(v))
			for i, t := range v {
				if tStr, ok := t.(string); ok {
					post.Tags[i] = tStr
				}
			}
		case string:
			// Если строка с запятыми
			tagParts := strings.Split(v, ",")
			post.Tags = make([]string, len(tagParts))
			for i, t := range tagParts {
				post.Tags[i] = strings.TrimSpace(t)
			}
		}
	}

	// Извлекаем deadline
	if deadlineRaw, ok := metadata["deadline"]; ok {
		if deadline, err := parseTime(deadlineRaw); err == nil {
			post.Deadline = &deadline
		}
	}

	// Извлекаем scheduled_at
	if scheduledRaw, ok := metadata["scheduled_at"]; ok {
		if scheduled, err := parseTime(scheduledRaw); err == nil {
			post.ScheduledAt = &scheduled
		}
	}

	// Извлекаем published_at
	if publishedRaw, ok := metadata["published_at"]; ok {
		if published, err := parseTime(publishedRaw); err == nil {
			post.PublishedAt = &published
		}
	}

	// Извлекаем external links
	if externalRaw, ok := metadata["external"]; ok {
		if externalMap, ok := externalRaw.(map[string]interface{}); ok {
			if telegramURL, ok := externalMap["telegram_url"].(string); ok {
				post.External.TelegramURL = telegramURL
			}
			// Blog URL игнорируем (удаляем)
		}
	}

	return post, nil
}

// parseTime парсит время из различных форматов.
func parseTime(raw interface{}) (time.Time, error) {
	switch v := raw.(type) {
	case time.Time:
		return v, nil
	case string:
		// Пробуем различные форматы
		formats := []string{
			time.RFC3339,
			time.RFC3339Nano,
			"2006-01-02T15:04:05Z07:00",
			"2006-01-02T15:04:05",
			"2006-01-02 15:04:05",
			"2006-01-02",
		}
		for _, format := range formats {
			if t, err := time.Parse(format, v); err == nil {
				return t, nil
			}
		}
		return time.Time{}, fmt.Errorf("неверный формат даты: %s", v)
	default:
		return time.Time{}, fmt.Errorf("неверный тип даты: %T", v)
	}
}

// IsValidPostStatus проверяет валидность статуса.
func IsValidPostStatus(status core.PostStatus) bool {
	validStatuses := []core.PostStatus{
		core.StatusIdea,
		core.StatusDraft,
		core.StatusReady,
		core.StatusScheduled,
		core.StatusPublished,
	}
	return slices.Contains(validStatuses, status)
}

// BuildFrontmatter строит YAML frontmatter из поста.
func BuildFrontmatter(post *core.Post) (string, error) {
	var buf bytes.Buffer

	buf.WriteString("---\n")
	
	// ID
	if post.ID != "" {
		writeYAMLField(&buf, "id", string(post.ID))
	}
	
	// Title
	if post.Title != "" {
		writeYAMLField(&buf, "title", post.Title)
	}
	
	// Slug
	if post.Slug != "" {
		writeYAMLField(&buf, "slug", post.Slug)
	}
	
	// Status
	writeYAMLField(&buf, "status", string(post.Status))
	
	// Platforms
	if len(post.Platforms) > 0 {
		platforms := make([]string, len(post.Platforms))
		for i, p := range post.Platforms {
			platforms[i] = string(p)
		}
		writeYAMLArray(&buf, "platforms", platforms)
	}
	
	// Tags
	if len(post.Tags) > 0 {
		writeYAMLArray(&buf, "tags", post.Tags)
	}
	
	// Deadline
	if post.Deadline != nil {
		writeYAMLField(&buf, "deadline", post.Deadline.Format(time.RFC3339))
	}
	
	// ScheduledAt
	if post.ScheduledAt != nil {
		writeYAMLField(&buf, "scheduled_at", post.ScheduledAt.Format(time.RFC3339))
	}
	
	// PublishedAt
	if post.PublishedAt != nil {
		writeYAMLField(&buf, "published_at", post.PublishedAt.Format(time.RFC3339))
	}
	
	// External
	if post.External.TelegramURL != "" {
		buf.WriteString("external:\n")
		fmt.Fprintf(&buf, "  telegram_url: %q\n", post.External.TelegramURL)
	}

	buf.WriteString("---")

	return buf.String(), nil
}

// writeYAMLField записывает поле YAML.
func writeYAMLField(buf *bytes.Buffer, key string, value string) {
	fmt.Fprintf(buf, "%s: %q\n", key, value)
}

// writeYAMLArray записывает массив YAML.
func writeYAMLArray(buf *bytes.Buffer, key string, values []string) {
	fmt.Fprintf(buf, "%s:\n", key)
	for _, v := range values {
		fmt.Fprintf(buf, "  - %q\n", v)
	}
}

// SerializePostWithFrontmatter сериализует пост в Markdown с нормализованным frontmatter.
func SerializePostWithFrontmatter(post *core.Post) ([]byte, error) {
	var buf bytes.Buffer
	
	// Строим frontmatter
	frontmatter, err := BuildFrontmatter(post)
	if err != nil {
		return nil, err
	}
	
	buf.WriteString(frontmatter)
	buf.WriteString("\n\n")
	buf.WriteString(post.Content)
	
	return buf.Bytes(), nil
}

// ImportReport отчёт об импорте одного файла.
type ImportReport struct {
	FilePath     string
	Slug         string
	Status       string
	HasFrontmatter bool
	FrontmatterType FrontmatterType
	Action       string // "created", "updated", "skipped"
	Error        error
}

// ImportStats статистика импорта.
type ImportStats struct {
	Total     int
	Imported  int
	Updated   int
	Skipped   int
	Errors    int
}
