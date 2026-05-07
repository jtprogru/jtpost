package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // database/sql driver "pgx" for goose migrations.
	"github.com/jtprogru/jtpost/internal/adapters/postgres/pgdb"
	"github.com/jtprogru/jtpost/internal/core"
	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Config настройки Postgres-репозитория.
type Config struct {
	DSN             string
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// PostRepository реализует core.PostRepository поверх pgxpool.Pool.
type PostRepository struct {
	pool    *pgxpool.Pool
	queries *pgdb.Queries
}

// NewPostgresRepository создаёт новый Postgres-репозиторий: парсит DSN, открывает пул,
// делает синхронный Ping, применяет миграции через goose.
func NewPostgresRepository(ctx context.Context, cfg Config) (*PostRepository, error) {
	if cfg.DSN == "" {
		return nil, errors.Join(core.ErrConfigInvalid, errors.New("postgres dsn required"))
	}

	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, errors.Join(core.ErrConfigInvalid, err)
	}

	if cfg.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		poolCfg.MinConns = int32(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, errors.Join(core.ErrConfigInvalid, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, errors.Join(core.ErrConfigInvalid, err)
	}

	// Apply migrations via goose using a separate *sql.DB.
	if err := applyMigrations(ctx, cfg.DSN); err != nil {
		pool.Close()
		return nil, err
	}

	return &PostRepository{
		pool:    pool,
		queries: pgdb.New(pool),
	}, nil
}

// applyMigrations открывает database/sql на pgx-stdlib и запускает goose.Up.
func applyMigrations(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return errors.Join(core.ErrMigrationFailed, err)
	}
	defer db.Close()

	goose.SetBaseFS(migrationsFS)
	if err := goose.SetDialect("postgres"); err != nil {
		return errors.Join(core.ErrMigrationFailed, err)
	}
	if err := goose.UpContext(ctx, db, "migrations"); err != nil {
		return errors.Join(core.ErrMigrationFailed, err)
	}
	return nil
}

// Close закрывает pgxpool.Pool.
func (r *PostRepository) Close() error {
	r.pool.Close()
	return nil
}

// ---- helpers ----

//nolint:funcorder // helper conversion sits in dedicated section with its peers.
func (r *PostRepository) postFromRow(row pgdb.Post) (*core.Post, error) {
	post := &core.Post{
		ID:          core.PostID(fromPgUUID(row.ID)),
		TenantID:    fromPgUUID(row.TenantID),
		AuthorID:    fromPgUUID(row.AuthorID),
		Title:       row.Title,
		Slug:        row.Slug,
		Status:      core.PostStatus(row.Status),
		Deadline:    fromPgTimestamp(row.Deadline),
		ScheduledAt: fromPgTimestamp(row.ScheduledAt),
		PublishedAt: fromPgTimestamp(row.PublishedAt),
		Excerpt:     fromPgText(row.Excerpt),
		Revision:    int(row.Revision),
		RevisionSHA: fromPgText(row.RevisionSha),
		Content:     row.Content,
		External:    core.ExternalLinks{TelegramURL: fromPgTextStr(row.TelegramUrl)},
		CreatedAt:   row.CreatedAt.Time,
		UpdatedAt:   row.UpdatedAt.Time,
	}
	if err := unmarshalJSON(row.Tags, &post.Tags); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(row.Attachments, &post.Attachments); err != nil {
		return nil, err
	}
	if err := unmarshalJSON(row.PublishHistory, &post.PublishHistory); err != nil {
		return nil, err
	}
	if len(row.CoverImage) > 0 && string(row.CoverImage) != "null" {
		var cover core.Attachment
		if err := unmarshalJSON(row.CoverImage, &cover); err != nil {
			return nil, err
		}
		post.CoverImage = &cover
	}
	return post, nil
}

func postToCreateParams(p *core.Post) (pgdb.CreatePostParams, error) {
	tagsJSON, err := marshalJSONArray(p.Tags)
	if err != nil {
		return pgdb.CreatePostParams{}, errors.Join(core.ErrValidation, err)
	}
	attachJSON, err := marshalJSONArray(p.Attachments)
	if err != nil {
		return pgdb.CreatePostParams{}, errors.Join(core.ErrValidation, err)
	}
	historyJSON, err := marshalJSONArray(p.PublishHistory)
	if err != nil {
		return pgdb.CreatePostParams{}, errors.Join(core.ErrValidation, err)
	}
	var coverJSON []byte
	if p.CoverImage != nil {
		coverJSON, err = marshalJSON(p.CoverImage)
		if err != nil {
			return pgdb.CreatePostParams{}, errors.Join(core.ErrValidation, err)
		}
	}
	return pgdb.CreatePostParams{
		ID:             toPgUUID(uuid.UUID(p.ID)),
		TenantID:       toPgUUID(p.TenantID),
		AuthorID:       toPgUUID(p.AuthorID),
		Title:          p.Title,
		Slug:           p.Slug,
		Status:         string(p.Status),
		Tags:           tagsJSON,
		Deadline:       toPgTimestamp(p.Deadline),
		ScheduledAt:    toPgTimestamp(p.ScheduledAt),
		PublishedAt:    toPgTimestamp(p.PublishedAt),
		Excerpt:        toPgText(p.Excerpt),
		CoverImage:     coverJSON,
		Attachments:    attachJSON,
		PublishHistory: historyJSON,
		Revision:       int32(p.Revision),
		RevisionSha:    toPgText(p.RevisionSHA),
		Content:        p.Content,
		TelegramUrl:    toPgTextStr(p.External.TelegramURL),
		CreatedAt:      toPgTimestampVal(p.CreatedAt),
		UpdatedAt:      toPgTimestampVal(p.UpdatedAt),
	}, nil
}

// ---- CRUD ----

// GetByID возвращает пост по ID; tenant из ctx; чужой/несуществующий → ErrNotFound.
func (r *PostRepository) GetByID(ctx context.Context, id core.PostID) (*core.Post, error) {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return nil, core.ErrTenantMismatch
	}
	row, err := r.queries.GetPostByIDInTenant(ctx, pgdb.GetPostByIDInTenantParams{
		ID:       toPgUUID(uuid.UUID(id)),
		TenantID: toPgUUID(tenantID),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return r.postFromRow(row)
}

// GetBySlug возвращает пост по slug в рамках tenant из ctx.
func (r *PostRepository) GetBySlug(ctx context.Context, slug string) (*core.Post, error) {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return nil, core.ErrTenantMismatch
	}
	row, err := r.queries.GetPostBySlug(ctx, pgdb.GetPostBySlugParams{
		TenantID: toPgUUID(tenantID),
		Slug:     slug,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, core.ErrNotFound
		}
		return nil, err
	}
	return r.postFromRow(row)
}

// List возвращает посты с фильтром/сортировкой/пагинацией.
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
	sb.WriteString("SELECT id, tenant_id, author_id, title, slug, status, tags, deadline, scheduled_at, published_at, excerpt, cover_image, attachments, publish_history, revision, revision_sha, content, telegram_url, created_at, updated_at FROM posts WHERE tenant_id = $1")
	args = append(args, toPgUUID(filter.TenantID))

	if filter.AuthorID != nil {
		args = append(args, toPgUUID(*filter.AuthorID))
		fmt.Fprintf(&sb, " AND author_id = $%d", len(args))
	}

	if len(filter.Statuses) > 0 {
		placeholders := make([]string, 0, len(filter.Statuses))
		for _, s := range filter.Statuses {
			args = append(args, string(s))
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		fmt.Fprintf(&sb, " AND status IN (%s)", strings.Join(placeholders, ","))
	}

	if filter.Search != "" {
		args = append(args, "%"+strings.ToLower(filter.Search)+"%")
		fmt.Fprintf(&sb, " AND (LOWER(title) LIKE $%d OR LOWER(slug) LIKE $%d)", len(args), len(args))
	}

	if len(filter.Tags) > 0 {
		// jsonb ?| array — any of tags present in jsonb array.
		tagsArr := make([]string, len(filter.Tags))
		copy(tagsArr, filter.Tags)
		args = append(args, tagsArr)
		fmt.Fprintf(&sb, " AND tags ?| $%d", len(args))
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
		args = append(args, filter.Limit)
		fmt.Fprintf(&sb, " LIMIT $%d", len(args))
	}
	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		fmt.Fprintf(&sb, " OFFSET $%d", len(args))
	}

	rows, err := r.pool.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	posts := []*core.Post{}
	for rows.Next() {
		var row pgdb.Post
		if err := rows.Scan(
			&row.ID, &row.TenantID, &row.AuthorID, &row.Title, &row.Slug, &row.Status,
			&row.Tags, &row.Deadline, &row.ScheduledAt, &row.PublishedAt,
			&row.Excerpt, &row.CoverImage, &row.Attachments, &row.PublishHistory,
			&row.Revision, &row.RevisionSha, &row.Content, &row.TelegramUrl,
			&row.CreatedAt, &row.UpdatedAt,
		); err != nil {
			return nil, err
		}
		p, err := r.postFromRow(row)
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
	if err := r.queries.CreatePost(ctx, params); err != nil {
		return err
	}
	return nil
}

// Update обновляет пост с проверкой optimistic-lock через revision-1.
func (r *PostRepository) Update(ctx context.Context, post *core.Post) error {
	tagsJSON, err := marshalJSONArray(post.Tags)
	if err != nil {
		return errors.Join(core.ErrValidation, err)
	}
	attachJSON, err := marshalJSONArray(post.Attachments)
	if err != nil {
		return errors.Join(core.ErrValidation, err)
	}
	historyJSON, err := marshalJSONArray(post.PublishHistory)
	if err != nil {
		return errors.Join(core.ErrValidation, err)
	}
	var coverJSON []byte
	if post.CoverImage != nil {
		coverJSON, err = marshalJSON(post.CoverImage)
		if err != nil {
			return errors.Join(core.ErrValidation, err)
		}
	}

	params := pgdb.UpdatePostParams{
		ID:             toPgUUID(uuid.UUID(post.ID)),
		AuthorID:       toPgUUID(post.AuthorID),
		Title:          post.Title,
		Slug:           post.Slug,
		Status:         string(post.Status),
		Tags:           tagsJSON,
		Deadline:       toPgTimestamp(post.Deadline),
		ScheduledAt:    toPgTimestamp(post.ScheduledAt),
		PublishedAt:    toPgTimestamp(post.PublishedAt),
		Excerpt:        toPgText(post.Excerpt),
		CoverImage:     coverJSON,
		Attachments:    attachJSON,
		PublishHistory: historyJSON,
		Revision:       int32(post.Revision),
		RevisionSha:    toPgText(post.RevisionSHA),
		Content:        post.Content,
		TelegramUrl:    toPgTextStr(post.External.TelegramURL),
		UpdatedAt:      toPgTimestampVal(post.UpdatedAt),
		TenantID:       toPgUUID(post.TenantID),
		Revision_2:     int32(post.Revision - 1),
	}
	rows, err := r.queries.UpdatePost(ctx, params)
	if err != nil {
		return err
	}
	if rows == 0 {
		exists, exErr := r.queries.PostExistsByID(ctx, toPgUUID(uuid.UUID(post.ID)))
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

// Delete удаляет пост в рамках tenant из ctx.
func (r *PostRepository) Delete(ctx context.Context, id core.PostID) error {
	tenantID, ok := core.TenantFromContext(ctx)
	if !ok {
		return core.ErrTenantMismatch
	}
	rows, err := r.queries.DeletePost(ctx, pgdb.DeletePostParams{
		ID:       toPgUUID(uuid.UUID(id)),
		TenantID: toPgUUID(tenantID),
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return core.ErrNotFound
	}
	return nil
}

// ImportPosts импортирует слайс постов в одной транзакции через UpsertPost.
func (r *PostRepository) ImportPosts(ctx context.Context, posts []*core.Post) error {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	q := r.queries.WithTx(tx)
	for _, p := range posts {
		params, err := postToCreateParams(p)
		if err != nil {
			return err
		}
		upsert := pgdb.UpsertPostParams(params)
		if err := q.UpsertPost(ctx, upsert); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// Count возвращает общее количество постов.
func (r *PostRepository) Count(ctx context.Context) (int64, error) {
	return r.queries.CountPosts(ctx)
}

// BeginTx начинает транзакцию. Возвращает новый ctx (без подмены), commit-функцию и ошибку.
// Закрытие транзакции через возвращённую функцию (commit on success).
//
// Note: для простоты F2 реализуется как обёртка; репозиторий продолжает писать через r.pool —
// тонкая транзакционная обёртка для compatibility с TransactionalRepository.
func (r *PostRepository) BeginTx(ctx context.Context) (context.Context, func() error, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return ctx, nil, err
	}
	commit := func() error {
		return tx.Commit(ctx)
	}
	return ctx, commit, nil
}
