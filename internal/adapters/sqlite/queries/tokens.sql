-- name: CreateToken :exec
INSERT INTO tokens (
    id, user_id, prefix, secret_hash, name, created_at, expires_at, last_used_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetTokenByPrefix :one
SELECT id, user_id, prefix, secret_hash, name, created_at, expires_at, last_used_at
FROM tokens
WHERE prefix = ?;

-- name: DeleteToken :execrows
DELETE FROM tokens WHERE id = ?;

-- name: ListTokensByUser :many
SELECT id, user_id, prefix, secret_hash, name, created_at, expires_at, last_used_at
FROM tokens
WHERE user_id = ?
ORDER BY created_at ASC;

-- name: UpdateTokenLastUsedAt :execrows
UPDATE tokens SET last_used_at = ? WHERE id = ?;
