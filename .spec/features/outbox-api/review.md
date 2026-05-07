# Review — B.5d

**Verdict:** PASS.

- 3 endpoints (list/get/retry) реализованы под двумя префиксами (legacy + v1).
- 6 unit-тестов через mockOutboxRepo (extended из B.5b).
- OpenAPI spec обновлена + types/client регенерированы; build/test/lint GREEN.
- B.5 направление полностью закрыто: storage → core → CLI → API (queue + list + show + retry).

**Out of scope:**
- API для `outbox enqueue` уже есть как `POST /posts/{id}/queue` (B.5b).
- Per-tenant фильтрация — полагается на middleware (current scope: global, как у CLI).
