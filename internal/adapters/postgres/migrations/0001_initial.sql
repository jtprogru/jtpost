-- +goose Up
CREATE TABLE posts (
    id              uuid PRIMARY KEY,
    tenant_id       uuid NOT NULL,
    author_id       uuid NOT NULL,
    title           text NOT NULL,
    slug            text NOT NULL,
    status          text NOT NULL,
    tags            jsonb NOT NULL DEFAULT '[]'::jsonb,
    deadline        timestamptz,
    scheduled_at    timestamptz,
    published_at    timestamptz,
    excerpt         text,
    cover_image     jsonb,
    attachments     jsonb NOT NULL DEFAULT '[]'::jsonb,
    publish_history jsonb NOT NULL DEFAULT '[]'::jsonb,
    revision        integer NOT NULL DEFAULT 1,
    revision_sha    text,
    content         text NOT NULL,
    telegram_url    text,
    created_at      timestamptz NOT NULL,
    updated_at      timestamptz NOT NULL,
    UNIQUE (tenant_id, slug)
);
CREATE INDEX idx_posts_tenant ON posts(tenant_id);
CREATE INDEX idx_posts_tenant_status ON posts(tenant_id, status);
CREATE INDEX idx_posts_tenant_author ON posts(tenant_id, author_id);
CREATE INDEX idx_posts_tenant_created_at ON posts(tenant_id, created_at);
-- +goose Down
DROP TABLE posts;
