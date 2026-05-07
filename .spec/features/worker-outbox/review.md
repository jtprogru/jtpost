# Review — B.5 Worker

**Verdict:** PASS.

- REQ-1..REQ-5 implemented.
- 10 new tests (5 worker unit + 5 sqlite integration); все passes под -race.
- Бanдл, factory, миграции в обоих диалектах согласованы.
- Атомарный claim через `UPDATE WHERE id = (SELECT ... LIMIT 1) RETURNING *` — работает и на SQLite (3.35+), и на Postgres (с SKIP LOCKED).
- Sync publish flow не тронут — outbox additive feature.
- task test/build/race GREEN.

**Откладки на будущее:**
- API endpoint `POST /posts/{id}/queue` (по аналогии с CLI `outbox enqueue`).
- Recovery stuck-in-flight: sweep-to-pending для записей с `status='in_flight' AND updated_at < now()-N`.
- Postgres contract test для outbox (надо запускать testcontainers).
