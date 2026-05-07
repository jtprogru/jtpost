# Requirements — B.5c

## REQ-1 SweepStuck
- 1.1 `OutboxRepository.SweepStuck(ctx, threshold)` возвращает кол-во reset-записей.
- 1.2 SQL: `UPDATE outbox_entries SET status='pending', updated_at=? WHERE status='in_flight' AND updated_at < ?`.
- 1.3 SQLite + Postgres адаптеры реализуют.

## REQ-2 Worker integration
- 2.1 `Worker.Run` вызывает SweepStuck на старте.
- 2.2 Каждые `cfg.SweepInterval` (default 5m) вызывает повторно.
- 2.3 `WorkerConfig.StuckThreshold` (default 10m).

## REQ-3 Tests
- 3.1 SQLite SweepStuck unit test (boundary: ровно threshold не sweep'ится).
- 3.2 Postgres integration test для всех outbox-методов (`//go:build integration`).
- 3.3 Worker test: SweepStuck вызывается при Run().
