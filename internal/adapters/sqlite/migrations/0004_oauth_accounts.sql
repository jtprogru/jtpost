-- +goose Up
CREATE TABLE oauth_accounts (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL,
    provider    TEXT NOT NULL,
    external_id TEXT NOT NULL,
    email       TEXT NOT NULL,
    created_at  TEXT NOT NULL,
    UNIQUE (provider, external_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_oauth_user ON oauth_accounts(user_id);

-- +goose Down
DROP TABLE oauth_accounts;
