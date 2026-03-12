package fsrepo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jtprogru/jtpost/internal/core"
	"gopkg.in/yaml.v3"
)

// FileSystemPostRepository реализация PostRepository на основе файловой системы.
type FileSystemPostRepository struct {
	rootPath string
}

// NewFileSystemRepository создаёт новый репозиторий с указанной корневой директорией.
func NewFileSystemRepository(rootPath string) (*FileSystemPostRepository, error) {
	absPath, err := filepath.Abs(rootPath)
	if err != nil {
		return nil, fmt.Errorf("ошибка получения абсолютного пути: %w", err)
	}

	if err := os.MkdirAll(absPath, 0o755); err != nil {
		return nil, fmt.Errorf("ошибка создания директории: %w", err)
	}

	return &FileSystemPostRepository{
		rootPath: absPath,
	}, nil
}

// GetByID возвращает пост по идентификатору.
func (r *FileSystemPostRepository) GetByID(ctx context.Context, id core.PostID) (*core.Post, error) {
	// Ищем файл по ID в названии
	filePath, err := r.findFileByID(id)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	post, err := ParsePost(data)
	if err != nil {
		return nil, err
	}

	post.ID = id
	return post, nil
}

// GetBySlug возвращает пост по slug.
func (r *FileSystemPostRepository) GetBySlug(ctx context.Context, slug string) (*core.Post, error) {
	filePath := filepath.Join(r.rootPath, slug+".md")

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	post, err := ParsePost(data)
	if err != nil {
		return nil, err
	}

	return post, nil
}

// List возвращает список постов с применением фильтров.
func (r *FileSystemPostRepository) List(ctx context.Context, filter core.PostFilter) ([]*core.Post, error) {
	entries, err := os.ReadDir(r.rootPath)
	if err != nil {
		return nil, err
	}

	var posts []*core.Post

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(r.rootPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue // Пропускаем проблемные файлы
		}

		post, err := ParsePost(data)
		if err != nil {
			continue // Пропускаем файлы с ошибками парсинга
		}

		// Применяем фильтры
		if !matchesFilter(post, filter) {
			continue
		}

		posts = append(posts, post)
	}

	return posts, nil
}

// Create создаёт новый пост.
func (r *FileSystemPostRepository) Create(ctx context.Context, post *core.Post) error {
	filePath := r.buildFilePath(post)

	// Проверяем, существует ли уже файл
	if _, err := os.Stat(filePath); err == nil {
		return core.ErrAlreadyExists
	}

	data, err := SerializePost(post)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0o644)
}

// Update обновляет существующий пост.
func (r *FileSystemPostRepository) Update(ctx context.Context, post *core.Post) error {
	filePath := r.buildFilePath(post)

	// Проверяем, существует ли файл
	if _, err := os.Stat(filePath); err != nil {
		if os.IsNotExist(err) {
			return core.ErrNotFound
		}
		return err
	}

	data, err := SerializePost(post)
	if err != nil {
		return err
	}

	return os.WriteFile(filePath, data, 0o644)
}

// Delete удаляет пост.
func (r *FileSystemPostRepository) Delete(ctx context.Context, id core.PostID) error {
	filePath, err := r.findFileByID(id)
	if err != nil {
		return err
	}

	return os.Remove(filePath)
}

// findFileByID ищет файл поста по ID.
// Поскольку файлы именуются по slug, мы читаем каждый файл и проверяем ID.
func (r *FileSystemPostRepository) findFileByID(id core.PostID) (string, error) {
	entries, err := os.ReadDir(r.rootPath)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(r.rootPath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		post, err := ParsePost(data)
		if err != nil {
			continue
		}

		if post.ID == id {
			return filePath, nil
		}
	}

	return "", core.ErrNotFound
}

// buildFilePath строит путь к файлу поста.
func (r *FileSystemPostRepository) buildFilePath(post *core.Post) string {
	filename := fmt.Sprintf("%s.md", post.Slug)
	return filepath.Join(r.rootPath, filename)
}

// matchesFilter проверяет, соответствует ли пост фильтрам.
func matchesFilter(post *core.Post, filter core.PostFilter) bool {
	// Фильтр по статусам
	if len(filter.Statuses) > 0 {
		found := false
		for _, s := range filter.Statuses {
			if post.Status == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Фильтр по платформам
	if len(filter.Platforms) > 0 {
		found := false
		for _, p := range filter.Platforms {
			for _, postP := range post.Platforms {
				if p == postP {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Фильтр по тегам
	if len(filter.Tags) > 0 {
		tagSet := make(map[string]bool)
		for _, t := range post.Tags {
			tagSet[strings.ToLower(t)] = true
		}
		found := false
		for _, t := range filter.Tags {
			if tagSet[strings.ToLower(t)] {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Поиск по заголовку/slug
	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(post.Title), search) &&
			!strings.Contains(strings.ToLower(post.Slug), search) {
			return false
		}
	}

	return true
}

// ParsePost парсит Markdown файл с YAML frontmatter.
func ParsePost(data []byte) (*core.Post, error) {
	// Разделяем frontmatter и контент
	content := bytes.TrimPrefix(data, []byte("---\n"))
	parts := bytes.SplitN(content, []byte("\n---\n"), 2)

	if len(parts) < 2 {
		return nil, fmt.Errorf("неверный формат frontmatter: ожидается --- ... ---")
	}

	frontmatter := parts[0]
	var body []byte
	if len(parts) > 1 {
		body = parts[1]
	}

	var post core.Post
	if err := yaml.Unmarshal(frontmatter, &post); err != nil {
		return nil, fmt.Errorf("ошибка парсинга YAML: %w", err)
	}

	post.Content = strings.TrimSpace(string(body))

	// Инициализируем пустые слайсы
	if post.Platforms == nil {
		post.Platforms = []core.Platform{}
	}
	if post.Tags == nil {
		post.Tags = []string{}
	}

	return &post, nil
}

// SerializePost сериализует пост в Markdown с YAML frontmatter.
func SerializePost(post *core.Post) ([]byte, error) {
	var buf bytes.Buffer

	buf.WriteString("---\n")
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(post); err != nil {
		return nil, err
	}
	encoder.Close()

	buf.WriteString("---\n\n")
	buf.WriteString(post.Content)

	return buf.Bytes(), nil
}
