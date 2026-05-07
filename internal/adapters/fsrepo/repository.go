package fsrepo

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/jtprogru/jtpost/internal/core"
)

// FileSystemPostRepository реализация PostRepository на основе файловой системы.
// Посты хранятся в подкаталогах: <rootPath>/<tenant_short_id>/<slug>.md.
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

// GetByID возвращает пост по идентификатору в рамках tenant'а из контекста.
func (r *FileSystemPostRepository) GetByID(ctx context.Context, id core.PostID) (*core.Post, error) {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return nil, core.ErrTenantMismatch
	}

	subdir := r.tenantSubdir(tenantID)
	entries, err := os.ReadDir(subdir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(subdir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		post, err := ParsePost(data)
		if err != nil {
			continue
		}

		if post.ID == id {
			if post.TenantID != tenantID {
				return nil, core.ErrNotFound
			}
			return post, nil
		}
	}

	return nil, core.ErrNotFound
}

// GetBySlug возвращает пост по slug в рамках tenant'а из контекста.
func (r *FileSystemPostRepository) GetBySlug(ctx context.Context, slug string) (*core.Post, error) {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return nil, core.ErrTenantMismatch
	}

	filePath := filepath.Join(r.tenantSubdir(tenantID), slug+".md")
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

	if post.TenantID != tenantID {
		return nil, core.ErrNotFound
	}

	return post, nil
}

// List возвращает список постов с применением фильтров и пагинации.
func (r *FileSystemPostRepository) List(_ context.Context, filter core.PostFilter) ([]*core.Post, error) {
	if filter.TenantID == uuid.Nil {
		return nil, fmt.Errorf("%w: tenant_id required", core.ErrValidation)
	}

	if filter.SortBy != "" && !core.IsValidSortKey(filter.SortBy) {
		return nil, fmt.Errorf("%w: invalid sort key %q", core.ErrValidation, filter.SortBy)
	}

	subdir := r.tenantSubdir(filter.TenantID)
	entries, err := os.ReadDir(subdir)
	if err != nil {
		if os.IsNotExist(err) {
			return []*core.Post{}, nil
		}
		return nil, err
	}

	var posts []*core.Post

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(subdir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		post, err := ParsePost(data)
		if err != nil {
			continue
		}

		if post.TenantID != filter.TenantID {
			continue
		}

		if !matchesFilter(post, filter) {
			continue
		}

		posts = append(posts, post)
	}

	applySort(posts, filter.SortBy, filter.SortOrder)
	posts = applyPaging(posts, filter.Offset, filter.Limit)

	if posts == nil {
		posts = []*core.Post{}
	}

	return posts, nil
}

// Create создаёт новый пост.
func (r *FileSystemPostRepository) Create(_ context.Context, post *core.Post) error {
	filePath := r.postPath(post)

	if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
		return fmt.Errorf("ошибка создания директории: %w", err)
	}

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
func (r *FileSystemPostRepository) Update(_ context.Context, post *core.Post) error {
	filePath := r.postPath(post)

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

// Delete удаляет пост по ID в рамках tenant'а из контекста.
func (r *FileSystemPostRepository) Delete(ctx context.Context, id core.PostID) error {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return core.ErrTenantMismatch
	}

	subdir := r.tenantSubdir(tenantID)
	entries, err := os.ReadDir(subdir)
	if err != nil {
		if os.IsNotExist(err) {
			return core.ErrNotFound
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		filePath := filepath.Join(subdir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		post, err := ParsePost(data)
		if err != nil {
			continue
		}

		if post.ID == id {
			if post.TenantID != tenantID {
				return core.ErrNotFound
			}
			return os.Remove(filePath)
		}
	}

	return core.ErrNotFound
}

// postPath возвращает путь к файлу поста с учётом tenant subdir.
func (r *FileSystemPostRepository) postPath(post *core.Post) string {
	return filepath.Join(r.rootPath, post.TenantShortID(), post.Slug+".md")
}

// tenantSubdir возвращает путь к подкаталогу tenant'а.
func (r *FileSystemPostRepository) tenantSubdir(tenantID uuid.UUID) string {
	short := strings.ReplaceAll(tenantID.String(), "-", "")
	if len(short) > 8 {
		short = short[:8]
	}
	return filepath.Join(r.rootPath, short)
}

// matchesFilter проверяет, соответствует ли пост фильтрам (без tenant — он применён выше).
func matchesFilter(post *core.Post, filter core.PostFilter) bool {
	// AuthorID
	if filter.AuthorID != nil && post.AuthorID != *filter.AuthorID {
		return false
	}

	// Statuses (AND ⇒ at least one match in filter set)
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

	// Tags (OR, case-insensitive)
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

	// Search в title/slug
	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		if !strings.Contains(strings.ToLower(post.Title), search) &&
			!strings.Contains(strings.ToLower(post.Slug), search) {
			return false
		}
	}

	return true
}

// applySort сортирует посты согласно sortBy/sortOrder. SortBy уже провалидирован.
func applySort(posts []*core.Post, sortBy, sortOrder string) {
	if sortBy == "" {
		return
	}

	desc := sortOrder == "desc"

	less := func(i, j int) bool { return false }
	switch sortBy {
	case "created_at":
		less = func(i, j int) bool { return posts[i].CreatedAt.Before(posts[j].CreatedAt) }
	case "updated_at":
		less = func(i, j int) bool { return posts[i].UpdatedAt.Before(posts[j].UpdatedAt) }
	case "deadline":
		less = func(i, j int) bool {
			if posts[i].Deadline == nil && posts[j].Deadline == nil {
				return false
			}
			if posts[i].Deadline == nil {
				return false
			}
			if posts[j].Deadline == nil {
				return true
			}
			return posts[i].Deadline.Before(*posts[j].Deadline)
		}
	case "scheduled_at":
		less = func(i, j int) bool {
			if posts[i].ScheduledAt == nil && posts[j].ScheduledAt == nil {
				return false
			}
			if posts[i].ScheduledAt == nil {
				return false
			}
			if posts[j].ScheduledAt == nil {
				return true
			}
			return posts[i].ScheduledAt.Before(*posts[j].ScheduledAt)
		}
	case "title":
		less = func(i, j int) bool { return posts[i].Title < posts[j].Title }
	case "status":
		less = func(i, j int) bool { return string(posts[i].Status) < string(posts[j].Status) }
	}

	sort.SliceStable(posts, func(i, j int) bool {
		if desc {
			return less(j, i)
		}
		return less(i, j)
	})
}

// applyPaging применяет Offset и Limit.
func applyPaging(posts []*core.Post, offset, limit int) []*core.Post {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(posts) {
		return []*core.Post{}
	}
	posts = posts[offset:]
	if limit > 0 && limit < len(posts) {
		posts = posts[:limit]
	}
	return posts
}
