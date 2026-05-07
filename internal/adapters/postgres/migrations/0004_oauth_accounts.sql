-- +goose Up
CREATE TABLE oauth_accounts (
    id          uuid PRIMARY KEY,
    user_id     uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider    text NOT NULL,
    external_id text NOT NULL,
    email       text NOT NULL,
    created_at  timestamptz NOT NULL,
    UNIQUE (provider, external_id)
);
CREATE INDEX idx_oauth_user ON oauth_accounts(user_id);

-- +goose Down
DROP TABLE oauth_accounts;
