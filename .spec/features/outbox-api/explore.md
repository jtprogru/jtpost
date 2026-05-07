# B.5d — HTTP API for outbox management

## Scope
- `GET /api/outbox?status=pending&limit=50` — list outbox entries (тенант-фильтр пока global, like CLI).
- `GET /api/outbox/{id}` — show один entry.
- `POST /api/outbox/{id}/retry` — сбросить `failed`/`in_flight` обратно в `pending` (next_attempt_at=now), reset attempts.

## Existing
- `core.OutboxRepository` интерфейс (Enqueue, ClaimNext, MarkDone, MarkRetry, MarkFailed, List, GetByID, SweepStuck).
- HTTP server уже знает `outbox` field (Server.outbox).
- CLI `outbox enqueue/list` уже есть.

## Decisions
- `Retry` использует `MarkRetry` с attempts=0 (полный сброс), next_attempt_at=now.
- Все endpoints требуют auth (bearer/cookie).
- 503 если outbox не сконфигурирован (как в `/queue`).
- OpenAPI spec обновляется + types/client регенерируются.
