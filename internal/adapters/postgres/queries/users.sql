-- name: CreateUser :exec
INSERT INTO users (
    id, tenant_id, email, password_hash, role, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
);

-- name: GetUserByID :one
SELECT id, tenant_id, email, password_hash, role, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, tenant_id, email, password_hash, role, created_at, updated_at
FROM users
WHERE tenant_id = $1 AND email = $2;

-- name: UpdateUser :execrows
UPDATE users
SET
    email = $2,
    password_hash = $3,
    role = $4,
    updated_at = $5
WHERE id = $1;

-- name: DeleteUser :execrows
DELETE FROM users WHERE id = $1;

-- name: ListUsersByTenant :many
SELECT id, tenant_id, email, password_hash, role, created_at, updated_at
FROM users
WHERE tenant_id = $1
ORDER BY created_at ASC;

-- name: CountUsersByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1;

-- name: CountOwnersByTenant :one
SELECT COUNT(*) FROM users WHERE tenant_id = $1 AND role = 'owner';
