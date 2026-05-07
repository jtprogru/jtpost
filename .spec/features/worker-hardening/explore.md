# B.5c — Worker hardening

## Problem
- Stuck `in_flight`: если worker крашится между ClaimNext и MarkDone/Retry,
  запись остаётся `in_flight` навсегда — worker её не подхватит снова.
- Postgres outbox адаптер не покрыт integration-тестами (sqlite — да).

## Solution
1. **`SweepStuck(ctx, threshold) (count int, err error)`** — сбрасывает в
   `pending` все `in_flight` записи, у которых `updated_at < now()-threshold`.
   Worker вызывает на старте + каждые 5 минут.
2. **Postgres integration test** через testcontainers (под build tag
   `integration`, как существующие pg-тесты).

## Decisions
- Threshold default: 10 минут. Конфиг через WorkerConfig.StuckThreshold.
- Sweep period: 5 минут (не каждый poll — слишком часто).
- При sweep сбрасываем только status (attempts оставляем — попытка засчитана).
