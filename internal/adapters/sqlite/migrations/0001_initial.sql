-- +goose Up
DROP TABLE IF EXISTS posts;
CREATE TABLE posts (
    id              TEXT PRIMARY KEY,
    tenant_id       TEXT NOT NULL,
    author_id       TEXT NOT NULL,
    title           TEXT NOT NULL,
    slug            TEXT NOT NULL,
    status          TEXT NOT NULL,
    tags            TEXT NOT NULL DEFAULT '[]',
    deadline        TEXT,
    scheduled_at    TEXT,
    published_at    TEXT,
    excerpt         TEXT,
    cover_image     TEXT,
    attachments     TEXT NOT NULL DEFAULT '[]',
    publish_history TEXT NOT NULL DEFAULT '[]',
    revision        INTEGER NOT NULL DEFAULT 1,
    revision_sha    TEXT,
    content         TEXT NOT NULL,
    telegram_url    TEXT,
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,
    UNIQUE (tenant_id, slug)
);
CREATE INDEX idx_posts_tenant ON posts(tenant_id);
CREATE INDEX idx_posts_tenant_status ON posts(tenant_id, status);
CREATE INDEX idx_posts_tenant_author ON posts(tenant_id, author_id);
CREATE INDEX idx_posts_tenant_created_at ON posts(tenant_id, created_at);

-- +goose Down
DROP TABLE posts;
