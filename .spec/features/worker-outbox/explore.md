# B.5 — Worker (publisher outbox)

## Problem
Текущий `publish` синхронен: CLI/API ждут результата вызова Telegram API.
Нет retry на transient errors, нет фонового planирования (scheduled_at не
триггерит публикацию автоматически).

## Solution: Outbox pattern
1. **Enqueue:** добавляем запись в таблицу `outbox_entries` (status=pending,
   next_attempt_at=now или scheduled_at).
2. **Worker:** долгоживущий процесс, в loop'е (poll interval=10s):
   - claims следующую entry (UPDATE status='in_flight' WHERE id IN (
     SELECT id FROM outbox_entries WHERE status='pending'
     AND next_attempt_at <= now() LIMIT 1) RETURNING *).
   - Загружает Post, вызывает publisher.Publish.
   - On success → status='done', post.Status='published', PublishedAt=now.
   - On error → attempts++; если attempts>=max_attempts → status='failed',
     post.Status='failed'; иначе status='pending', next_attempt_at=now+backoff.
3. **Backoff:** exponential 1m,5m,25m,2h,8h (max_attempts=5).

## Existing
- `core.Publisher` interface (`internal/core/publisher.go`).
- `telegram.Publisher` реализация.
- `core.PostService.PublishPost(ctx, id, in, publisher)` — синхронный путь.
- sqlc + goose в обоих SQL адаптерах.

## Decisions (MVP)
- Публикация остаётся синхронной по умолчанию (`publish`); enqueue — отдельная
  команда `outbox enqueue <id>` и API `POST /posts/{id}/queue` (опционально, не в MVP).
- Outbox `kind` пока только `publish`. Schema готова к расширению.
- Worker single-process; concurrency через UPDATE...RETURNING per-row claim
  (атомарно даже на SQLite).
- `claimed_by`/`claimed_at` для recovery — out of MVP scope, делаем "stuck
  in_flight" recovery через простой "WHERE status='in_flight' AND updated_at <
  now()-30m" → sweep-to-pending. Out of MVP — добавим если будет нужно.

## Out of scope
- Multiple workers (нужны row-locks, для SQLite сложно).
- API endpoints для outbox (CLI достаточно).
- Webhook для status updates.
