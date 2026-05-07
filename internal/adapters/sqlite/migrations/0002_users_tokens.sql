-- +goose Up
CREATE TABLE users (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL,
    email         TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL,
    UNIQUE (tenant_id, email)
);
CREATE INDEX idx_users_tenant ON users(tenant_id);

CREATE TABLE tokens (
    id            TEXT PRIMARY KEY,
    user_id       TEXT NOT NULL,
    prefix        TEXT NOT NULL UNIQUE,
    secret_hash   TEXT NOT NULL,
    name          TEXT NOT NULL,
    created_at    TEXT NOT NULL,
    expires_at    TEXT,
    last_used_at  TEXT,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_tokens_user ON tokens(user_id);

-- +goose Down
DROP TABLE tokens;
DROP TABLE users;
