-- name: CreateSession :exec
INSERT INTO sessions (
    id, user_id, prefix, secret_hash, csrf_token, created_at, expires_at, last_used_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
);

-- name: GetSessionByPrefix :one
SELECT id, user_id, prefix, secret_hash, csrf_token, created_at, expires_at, last_used_at
FROM sessions
WHERE prefix = $1;

-- name: DeleteSession :execrows
DELETE FROM sessions WHERE id = $1;

-- name: DeleteSessionsByUser :execrows
DELETE FROM sessions WHERE user_id = $1;

-- name: UpdateSessionLastUsedAt :execrows
UPDATE sessions SET last_used_at = $1 WHERE id = $2;

-- name: UpdateSessionCSRFToken :execrows
UPDATE sessions SET csrf_token = $1 WHERE id = $2;

-- name: ListSessionsByUser :many
SELECT id, user_id, prefix, secret_hash, csrf_token, created_at, expires_at, last_used_at
FROM sessions
WHERE user_id = $1
ORDER BY created_at ASC;
