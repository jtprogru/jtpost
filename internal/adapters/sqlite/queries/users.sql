-- name: CreateUser :exec
INSERT INTO users (
    id, tenant_id, email, password_hash, role, created_at, updated_at
) VALUES (
    ?, ?, ?, ?, ?, ?, ?
);

-- name: GetUserByID :one
SELECT id, tenant_id, email, password_hash, role, created_at, updated_at
FROM users
WHERE id = ?;

-- name: GetUserByEmail :one
SELECT id, tenant_id, email, password_hash, role, created_at, updated_at
FROM users
WHERE tenant_id = ? AND email = ?;

-- name: UpdateUser :execrows
UPDATE users
SET
    email = ?,
    password_hash = ?,
    role = ?,
    updated_at = ?
WHERE id = ?;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = ?;

-- name: ListUsersByTenant :many
SELECT id, tenant_id, email, password_hash, role, created_at, updated_at
FROM users
WHERE tenant_id = ?
ORDER BY created_at ASC;

-- name: CountUsersByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = ?;

-- name: CountOwnersByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = ? AND role = 'owner';
