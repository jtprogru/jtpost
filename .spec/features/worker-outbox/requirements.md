# Requirements — B.5

## REQ-1 Schema
- 1.1 Таблица `outbox_entries` (id PK UUID, post_id, tenant_id, kind TEXT,
  status TEXT [pending|in_flight|done|failed], attempts INT, max_attempts INT,
  next_attempt_at TIMESTAMP, last_error TEXT, created_at, updated_at).
- 1.2 Индексы: (status, next_attempt_at) для poll'а; (post_id) для lookup.
- 1.3 Миграции в SQLite (0005) и Postgres (0005).

## REQ-2 Repository (core.OutboxRepository)
- 2.1 `Enqueue(ctx, entry) error` — INSERT new entry.
- 2.2 `ClaimNext(ctx) (*OutboxEntry, error)` — атомарно claim первую готовую
  pending entry, возвращает nil,nil если ничего нет.
- 2.3 `MarkDone(ctx, id) error`.
- 2.4 `MarkRetry(ctx, id, attempts, nextAt, errMsg) error`.
- 2.5 `MarkFailed(ctx, id, errMsg) error`.
- 2.6 `List(ctx, filter) ([]OutboxEntry, error)` — для CLI list.
- 2.7 `GetByID(ctx, id) (*OutboxEntry, error)`.

## REQ-3 Worker
- 3.1 `core.Worker` с конфигом {PollInterval, MaxAttempts, BackoffSchedule}.
- 3.2 `Worker.Run(ctx)` — loop до отмены ctx.
- 3.3 `Worker.processOne(ctx)` — claim+process; вызывается тестом напрямую.
- 3.4 Backoff: defaults [1m,5m,25m,2h,8h] (5 попыток).

## REQ-4 CLI
- 4.1 `jtpost worker run [--interval 10s] [--max-attempts N]` — запуск worker'а.
- 4.2 `jtpost outbox enqueue <post-id>` — поставить пост в очередь на публикацию.
- 4.3 `jtpost outbox list [--status pending|in_flight|done|failed]` — таблица entries.

## REQ-5 Tests
- 5.1 Contract test для OutboxRepository (sqlite).
- 5.2 Worker.processOne: success path + retry path + permanent fail.
- 5.3 Backoff calculation tests.
