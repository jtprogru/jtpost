// Package sqlite предоставляет SQLite-адаптер core.PostRepository.
//
// Реализация построена поверх sqlc-сгенерированного слоя в `sqlitedb` и
// goose-миграций, встроенных через embed.FS.
package sqlite

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pressly/goose/v3"

	"github.com/jtprogru/jtpost/internal/adapters/sqlite/sqlitedb"
	"github.com/jtprogru/jtpost/internal/core"

	_ "modernc.org/sqlite" // required for database/sql driver registration
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// PostRepository реализация core.PostRepository поверх SQLite + sqlc.
type PostRepository struct {
	db      *sql.DB
	queries *sqlitedb.Queries
}

// Config конфигурация SQLite репозитория.
type Config struct {
	// DSN строка подключения SQLite (путь к .db-файлу или ":memory:").
	DSN string
}

// txKey ключ для хранения транзакции в контексте.
type txKey struct{}

// NewSQLitePostRepository открывает БД, применяет миграции и возвращает репозиторий.
func NewSQLitePostRepository(cfg Config) (*PostRepository, error) {
	dsn := withSQLitePragmas(cfg.DSN)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("ошибка подключения к SQLite: %w", err)
	}
	// SQLite — single-writer; ограничиваем пул одной connection чтобы избежать
	// SQLITE_BUSY race между конкурентными writers (async session-touch +
	// RefreshCSRF и т.п.). Pragma в DSN тоже применяется per-connection.
	db.SetMaxOpenConns(1)

	if err := applyMigrations(db); err != nil {
		_ = db.Close()
		return nil, errors.Join(core.ErrMigrationFailed, err)
	}

	return &PostRepository{
		db:      db,
		queries: sqlitedb.New(db),
	}, nil
}

// withSQLitePragmas подмешивает критичные PRAGMA-параметры в DSN, чтобы они
// применялись ко всем connection из пула (а не только к первой).
//
// _pragma=foreign_keys(1)         — включить FK enforcement;
// _pragma=busy_timeout(5000)      — ждать до 5s при contention;
// _pragma=journal_mode(WAL)       — WAL даёт concurrent reads + 1 writer;
// _txlock=immediate               — write transactions сразу claim'ят lock.
func withSQLitePragmas(dsn string) string {
	if dsn == ":memory:" || dsn == "" {
		// In-memory: префикс file: + URI-mode для ?-параметров.
		return "file::memory:?cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate"
	}
	sep := "?"
	if strings.ContainsRune(dsn, '?') {
		sep = "&"
	}
	return dsn + sep + "_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_txlock=immediate"
}

// applyMigrations выполняет goose.UpContext поверх встроенных миграций.
func applyMigrations(db *sql.DB) error {
	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return fmt.Errorf("ошибка установки goose dialect: %w", err)
	}
	if err := goose.UpContext(context.Background(), db, "migrations"); err != nil {
		return fmt.Errorf("ошибка применения миграций: %w", err)
	}
	return nil
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
		return tx.Commit()
	}

	ctxWithTx := context.WithValue(ctx, txKey{}, tx)
	return ctxWithTx, commit, nil
}

// GetByID возвращает пост по идентификатору.
func (r *PostRepository) GetByID(ctx context.Context, id core.PostID) (*core.Post, error) {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return nil, core.ErrTenantMismatch
	}

	row, err := r.queries.GetPostByIDInTenant(ctx, sqlitedb.GetPostByIDInTenantParams{
		ID:       id.String(),
		TenantID: tenantID.String(),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return rowToPost(row)
}

// GetBySlug возвращает пост по slug в рамках tenant'а из контекста.
func (r *PostRepository) GetBySlug(ctx context.Context, slug string) (*core.Post, error) {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return nil, core.ErrTenantMismatch
	}

	row, err := r.queries.GetPostBySlug(ctx, sqlitedb.GetPostBySlugParams{
		TenantID: tenantID.String(),
		Slug:     slug,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return rowToPost(row)
}

// List реализует расширенную фильтрацию/сортировку/пагинацию через
// динамический SQL поверх *sql.DB (sqlc-набор не покрывает динамические
// предикаты).
func (r *PostRepository) List(ctx context.Context, filter core.PostFilter) ([]*core.Post, error) {
	if filter.TenantID == uuid.Nil {
		return nil, fmt.Errorf("%w: tenant_id required", core.ErrValidation)
	}
	if filter.SortBy != "" && !core.IsValidSortKey(filter.SortBy) {
		return nil, fmt.Errorf("%w: invalid sort key %q", core.ErrValidation, filter.SortBy)
	}

	var (
		sb   strings.Builder
		args []any
	)
	sb.WriteString(`
SELECT id, tenant_id, author_id, title, slug, status,
       tags, deadline, scheduled_at, published_at,
       excerpt, cover_image, attachments, publish_history,
       revision, revision_sha, content, telegram_url,
       created_at, updated_at
FROM posts
WHERE tenant_id = ?`)
	args = append(args, filter.TenantID.String())

	if filter.AuthorID != nil && *filter.AuthorID != uuid.Nil {
		sb.WriteString(" AND author_id = ?")
		args = append(args, filter.AuthorID.String())
	}

	if len(filter.Statuses) > 0 {
		placeholders := make([]string, len(filter.Statuses))
		for i, st := range filter.Statuses {
			placeholders[i] = "?"
			args = append(args, string(st))
		}
		sb.WriteString(" AND status IN (")
		sb.WriteString(strings.Join(placeholders, ","))
		sb.WriteString(")")
	}

	for _, tag := range filter.Tags {
		sb.WriteString(" AND tags LIKE ?")
		args = append(args, "%"+tag+"%")
	}

	if filter.Search != "" {
		sb.WriteString(" AND (title LIKE ? OR content LIKE ?)")
		s := "%" + filter.Search + "%"
		args = append(args, s, s)
	}

	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	sortOrder := strings.ToUpper(filter.SortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		if filter.SortBy == "" {
			sortOrder = "DESC"
		} else {
			sortOrder = "ASC"
		}
	}
	fmt.Fprintf(&sb, " ORDER BY %s %s", sortBy, sortOrder)

	if filter.Limit > 0 {
		sb.WriteString(" LIMIT ?")
		args = append(args, filter.Limit)
	}
	if filter.Offset > 0 {
		sb.WriteString(" OFFSET ?")
		args = append(args, filter.Offset)
	}

	rows, err := r.db.QueryContext(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := make([]*core.Post, 0)
	for rows.Next() {
		var m sqlitedb.Post
		if err := rows.Scan(
			&m.ID, &m.TenantID, &m.AuthorID, &m.Title, &m.Slug, &m.Status,
			&m.Tags, &m.Deadline, &m.ScheduledAt, &m.PublishedAt,
			&m.Excerpt, &m.CoverImage, &m.Attachments, &m.PublishHistory,
			&m.Revision, &m.RevisionSha, &m.Content, &m.TelegramUrl,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p, err := rowToPost(m)
		if err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return posts, nil
}

// Create создаёт новый пост.
func (r *PostRepository) Create(ctx context.Context, post *core.Post) error {
	params, err := postToCreateParams(post)
	if err != nil {
		return err
	}
	return r.queries.CreatePost(ctx, params)
}

// Update обновляет существующий пост с проверкой optimistic-lock через revision.
func (r *PostRepository) Update(ctx context.Context, post *core.Post) error {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return core.ErrTenantMismatch
	}

	tags, err := json.Marshal(safeTags(post.Tags))
	if err != nil {
		return err
	}
	attachments, err := json.Marshal(safeAttachments(post.Attachments))
	if err != nil {
		return err
	}
	publishHistory, err := json.Marshal(safePublishHistory(post.PublishHistory))
	if err != nil {
		return err
	}
	coverImage, err := marshalCoverImage(post.CoverImage)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	rows, err := r.queries.UpdatePost(ctx, sqlitedb.UpdatePostParams{
		AuthorID:       post.AuthorID.String(),
		Title:          post.Title,
		Slug:           post.Slug,
		Status:         string(post.Status),
		Tags:           string(tags),
		Deadline:       nullableTime(post.Deadline),
		ScheduledAt:    nullableTime(post.ScheduledAt),
		PublishedAt:    nullableTime(post.PublishedAt),
		Excerpt:        nullableStringPtr(post.Excerpt),
		CoverImage:     coverImage,
		Attachments:    string(attachments),
		PublishHistory: string(publishHistory),
		Revision:       int64(post.Revision),
		RevisionSha:    nullableStringPtr(post.RevisionSHA),
		Content:        post.Content,
		TelegramUrl:    nullableString(post.External.TelegramURL),
		UpdatedAt:      now,
		ID:             post.ID.String(),
		TenantID:       tenantID.String(),
		Revision_2:     int64(post.Revision - 1),
	})
	if err != nil {
		return err
	}

	if rows == 0 {
		exists, exErr := r.queries.PostExistsByID(ctx, post.ID.String())
		if exErr != nil {
			return exErr
		}
		if exists {
			return core.ErrConflict
		}
		return core.ErrNotFound
	}

	return nil
}

// Delete удаляет пост в рамках tenant'а из контекста. Идемпотентен.
func (r *PostRepository) Delete(ctx context.Context, id core.PostID) error {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return core.ErrTenantMismatch
	}
	_, err := r.queries.DeletePost(ctx, sqlitedb.DeletePostParams{
		ID:       id.String(),
		TenantID: tenantID.String(),
	})
	return err
}

// ImportPosts импортирует посты в одной транзакции через UPSERT.
func (r *PostRepository) ImportPosts(ctx context.Context, posts []*core.Post) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("ошибка начала транзакции: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	q := r.queries.WithTx(tx)
	for _, post := range posts {
		params, err := postToUpsertParams(post)
		if err != nil {
			return err
		}
		if err := q.UpsertPost(ctx, params); err != nil {
			return fmt.Errorf("ошибка импорта поста %s: %w", post.ID, err)
		}
	}
	return tx.Commit()
}

// Count возвращает общее количество постов.
func (r *PostRepository) Count(ctx context.Context) (int64, error) {
	return r.queries.CountPosts(ctx)
}

// ---------- helpers: marshalling ----------

func safeTags(t []string) []string {
	if t == nil {
		return []string{}
	}
	return t
}

func safeAttachments(a []core.Attachment) []core.Attachment {
	if a == nil {
		return []core.Attachment{}
	}
	return a
}

func safePublishHistory(p []core.PublishAttempt) []core.PublishAttempt {
	if p == nil {
		return []core.PublishAttempt{}
	}
	return p
}

func marshalCoverImage(c *core.Attachment) (sql.NullString, error) {
	if c == nil {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(c)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func nullableString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}

func nullableStringPtr(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}

func nullableTime(t *time.Time) sql.NullString {
	if t == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: t.UTC().Format(time.RFC3339), Valid: true}
}

func parseTime(s string) (*time.Time, error) {
	if s == "" {
		return nil, nil //nolint:nilnil // empty input → nil time, no error (optional field)
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseTimeNS(ns sql.NullString) (*time.Time, error) {
	if !ns.Valid || ns.String == "" {
		return nil, nil //nolint:nilnil // null/empty → nil time, no error
	}
	return parseTime(ns.String)
}

// rowToPost конвертирует sqlitedb.Post в *core.Post.
func rowToPost(row sqlitedb.Post) (*core.Post, error) {
	id, err := core.ParsePostID(row.ID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid post id: %w", err))
	}
	tenantID, err := uuid.Parse(row.TenantID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid tenant_id: %w", err))
	}
	authorID, err := uuid.Parse(row.AuthorID)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid author_id: %w", err))
	}

	createdAt, err := time.Parse(time.RFC3339, row.CreatedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid created_at: %w", err))
	}
	updatedAt, err := time.Parse(time.RFC3339, row.UpdatedAt)
	if err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid updated_at: %w", err))
	}

	post := &core.Post{
		ID:        id,
		TenantID:  tenantID,
		AuthorID:  authorID,
		Title:     row.Title,
		Slug:      row.Slug,
		Status:    core.PostStatus(row.Status),
		Revision:  int(row.Revision),
		Content:   row.Content,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}

	if row.Tags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(row.Tags), &tags); err != nil {
			return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid tags json: %w", err))
		}
		post.Tags = tags
	}

	if row.Attachments != "" {
		var atts []core.Attachment
		if err := json.Unmarshal([]byte(row.Attachments), &atts); err != nil {
			return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid attachments json: %w", err))
		}
		post.Attachments = atts
	}

	if row.PublishHistory != "" {
		var ph []core.PublishAttempt
		if err := json.Unmarshal([]byte(row.PublishHistory), &ph); err != nil {
			return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid publish_history json: %w", err))
		}
		post.PublishHistory = ph
	}

	if row.CoverImage.Valid && row.CoverImage.String != "" {
		var ci core.Attachment
		if err := json.Unmarshal([]byte(row.CoverImage.String), &ci); err != nil {
			return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid cover_image json: %w", err))
		}
		post.CoverImage = &ci
	}

	if t, err := parseTimeNS(row.Deadline); err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid deadline: %w", err))
	} else if t != nil {
		post.Deadline = t
	}
	if t, err := parseTimeNS(row.ScheduledAt); err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid scheduled_at: %w", err))
	} else if t != nil {
		post.ScheduledAt = t
	}
	if t, err := parseTimeNS(row.PublishedAt); err != nil {
		return nil, errors.Join(core.ErrValidation, fmt.Errorf("invalid published_at: %w", err))
	} else if t != nil {
		post.PublishedAt = t
	}

	if row.Excerpt.Valid {
		s := row.Excerpt.String
		post.Excerpt = &s
	}
	if row.RevisionSha.Valid {
		s := row.RevisionSha.String
		post.RevisionSHA = &s
	}
	if row.TelegramUrl.Valid {
		post.External.TelegramURL = row.TelegramUrl.String
	}

	return post, nil
}

func postToCreateParams(post *core.Post) (sqlitedb.CreatePostParams, error) {
	tags, err := json.Marshal(safeTags(post.Tags))
	if err != nil {
		return sqlitedb.CreatePostParams{}, err
	}
	attachments, err := json.Marshal(safeAttachments(post.Attachments))
	if err != nil {
		return sqlitedb.CreatePostParams{}, err
	}
	publishHistory, err := json.Marshal(safePublishHistory(post.PublishHistory))
	if err != nil {
		return sqlitedb.CreatePostParams{}, err
	}
	coverImage, err := marshalCoverImage(post.CoverImage)
	if err != nil {
		return sqlitedb.CreatePostParams{}, err
	}

	createdAt := post.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	updatedAt := post.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = createdAt
	}
	revision := post.Revision
	if revision == 0 {
		revision = 1
	}

	return sqlitedb.CreatePostParams{
		ID:             post.ID.String(),
		TenantID:       post.TenantID.String(),
		AuthorID:       post.AuthorID.String(),
		Title:          post.Title,
		Slug:           post.Slug,
		Status:         string(post.Status),
		Tags:           string(tags),
		Deadline:       nullableTime(post.Deadline),
		ScheduledAt:    nullableTime(post.ScheduledAt),
		PublishedAt:    nullableTime(post.PublishedAt),
		Excerpt:        nullableStringPtr(post.Excerpt),
		CoverImage:     coverImage,
		Attachments:    string(attachments),
		PublishHistory: string(publishHistory),
		Revision:       int64(revision),
		RevisionSha:    nullableStringPtr(post.RevisionSHA),
		Content:        post.Content,
		TelegramUrl:    nullableString(post.External.TelegramURL),
		CreatedAt:      createdAt.UTC().Format(time.RFC3339),
		UpdatedAt:      updatedAt.UTC().Format(time.RFC3339),
	}, nil
}

func postToUpsertParams(post *core.Post) (sqlitedb.UpsertPostParams, error) {
	cp, err := postToCreateParams(post)
	if err != nil {
		return sqlitedb.UpsertPostParams{}, err
	}
	return sqlitedb.UpsertPostParams(cp), nil
}
