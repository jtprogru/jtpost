-- +goose Up
CREATE TABLE users (
    id            uuid PRIMARY KEY,
    tenant_id     uuid NOT NULL,
    email         text NOT NULL,
    password_hash text NOT NULL,
    role          text NOT NULL,
    created_at    timestamptz NOT NULL,
    updated_at    timestamptz NOT NULL,
    UNIQUE (tenant_id, email)
);
CREATE INDEX idx_users_tenant ON users(tenant_id);

CREATE TABLE tokens (
    id            uuid PRIMARY KEY,
    user_id       uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    prefix        text NOT NULL UNIQUE,
    secret_hash   text NOT NULL,
    name          text NOT NULL,
    created_at    timestamptz NOT NULL,
    expires_at    timestamptz,
    last_used_at  timestamptz
);
CREATE INDEX idx_tokens_user ON tokens(user_id);

-- +goose Down
DROP TABLE tokens;
DROP TABLE users;
