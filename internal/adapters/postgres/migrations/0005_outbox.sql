-- +goose Up
CREATE TABLE outbox_entries (
    id              UUID PRIMARY KEY,
    post_id         UUID NOT NULL,
    tenant_id       UUID NOT NULL,
    kind            TEXT NOT NULL,
    status          TEXT NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0,
    max_attempts    INTEGER NOT NULL DEFAULT 5,
    next_attempt_at TIMESTAMPTZ NOT NULL,
    last_error      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL,
    updated_at      TIMESTAMPTZ NOT NULL
);
CREATE INDEX idx_outbox_status_next ON outbox_entries(status, next_attempt_at);
CREATE INDEX idx_outbox_post ON outbox_entries(post_id);

-- +goose Down
DROP TABLE outbox_entries;
