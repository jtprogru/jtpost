-- name: CreateOAuthAccount :exec
INSERT INTO oauth_accounts (
    id, user_id, provider, external_id, email, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6
);

-- name: GetOAuthAccountByExternalID :one
SELECT id, user_id, provider, external_id, email, created_at
FROM oauth_accounts
WHERE provider = $1 AND external_id = $2;

-- name: ListOAuthAccountsByUser :many
SELECT id, user_id, provider, external_id, email, created_at
FROM oauth_accounts
WHERE user_id = $1
ORDER BY created_at ASC;

-- name: DeleteOAuthAccount :execrows
DELETE FROM oauth_accounts WHERE id = $1;
