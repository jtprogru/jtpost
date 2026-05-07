-- name: CreateOutbox :exec
INSERT INTO outbox_entries (
    id, post_id, tenant_id, kind, status, attempts, max_attempts,
    next_attempt_at, last_error, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetOutboxByID :one
SELECT id, post_id, tenant_id, kind, status, attempts, max_attempts,
       next_attempt_at, last_error, created_at, updated_at
FROM outbox_entries WHERE id = ?;

-- name: ClaimNextOutbox :one
UPDATE outbox_entries
SET status = 'in_flight', updated_at = ?
WHERE id = (
    SELECT id FROM outbox_entries oe
    WHERE oe.status = 'pending' AND oe.next_attempt_at <= ?
    ORDER BY oe.next_attempt_at ASC
    LIMIT 1
)
RETURNING id, post_id, tenant_id, kind, status, attempts, max_attempts,
          next_attempt_at, last_error, created_at, updated_at;

-- name: MarkOutboxDone :execrows
UPDATE outbox_entries
SET status = 'done', updated_at = ?
WHERE id = ?;

-- name: MarkOutboxRetry :execrows
UPDATE outbox_entries
SET status = 'pending', attempts = ?, next_attempt_at = ?, last_error = ?, updated_at = ?
WHERE id = ?;

-- name: MarkOutboxFailed :execrows
UPDATE outbox_entries
SET status = 'failed', last_error = ?, updated_at = ?
WHERE id = ?;

-- name: ListOutboxByStatus :many
SELECT id, post_id, tenant_id, kind, status, attempts, max_attempts,
       next_attempt_at, last_error, created_at, updated_at
FROM outbox_entries
WHERE status = ?
ORDER BY created_at DESC
LIMIT ?;

-- name: SweepStuckOutbox :execrows
UPDATE outbox_entries
SET status = 'pending', updated_at = ?
WHERE status = 'in_flight' AND updated_at < ?;

-- name: ListOutboxAll :many
SELECT id, post_id, tenant_id, kind, status, attempts, max_attempts,
       next_attempt_at, last_error, created_at, updated_at
FROM outbox_entries
ORDER BY created_at DESC
LIMIT ?;
