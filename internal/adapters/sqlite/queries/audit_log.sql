-- name: AppendAuditLog :exec
INSERT INTO audit_log (
    id, occurred_at, tenant_id, actor_id, actor_type, action,
    resource_type, resource_id, outcome, ip, user_agent, metadata
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListAuditLog :many
SELECT id, occurred_at, tenant_id, actor_id, actor_type, action,
       resource_type, resource_id, outcome, ip, user_agent, metadata
FROM audit_log
WHERE (sqlc.arg('tenant_id') = '' OR tenant_id = sqlc.arg('tenant_id'))
  AND (sqlc.arg('actor_id') = '' OR actor_id = sqlc.arg('actor_id'))
  AND (sqlc.arg('action') = '' OR action = sqlc.arg('action'))
ORDER BY occurred_at DESC
LIMIT sqlc.arg('lim');
