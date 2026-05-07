-- name: CreatePost :exec
INSERT INTO posts (
    id, tenant_id, author_id, title, slug, status,
    tags, deadline, scheduled_at, published_at,
    excerpt, cover_image, attachments, publish_history,
    revision, revision_sha, content, telegram_url,
    created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17, $18,
    $19, $20
);

-- name: GetPostByID :one
SELECT * FROM posts WHERE id = $1;

-- name: GetPostByIDInTenant :one
SELECT * FROM posts WHERE id = $1 AND tenant_id = $2;

-- name: GetPostBySlug :one
SELECT * FROM posts WHERE tenant_id = $1 AND slug = $2;

-- name: PostExistsByID :one
SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1) AS exists;

-- name: UpdatePost :execrows
UPDATE posts SET
    author_id = $2,
    title = $3,
    slug = $4,
    status = $5,
    tags = $6,
    deadline = $7,
    scheduled_at = $8,
    published_at = $9,
    excerpt = $10,
    cover_image = $11,
    attachments = $12,
    publish_history = $13,
    revision = $14,
    revision_sha = $15,
    content = $16,
    telegram_url = $17,
    updated_at = $18
WHERE id = $1 AND tenant_id = $19 AND revision = $20;

-- name: DeletePost :execrows
DELETE FROM posts WHERE id = $1 AND tenant_id = $2;

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
    $1, $2, $3, $4, $5, $6,
    $7, $8, $9, $10,
    $11, $12, $13, $14,
    $15, $16, $17, $18,
    $19, $20
)
ON CONFLICT (id) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    author_id = EXCLUDED.author_id,
    title = EXCLUDED.title,
    slug = EXCLUDED.slug,
    status = EXCLUDED.status,
    tags = EXCLUDED.tags,
    deadline = EXCLUDED.deadline,
    scheduled_at = EXCLUDED.scheduled_at,
    published_at = EXCLUDED.published_at,
    excerpt = EXCLUDED.excerpt,
    cover_image = EXCLUDED.cover_image,
    attachments = EXCLUDED.attachments,
    publish_history = EXCLUDED.publish_history,
    revision = EXCLUDED.revision,
    revision_sha = EXCLUDED.revision_sha,
    content = EXCLUDED.content,
    telegram_url = EXCLUDED.telegram_url,
    created_at = EXCLUDED.created_at,
    updated_at = EXCLUDED.updated_at;
