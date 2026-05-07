-- name: AppendAuditLog :exec
INSERT INTO audit_log (
    id, occurred_at, tenant_id, actor_id, actor_type, action,
    resource_type, resource_id, outcome, ip, user_agent, metadata
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12);

-- name: ListAuditLog :many
SELECT id, occurred_at, tenant_id, actor_id, actor_type, action,
       resource_type, resource_id, outcome, ip, user_agent, metadata
FROM audit_log
WHERE (sqlc.narg('tenant_id')::UUID IS NULL OR tenant_id = sqlc.narg('tenant_id')::UUID)
  AND (sqlc.narg('actor_id')::UUID IS NULL OR actor_id = sqlc.narg('actor_id')::UUID)
  AND (sqlc.narg('action')::TEXT IS NULL OR action = sqlc.narg('action')::TEXT)
ORDER BY occurred_at DESC
LIMIT sqlc.arg('lim');
