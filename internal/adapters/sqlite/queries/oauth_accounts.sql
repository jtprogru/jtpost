-- name: CreateOAuthAccount :exec
INSERT INTO oauth_accounts (
    id, user_id, provider, external_id, email, created_at
) VALUES (
    ?, ?, ?, ?, ?, ?
);

-- name: GetOAuthAccountByExternalID :one
SELECT id, user_id, provider, external_id, email, created_at
FROM oauth_accounts
WHERE provider = ? AND external_id = ?;

-- name: ListOAuthAccountsByUser :many
SELECT id, user_id, provider, external_id, email, created_at
FROM oauth_accounts
WHERE user_id = ?
ORDER BY created_at ASC;

-- name: DeleteOAuthAccount :execrows
DELETE FROM oauth_accounts WHERE id = ?;
