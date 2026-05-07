-- name: CreateToken :exec
INSERT INTO tokens (
    id, user_id, prefix, secret_hash, name, created_at, expires_at, last_used_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: GetTokenByPrefix :one
SELECT id, user_id, prefix, secret_hash, name, created_at, expires_at, last_used_at
FROM tokens
WHERE prefix = $1;

-- name: DeleteToken :execrows
DELETE FROM tokens WHERE id = $1;

-- name: ListTokensByUser :many
SELECT id, user_id, prefix, secret_hash, name, created_at, expires_at, last_used_at
FROM tokens
WHERE user_id = $1
ORDER BY created_at ASC;

-- name: UpdateTokenLastUsedAt :execrows
UPDATE tokens SET last_used_at = $1 WHERE id = $2;
