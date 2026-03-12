package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jtprogru/jtpost/internal/core"
	_ "modernc.org/sqlite" // required for database/sql driver registration
)

// PostRepository реализация PostRepository на основе SQLite.
type PostRepository struct {
	db *sql.DB
}

// Config конфигурация SQLite репозитория.
type Config struct {
	// DSN строка подключения к базе данных (путь к файлу .db)
	DSN string
}

// NewSQLitePostRepository создаёт новый SQLite репозиторий.
// Возвращает PostRepository для совместимости с интерфейсом core.PostRepository.
func NewSQLitePostRepository(cfg Config) (*PostRepository, error) {
	db, err := sql.Open("sqlite", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к SQLite: %w", err)
	}

	// Включаем поддержку внешних ключей
	if _, err := db.ExecContext(context.Background(), "PRAGMA foreign_keys = ON"); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка включения foreign keys: %w", err)
	}

	repo := &PostRepository{db: db}

	// Выполняем миграции
	if err := repo.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ошибка миграции: %w", err)
	}

	return repo, nil
}

// Close закрывает подключение к базе данных.
func (r *PostRepository) Close() error {
	return r.db.Close()
}

// BeginTx начинает транзакцию.
func (r *PostRepository) BeginTx(ctx context.Context) (context.Context, func() error, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("ошибка начала транзакции: %w", err)
	}

	commit := func() error {
		if err := tx.Commit(); err != nil {
			return err
		}
		return nil
	}

	// Возвращаем контекст с транзакцией
	ctxWithTx := context.WithValue(ctx, txKey{}, tx)

	return ctxWithTx, commit, nil
}

// GetByID возвращает пост по идентификатору.
func (r *PostRepository) GetByID(ctx context.Context, id core.PostID) (*core.Post, error) {
	query := `
	SELECT id, title, slug, status, platforms, tags, deadline, scheduled_at,
	       published_at, content, telegram_url, created_at, updated_at
	FROM posts WHERE id = ?
	`

	row := r.db.QueryRowContext(ctx, query, id)
	return scanPost(row)
}

// GetBySlug возвращает пост по slug.
func (r *PostRepository) GetBySlug(ctx context.Context, slug string) (*core.Post, error) {
	query := `
	SELECT id, title, slug, status, platforms, tags, deadline, scheduled_at,
	       published_at, content, telegram_url, created_at, updated_at
	FROM posts WHERE slug = ?
	`

	row := r.db.QueryRowContext(ctx, query, slug)
	return scanPost(row)
}

// List возвращает список постов с применением фильтров.
func (r *PostRepository) List(ctx context.Context, filter core.PostFilter) ([]*core.Post, error) {
	query := &strings.Builder{}
	query.WriteString(`
SELECT id, title, slug, status, platforms, tags, deadline, scheduled_at,
       published_at, content, telegram_url, created_at, updated_at
FROM posts WHERE 1=1
`)

	args := make([]any, 0)

	// Фильтр по статусам
	if len(filter.Statuses) > 0 {
		placeholders := make([]string, len(filter.Statuses))
		for i, status := range filter.Statuses {
			placeholders[i] = "?"
			args = append(args, string(status))
		}
		fmt.Fprintf(query, " AND status IN (%s)", joinStrings(placeholders))
	}

	// Фильтр по тегам
	if len(filter.Tags) > 0 {
		for _, tag := range filter.Tags {
			query.WriteString(" AND tags LIKE ?")
			args = append(args, "%"+tag+"%")
		}
	}

	// Поиск по заголовку и содержимому
	if filter.Search != "" {
		query.WriteString(" AND (title LIKE ? OR content LIKE ?)")
		searchTerm := "%" + filter.Search + "%"
		args = append(args, searchTerm, searchTerm)
	}

	query.WriteString(" ORDER BY created_at DESC")

	rows, err := r.db.QueryContext(ctx, query.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]*core.Post, 0)
	for rows.Next() {
		post, err := scanPostRow(rows)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return posts, nil
}

// Create создаёт новый пост.
func (r *PostRepository) Create(ctx context.Context, post *core.Post) error {
	query := `
	INSERT INTO posts (id, title, slug, status, platforms, tags, deadline,
	                   scheduled_at, published_at, content, telegram_url, created_at, updated_at)
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	now := time.Now().Format(time.RFC3339)

	platformsJSON, err := json.Marshal(post.Platforms)
	if err != nil {
		return err
	}

	tagsJSON, err := json.Marshal(post.Tags)
	if err != nil {
		return err
	}

	deadline := ""
	if post.Deadline != nil {
		deadline = post.Deadline.Format(time.RFC3339)
	}

	scheduledAt := ""
	if post.ScheduledAt != nil {
		scheduledAt = post.ScheduledAt.Format(time.RFC3339)
	}

	publishedAt := ""
	if post.PublishedAt != nil {
		publishedAt = post.PublishedAt.Format(time.RFC3339)
	}

	_, err = r.db.ExecContext(ctx, query,
		post.ID, post.Title, post.Slug, string(post.Status),
		string(platformsJSON), string(tagsJSON),
		deadline, scheduledAt, publishedAt,
		post.Content, post.External.TelegramURL,
		now, now,
	)

	return err
}

// Update обновляет существующий пост.
func (r *PostRepository) Update(ctx context.Context, post *core.Post) error {
	query := `
	UPDATE posts
	SET title = ?, slug = ?, status = ?, platforms = ?, tags = ?,
	    deadline = ?, scheduled_at = ?, published_at = ?,
	    content = ?, telegram_url = ?, updated_at = ?
	WHERE id = ?
	`

	now := time.Now().Format(time.RFC3339)

	platformsJSON, err := json.Marshal(post.Platforms)
	if err != nil {
		return err
	}

	tagsJSON, err := json.Marshal(post.Tags)
	if err != nil {
		return err
	}

	deadline := ""
	if post.Deadline != nil {
		deadline = post.Deadline.Format(time.RFC3339)
	}

	scheduledAt := ""
	if post.ScheduledAt != nil {
		scheduledAt = post.ScheduledAt.Format(time.RFC3339)
	}

	publishedAt := ""
	if post.PublishedAt != nil {
		publishedAt = post.PublishedAt.Format(time.RFC3339)
	}

	_, err = r.db.ExecContext(ctx, query,
		post.Title, post.Slug, string(post.Status),
		string(platformsJSON), string(tagsJSON),
		deadline, scheduledAt, publishedAt,
		post.Content, post.External.TelegramURL,
		now, post.ID,
	)

	return err
}

// Delete удаляет пост.
func (r *PostRepository) Delete(ctx context.Context, id core.PostID) error {
	query := `DELETE FROM posts WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

// ImportPosts импортирует посты из другого репозитория.
func (r *PostRepository) ImportPosts(ctx context.Context, posts []*core.Post) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT OR REPLACE INTO posts
		(id, title, slug, status, platforms, tags, deadline,
		 scheduled_at, published_at, content, telegram_url, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("ошибка подготовки statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now().Format(time.RFC3339)

	for _, post := range posts {
		platformsJSON, err := json.Marshal(post.Platforms)
		if err != nil {
			return err
		}

		tagsJSON, err := json.Marshal(post.Tags)
		if err != nil {
			return err
		}

		deadline := ""
		if post.Deadline != nil {
			deadline = post.Deadline.Format(time.RFC3339)
		}

		scheduledAt := ""
		if post.ScheduledAt != nil {
			scheduledAt = post.ScheduledAt.Format(time.RFC3339)
		}

		publishedAt := ""
		if post.PublishedAt != nil {
			publishedAt = post.PublishedAt.Format(time.RFC3339)
		}

		_, err = stmt.ExecContext(ctx,
			post.ID, post.Title, post.Slug, string(post.Status),
			string(platformsJSON), string(tagsJSON),
			deadline, scheduledAt, publishedAt,
			post.Content, post.External.TelegramURL,
			now, now,
		)
		if err != nil {
			return fmt.Errorf("ошибка импорта поста %s: %w", post.ID, err)
		}
	}

	return tx.Commit()
}

// Count возвращает количество постов в хранилище.
func (r *PostRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM posts").Scan(&count)
	return count, err
}

// txKey ключ для хранения транзакции в контексте.
type txKey struct{}

// scanPost сканирует пост из sql.Row.
func scanPost(row *sql.Row) (*core.Post, error) {
	var (
		id, title, slug, status, platformsJSON, tagsJSON string
		deadline, scheduledAt, publishedAt               string
		content, telegramURL, createdAt, updatedAt       string
	)

	err := row.Scan(&id, &title, &slug, &status, &platformsJSON, &tagsJSON,
		&deadline, &scheduledAt, &publishedAt, &content, &telegramURL,
		&createdAt, &updatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, core.ErrNotFound
		}
		return nil, err
	}

	var platforms []core.Platform
	if platformsJSON != "" {
		if err := json.Unmarshal([]byte(platformsJSON), &platforms); err != nil {
			platforms = []core.Platform{core.PlatformTelegram}
		}
	} else {
		platforms = []core.Platform{core.PlatformTelegram}
	}

	var tags []string
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &tags)
	}

	post := &core.Post{
		ID:        core.PostID(id),
		Title:     title,
		Slug:      slug,
		Status:    core.PostStatus(status),
		Platforms: platforms,
		Tags:      tags,
		Content:   content,
		External: core.ExternalLinks{
			TelegramURL: telegramURL,
		},
	}

	if deadline != "" {
		if t, err := time.Parse(time.RFC3339, deadline); err == nil {
			post.Deadline = &t
		}
	}

	if scheduledAt != "" {
		if t, err := time.Parse(time.RFC3339, scheduledAt); err == nil {
			post.ScheduledAt = &t
		}
	}

	if publishedAt != "" {
		if t, err := time.Parse(time.RFC3339, publishedAt); err == nil {
			post.PublishedAt = &t
		}
	}

	return post, nil
}

// scanPostRow сканирует пост из sql.Rows.
func scanPostRow(rows interface{ Scan(dest ...any) error }) (*core.Post, error) {
	var (
		id, title, slug, status, platformsJSON, tagsJSON string
		deadline, scheduledAt, publishedAt               string
		content, telegramURL, createdAt, updatedAt       string
	)

	err := rows.Scan(&id, &title, &slug, &status, &platformsJSON, &tagsJSON,
		&deadline, &scheduledAt, &publishedAt, &content, &telegramURL,
		&createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	var platforms []core.Platform
	if platformsJSON != "" {
		if err := json.Unmarshal([]byte(platformsJSON), &platforms); err != nil {
			platforms = []core.Platform{core.PlatformTelegram}
		}
	} else {
		platforms = []core.Platform{core.PlatformTelegram}
	}

	var tags []string
	if tagsJSON != "" {
		_ = json.Unmarshal([]byte(tagsJSON), &tags)
	}

	post := &core.Post{
		ID:        core.PostID(id),
		Title:     title,
		Slug:      slug,
		Status:    core.PostStatus(status),
		Platforms: platforms,
		Tags:      tags,
		Content:   content,
		External: core.ExternalLinks{
			TelegramURL: telegramURL,
		},
	}

	if deadline != "" {
		if t, err := time.Parse(time.RFC3339, deadline); err == nil {
			post.Deadline = &t
		}
	}

	if scheduledAt != "" {
		if t, err := time.Parse(time.RFC3339, scheduledAt); err == nil {
			post.ScheduledAt = &t
		}
	}

	if publishedAt != "" {
		if t, err := time.Parse(time.RFC3339, publishedAt); err == nil {
			post.PublishedAt = &t
		}
	}

	return post, nil
}

// joinStrings объединяет строки через запятую.
func joinStrings(strs []string) string {
	if len(strs) == 0 {
		return ""
	}
	var builder strings.Builder
	builder.Grow(len(strs) * 2) // Оценка 2 символа на элемент (запятая + пробел)
	builder.WriteString(strs[0])
	for i := 1; i < len(strs); i++ {
		builder.WriteString(", ")
		builder.WriteString(strs[i])
	}
	return builder.String()
}

// migrate выполняет миграцию схемы базы данных.
func (r *PostRepository) migrate() error {
	schema := `
	CREATE TABLE IF NOT EXISTS posts (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		slug TEXT NOT NULL UNIQUE,
		status TEXT NOT NULL,
		platforms TEXT,
		tags TEXT,
		deadline TEXT,
		scheduled_at TEXT,
		published_at TEXT,
		content TEXT NOT NULL,
		telegram_url TEXT,
		created_at TEXT NOT NULL,
		updated_at TEXT NOT NULL
	);

	CREATE INDEX IF NOT EXISTS idx_posts_status ON posts(status);
	CREATE INDEX IF NOT EXISTS idx_posts_slug ON posts(slug);
	CREATE INDEX IF NOT EXISTS idx_posts_platforms ON posts(platforms);
	`

	_, err := r.db.ExecContext(context.Background(), schema)
	return err
}
