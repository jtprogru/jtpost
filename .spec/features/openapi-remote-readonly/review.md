# Code Review: openapi-remote-readonly (F5c)

## Verdict: PASS

Все 8 требований реализованы. 12 пакетов test sweep GREEN. Build clean. Lint — pre-existing minor finding'и. Verdict = `PASS`.

## Change Set

5 created + 6 modified. Pattern repetitive (по design): для каждой read-only команды создан `*_remote.go` (~30 строк) + branching в RunE.

## Requirements Traceability

| REQ | Tests | Code |
|-----|-------|------|
| 1.1 (show) | TestShow_RemoteMode_* (3 tests) | `show.go`, `show_remote.go` |
| 1.2 (stats) | TestStats_RemoteMode_Success | `stats.go`, `stats_remote.go` |
| 1.3 (plan) | TestPlan_RemoteMode_Success | `plan.go`, `plan_remote.go` |
| 1.4 (next) | TestNext_RemoteMode_Success/_404 | `next.go`, `next_remote.go` |
| 1.5 (local unchanged) | existing tests pass | branching pattern в каждой cmd |
| 2.1 (helper) | косвенно через TestList_RemoteMode (refactored) | `remote.go:runRemote` |
| 2.2 (refactor) | TestList_RemoteMode_* — pass без изменений semantics | `list.go`, `list_remote.go` |
| 3.1 (tests) | 8 новых mock-server tests + existing | — |

## Design Conformance

### 3.1 Architectural Boundaries ✅
- DRY-helper `runRemote` — единая точка enter remote-mode.
- Каждая команда: branching в RunE → delegate в `run*Remote(ctx, cli, out)`.

### 3.2 Data Models ✅
Без новых types — переиспользуются `apiclient.Post`, `apiclient.PlanItem`, etc.

### 3.3 API Contracts ✅
Все 4 endpoints из spec вызываются через generated client (`GetPostWithResponse`, `GetStatsWithResponse`, `GetPlanWithResponse`, `GetNextPostWithResponse`).

### 3.4 Error Handling ✅
Standard pattern в каждом *_remote.go: 200 → JSON output; 401 → "unauthorized"; 404 (где применимо) → "not found"; default → "remote API returned <code>"; connection error → wrapped.

## Code Quality

### 4.1 Naming & Clarity ✅
`run<Cmd>Remote` consistent naming. `runRemote(cmd, fn)` self-explanatory.

### 4.2 Dead Code ✅
- F5b `contextBackground()` helper — упразднён (inline `context.Background()`).

### 4.3 Scope Creep ⚠ minor
- F5c proof-of-concept ограничен read-only. Write-команды (new/edit/delete) — F5d. Documented.
- `show` slug-lookup — нет endpoint в spec → требует UUID для remote-mode. Documented.

### 4.4 Test Quality ✅
8 новых tests + рефактор existing — все pass. Mock-server pattern шаблонный.

## Security

✅ Нет findings. Auth-injection через RequestEditorFn (F5b infrastructure). No secrets logged.

## Verification Evidence

- **Tests** (`go test ./...`): 12 пакетов GREEN.
- **Build**: `task build` OK.

## Findings

| ID | Severity | File | Description |
|----|----------|------|-------------|
| F-1 | minor | `show_remote.go` | UUID-only для remote (slug needs new endpoint). |
| F-2 | minor | `*_remote.go` files | JSON-only output. Table-mode — F5d. |
| F-3..N | minor | прочие | Pre-existing F2..F5b minor lint. |

## Pipeline state

4/4 задач T-1..T-4 marked complete.
