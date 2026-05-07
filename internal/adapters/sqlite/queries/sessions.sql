-- name: CreateSession :exec
INSERT INTO sessions (
    id, user_id, prefix, secret_hash, csrf_token, created_at, expires_at, last_used_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?, ?
);

-- name: GetSessionByPrefix :one
SELECT id, user_id, prefix, secret_hash, csrf_token, created_at, expires_at, last_used_at
FROM sessions
WHERE prefix = ?;

-- name: DeleteSession :execrows
DELETE FROM sessions WHERE id = ?;

-- name: DeleteSessionsByUser :execrows
DELETE FROM sessions WHERE user_id = ?;

-- name: UpdateSessionLastUsedAt :execrows
UPDATE sessions SET last_used_at = ? WHERE id = ?;

-- name: UpdateSessionCSRFToken :execrows
UPDATE sessions SET csrf_token = ? WHERE id = ?;

-- name: ListSessionsByUser :many
SELECT id, user_id, prefix, secret_hash, csrf_token, created_at, expires_at, last_used_at
FROM sessions
WHERE user_id = ?
ORDER BY created_at ASC;
