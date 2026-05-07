# Task Plan — B.5

## T-1 Domain
- `internal/core/outbox.go` — OutboxEntry, OutboxStatus, OutboxRepository iface, BackoffSchedule.

## T-2 Schema + sqlc queries
- `0005_outbox.sql` для sqlite + postgres.
- `queries/outbox.sql` (CreateOutbox, ClaimNextOutbox, MarkOutboxDone, MarkOutboxRetry, MarkOutboxFailed, ListOutbox, GetOutboxByID).
- `task generate` (sqlc).

## T-3 Adapters
- `internal/adapters/sqlite/outbox.go` (+Outbox() facade на PostRepository).
- `internal/adapters/postgres/outbox.go`.
- Compile-time `_ core.OutboxRepository = (*OutboxRepository)(nil)`.
- Bundle: добавить Outbox в storage.Bundle.

## T-4 Worker
- `internal/core/worker.go` — Worker, Run, processOne, computeBackoff.
- `internal/core/worker_test.go`.

## T-5 CLI
- `internal/cli/worker.go` — `worker run` cobra cmd.
- `internal/cli/outbox.go` — `outbox enqueue/list`.
- Wire в root.go.

## T-6 Tests
- contract: добавить outbox subtests в repotest или отдельный outbox_contract_test.go.
- sqlite outbox unit test.

## T-7 Wrap-up
- task test/build/lint.
- CHANGELOG.
