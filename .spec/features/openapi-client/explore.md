# Exploration: OpenAPI client codegen + `--remote` mode (F5b)

## Intent

Продолжение B.3 после F5 (types). F5 декларировал spec и сгенерировал types; F5b генерирует **HTTP-client** из той же спецификации и подключает его к CLI как опциональный режим работы (`jtpost <cmd> --remote URL --auth TOKEN`). Локальный режим (storage backend) остаётся default.

**Scope F5b:**

1. Расширить `oapi-codegen-config.yaml` чтобы генерировал client помимо types: `generate: { models: true, client: true }` ИЛИ создать отдельный config (предпочтительно — отдельный, чтобы types не дублировались).
2. Создать `oapi-codegen-config-client.yaml` → `internal/adapters/apiclient/client.gen.go` (package `apiclient`).
3. Добавить task `generate:apiclient` в Taskfile; aggregate-task `generate` запускает оба openapi-job.
4. Добавить глобальные CLI-флаги `--remote URL` и `--auth TOKEN`.
5. Реализовать `--remote` для **одной команды** `jtpost list` как proof-of-concept (вместо локального repo использует apiclient.GetPosts).
6. Документировать использование в CHANGELOG/README.

**Чего F5b НЕ делает:**
- НЕ внедряет `--remote` для всех команд (`jtpost new/edit/delete` — отложено в F5c).
- НЕ делает server-side ServerInterface codegen (отложено в F5c).
- НЕ убирает legacy `/api/...` (отложено в F5c).
- НЕ добавляет caching, retry-logic, refresh-token rotation в client.
- НЕ интегрирует CLI sessions (cookie-based auth для CLI — out of scope; используется только Bearer-token через `--auth`).

**Триггер:** F5 закрыл types. CLI `--remote` mode — большой user-facing value B.3.

---

## Investigation

### Existing CLI

`internal/cli/list.go` — текущая команда:
```go
repo, closer, err := openRepo(cfg)
service := core.NewPostService(repo, ...)
posts, err := service.ListPosts(ctx, filter)
```

Используется `core.PostService` через storage Bundle. Output — `printTable` или `printPostsJSON`.

### oapi-codegen client mode

`oapi-codegen` v2 генерирует:
- `Client` struct с методами (`GetPosts(ctx, params)`, `CreatePost(ctx, body)`, etc.).
- `ClientWithResponses` — convenience с typed responses (200/400/...).
- Body decoding автоматически в structs.

Конфигурация:
```yaml
generate:
  client: true
output-options:
  client-type-name: Client  # default
```

### Что F5b затронет

- `oapi-codegen-config-client.yaml` (NEW)
- `Taskfile.yml` — `generate:apiclient` + alias
- `internal/adapters/apiclient/client.gen.go` (NEW, generated)
- `internal/cli/root.go` — `--remote`, `--auth` global flags
- `internal/cli/list.go` — branching: local-mode vs remote-mode
- `internal/cli/remote.go` (NEW) — helper builds apiclient.Client from cfg+flags
- `CHANGELOG.md`

### Что F5b НЕ затронет

- `core.PostService`, `core.PostRepository` — domain без изменений.
- Storage adapters — without changes.
- HTTP server — без изменений.
- Auth chain — без изменений.

---

## Build Tooling

- `task generate:openapi` (types) → переименуем в `task generate:openapi:types`.
- `task generate:openapi:client` (NEW).
- `task generate` aggregate.
- Test/Build/Lint без изменений.

---

## Options Considered

### Option A: Separate config files (recommended)

`oapi-codegen-config.yaml` (types) + `oapi-codegen-config-client.yaml` (client). Два output files.

- **Pros:** клиентский код не смешан с types; isolated package.
- **Cons:** два конфига.
- **Сложность:** Low.

### Option B: Single config с models+client

`generate: {models: true, client: true}` → один output file.

- **Pros:** один конфиг.
- **Cons:** package mixes types и client; меняется existing oapigen package.
- **Сложность:** Low.

### Option C: Custom client (без oapi-codegen client)

Написать handcraft Go client.

- **Pros:** zero codegen.
- **Cons:** drift от spec; manual work для 13 endpoints.
- **Сложность:** Medium.

---

## Recommended Direction

**Option A** — separate config + package `apiclient`.

Steps:
1. `oapi-codegen-config-client.yaml` → `internal/adapters/apiclient/client.gen.go` (package `apiclient`, imports types from `oapigen`).
2. `task generate:openapi:client`.
3. CLI `--remote URL`, `--auth TOKEN` global flags.
4. `internal/cli/remote.go` helper `newAPIClient(cfg, log)` возвращает `(*apiclient.ClientWithResponses, error)` если `--remote` specified, иначе nil.
5. `internal/cli/list.go` branching: если `--remote` → используем client; иначе текущий path.
6. Tests: `TestList_Remote` (через httptest mock-server) + manual smoke.

---

## Scope Boundaries

### Must-have (F5b)

- Client codegen в `apiclient`.
- Taskfile updates.
- Global `--remote`/`--auth` flags.
- `jtpost list --remote URL --auth jtpat_...` работает.
- Existing local-mode `jtpost list` без `--remote` — НЕ изменён.
- Tests: TestList_Remote (mock server).

### Deferred (F5c)

- `--remote` для `jtpost new/edit/delete/show/stats/plan/tags/next`.
- Server-side ServerInterface codegen.
- CLI `jtpost auth login --remote` interactive flow.
- Token caching, retry, retry-after handling.
- ReDoc/Swagger UI на `/api/docs`.
- Legacy `/api/...` deprecation.

---

## Assumptions & Open Questions

### Assumptions

- `[ASSUMPTION: types из oapigen package переиспользуются клиентом через import override]`
- `[ASSUMPTION: client-type — ClientWithResponses (typed responses); raw Client — secondary]`
- `[ASSUMPTION: --remote URL включает schema (http://localhost:8080) — без default normalization]`
- `[ASSUMPTION: --auth — только Bearer-token (PAT); cookie-session для CLI отложено]`
- `[ASSUMPTION: list --remote реализуется один (proof-of-concept); rest через F5c]`

### Open Questions

1. URL flag валидация (e.g. trailing slash trim)? — Trim на boundary helper.
2. Default --remote из cfg.Server.BaseURL? — Да (если задан).
3. Auth source: ENV `JTPOST_AUTH_TOKEN` fallback к --auth? — Да.

---

## Done When

- [x] Codebase читан.
- [x] 3 опции.
- [x] Trade-offs.
- [x] Scope.
- [x] Assumptions.
- [x] Open Questions (3).
- [x] Build tooling.
- [ ] Артефакт зарегистрирован.
