-- name: CreatePost :exec
INSERT INTO posts (
    id, tenant_id, author_id, title, slug, status,
    tags, deadline, scheduled_at, published_at,
    excerpt, cover_image, attachments, publish_history,
    revision, revision_sha, content, telegram_url,
    created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?
);

-- name: GetPostByID :one
SELECT
    id, tenant_id, author_id, title, slug, status,
    tags, deadline, scheduled_at, published_at,
    excerpt, cover_image, attachments, publish_history,
    revision, revision_sha, content, telegram_url,
    created_at, updated_at
FROM posts
WHERE id = ?;

-- name: GetPostByIDInTenant :one
SELECT
    id, tenant_id, author_id, title, slug, status,
    tags, deadline, scheduled_at, published_at,
    excerpt, cover_image, attachments, publish_history,
    revision, revision_sha, content, telegram_url,
    created_at, updated_at
FROM posts
WHERE id = ? AND tenant_id = ?;

-- name: GetPostBySlug :one
SELECT
    id, tenant_id, author_id, title, slug, status,
    tags, deadline, scheduled_at, published_at,
    excerpt, cover_image, attachments, publish_history,
    revision, revision_sha, content, telegram_url,
    created_at, updated_at
FROM posts
WHERE tenant_id = ? AND slug = ?;

-- name: PostExistsByID :one
SELECT EXISTS(SELECT 1 FROM posts WHERE id = ?) AS found;

-- name: UpdatePost :execrows
UPDATE posts
SET
    author_id = ?,
    title = ?,
    slug = ?,
    status = ?,
    tags = ?,
    deadline = ?,
    scheduled_at = ?,
    published_at = ?,
    excerpt = ?,
    cover_image = ?,
    attachments = ?,
    publish_history = ?,
    revision = ?,
    revision_sha = ?,
    content = ?,
    telegram_url = ?,
    updated_at = ?
WHERE id = ? AND tenant_id = ? AND revision = ?;

-- name: DeletePost :execrows
DELETE FROM posts
WHERE id = ? AND tenant_id = ?;

-- name: CountPosts :one
SELECT COUNT(*) FROM posts;

-- name: UpsertPost :exec
INSERT INTO posts (
    id, tenant_id, author_id, title, slug, status,
    tags, deadline, scheduled_at, published_at,
    excerpt, cover_image, attachments, publish_history,
    revision, revision_sha, content, telegram_url,
    created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?
)
ON CONFLICT(id) DO UPDATE SET
    tenant_id = excluded.tenant_id,
    author_id = excluded.author_id,
    title = excluded.title,
    slug = excluded.slug,
    status = excluded.status,
    tags = excluded.tags,
    deadline = excluded.deadline,
    scheduled_at = excluded.scheduled_at,
    published_at = excluded.published_at,
    excerpt = excluded.excerpt,
    cover_image = excluded.cover_image,
    attachments = excluded.attachments,
    publish_history = excluded.publish_history,
    revision = excluded.revision,
    revision_sha = excluded.revision_sha,
    content = excluded.content,
    telegram_url = excluded.telegram_url,
    created_at = excluded.created_at,
    updated_at = excluded.updated_at;
