# `--remote` для read-only CLI команд (F5c) — Requirements

## Overview

Расширяет F5b proof-of-concept: добавляет `--remote URL --auth TOKEN` mode для `show`, `stats`, `plan`, `next` команд. Локальный режим без `--remote` — без изменений.

## Requirements

**REQ-1.1** WHEN `jtpost show <id> --remote URL --auth TOKEN` запускается, the system SHALL вызвать `cli.GetPostWithResponse(ctx, id)`, на 200 — JSON-вывод, на 401 — exit с error "unauthorized", на 404 — exit с error "post not found", на 5xx — exit с error.

**REQ-1.2** WHEN `jtpost stats --remote ...` запускается, the system SHALL вызвать `cli.GetStatsWithResponse(ctx)` и вывести stats как JSON.

**REQ-1.3** WHEN `jtpost plan --remote ...` запускается, the system SHALL вызвать `cli.GetPlanWithResponse(ctx, params)` и вывести plan как JSON.

**REQ-1.4** WHEN `jtpost next --remote ...` запускается, the system SHALL вызвать `cli.GetNextPostWithResponse(ctx)` и вывести post как JSON.

**REQ-1.5** WHEN любая из read-only команд запускается БЕЗ `--remote`, the system SHALL продолжить использовать local Bundle/repo (текущее поведение).

**REQ-2.1** WHEN модуль `cli` экспортирует helper `runRemote(cmd *cobra.Command, fn func(*apiclient.ClientWithResponses) error) error`, the system SHALL централизованно обрабатывать `newAPIClient`+ context fallback + ошибки 401/5xx.

**REQ-2.2** WHEN F5b `runListRemote` рефакторится на `runRemote`, the system SHALL сохранять existing behavior (TestList_RemoteMode_* проходят).

**REQ-3.1** WHEN запускается `task test`, the system SHALL пройти tests для каждой команды через httptest mock-server: success + 401 + 404 (где применимо).

## Verification Commands

| Action | Command |
|--------|---------|
| Test | `task test` |
| Build | `task build` |
