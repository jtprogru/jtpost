-- name: CreateOutbox :exec
INSERT INTO outbox_entries (
    id, post_id, tenant_id, kind, status, attempts, max_attempts,
    next_attempt_at, last_error, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: GetOutboxByID :one
SELECT id, post_id, tenant_id, kind, status, attempts, max_attempts,
       next_attempt_at, last_error, created_at, updated_at
FROM outbox_entries WHERE id = $1;

-- name: ClaimNextOutbox :one
UPDATE outbox_entries
SET status = 'in_flight', updated_at = $1
WHERE id = (
    SELECT oe.id FROM outbox_entries oe
    WHERE oe.status = 'pending' AND oe.next_attempt_at <= $2
    ORDER BY oe.next_attempt_at ASC
    FOR UPDATE SKIP LOCKED
    LIMIT 1
)
RETURNING id, post_id, tenant_id, kind, status, attempts, max_attempts,
          next_attempt_at, last_error, created_at, updated_at;

-- name: MarkOutboxDone :execrows
UPDATE outbox_entries
SET status = 'done', updated_at = $1
WHERE id = $2;

-- name: MarkOutboxRetry :execrows
UPDATE outbox_entries
SET status = 'pending', attempts = $1, next_attempt_at = $2, last_error = $3, updated_at = $4
WHERE id = $5;

-- name: MarkOutboxFailed :execrows
UPDATE outbox_entries
SET status = 'failed', last_error = $1, updated_at = $2
WHERE id = $3;

-- name: ListOutboxByStatus :many
SELECT id, post_id, tenant_id, kind, status, attempts, max_attempts,
       next_attempt_at, last_error, created_at, updated_at
FROM outbox_entries
WHERE status = $1
ORDER BY created_at DESC
LIMIT $2;

-- name: SweepStuckOutbox :execrows
UPDATE outbox_entries
SET status = 'pending', updated_at = $1
WHERE status = 'in_flight' AND updated_at < $2;

-- name: ListOutboxAll :many
SELECT id, post_id, tenant_id, kind, status, attempts, max_attempts,
       next_attempt_at, last_error, created_at, updated_at
FROM outbox_entries
ORDER BY created_at DESC
LIMIT $1;
