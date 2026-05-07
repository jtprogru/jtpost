# Code Review: openapi-client (F5b)

## Verdict: PASS

Все 16 требований реализованы и покрыты тестами. Test sweep + race + generate freshness — clean. Lint — pre-existing minor finding'и. Verdict = `PASS`.

## Change Set

5 created + 6 modified. Ключевые: `client.gen.go` (generated), `remote.go` + helper, `list_remote.go` для proof-of-concept, обновлённый `Taskfile`, CLI флаги в root.go.

## Requirements Traceability

| Group | REQ | Tests | Code |
|-------|-----|-------|------|
| 1 (Codegen) | 1.1..1.5 | `task generate` clean | `oapi-codegen-config-client.yaml`, `client.gen.go` |
| 2 (CLI flags) | 2.1..2.3 | TestNewAPIClient_* | `root.go`, `remote.go` |
| 3 (Helper) | 3.1..3.3 | TestNewAPIClient_* (6 cases) | `remote.go:newAPIClient` |
| 4 (List remote) | 4.1..4.4 | TestList_RemoteMode_* | `list.go`, `list_remote.go` |
| 5 (Tests) | 5.1..5.3 | сами тесты | — |

## Design Conformance

### 3.1 Architectural Boundaries ✅
- `apiclient` package — изолирован, generated, depends only on net/http+oapigen-runtime.
- `internal/cli/remote.go` — единая точка инициализации client из CLI-flags.

### 3.2 Data Models ✅
- Generated client использует `apiclient.Post`, `apiclient.LoginResponse`, etc. (duplicates oapigen types — known tradeoff `models: true` для simplicity).

### 3.3 API Contracts ✅
- Все 13 operations generated (LoginPosts, AuthLogin, OauthCallback, etc.).
- Auth-injection через `WithRequestEditorFn` — стандарт oapi-codegen.

### 3.4 Error Handling ✅
- Invalid URL → error в `newAPIClient` до request'а.
- Empty auth + empty env → error до request'а.
- 401 → возвращается через `resp.StatusCode()` check в runListRemote.
- 5xx/connection error → wrapped error.

### 3.5 Correctness Properties ✅
Все 16 CP покрыты тестами или явной проверкой в коде.

### 3.6 Documentation Consistency ✅
Mermaid в design отражает структуру.

## Code Quality

### 4.1 Naming & Clarity ✅
- `newAPIClient` returns triple `(*Client, isRemote bool, error)` — explicit semantics.
- `runListRemote` — speaking name.
- `contextBackground()` helper — defensive fallback.

### 4.2 Dead Code ✅
- Generated code изолирован.

### 4.3 Scope Creep ⚠ minor
- F5b делает только `jtpost list --remote`; остальные команды отложены в F5c. Documented.
- Spec-rename operationId `login` → `authLogin` — обязательное для codegen, не scope-creep.

### 4.4 Test Quality ✅
- Mock httptest.NewServer для proof-of-concept e2e-стиля.
- 6 unit-тестов newAPIClient покрывают все ветки.

## Security

✅ Security findings — none.

- Bearer-token инжектится через `WithRequestEditorFn` — никогда не логируется (LoggingMiddleware F4b/c уже маскирует).
- Auth-required validation до сетевого request'а (no leakage).
- Generated code не имеет hardcoded secrets.

## Verification Evidence

- **Tests** (`go test ./...`): 12 пакетов GREEN.
- **Race**: 12 пакетов GREEN.
- **Build**: `task build` OK.
- **Generate**: `task generate` clean (with regenerated `types.gen.go` after operationId rename).

## Findings

| ID | Severity | File | Description |
|----|----------|------|-------------|
| F-1 | minor | `oapi-codegen-config-client.yaml` | `models: true` → дубликат types в apiclient package. Альтернатива (import-mapping) сложна в v2.7.0; documented. |
| F-2 | minor | `internal/cli/list_remote.go` | Output JSON-only; table-mode для apiclient.Post — F5c. |
| F-3 | minor | `internal/cli/list_remote.go:contextBackground` | Helper для nil-ctx — defensive; обычно не нужен в production. |
| F-4..F-N | minor | прочие | Pre-existing F2..F5 minor lint findings. |

## Recommendations

**Minor follow-up (F5c):**
1. F-1: investigate proper import-mapping syntax v2 для type reuse.
2. F-2: implement table-mode для remote-выhода.
3. Реализовать `--remote` для остальных commands (new/edit/delete/show/stats/plan/tags).

## Pipeline state

4/4 задач T-1..T-4 marked complete.
