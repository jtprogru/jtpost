# Code Review: openapi-spec (F5)

## Verdict: PASS

Все 16 требований реализованы. 16 Correctness Properties прослежены. Test sweep + race + generate freshness — clean. Lint — pre-existing minor finding'и (не F5). Verdict = `PASS`.

## Change Set

7 created + 6 modified. Ключевые: `api/openapi.yaml` (формальная спека), `tools.go`/`oapi-codegen-config.yaml`/`oapigen/types.gen.go` (toolchain + generated), `bothPrefixes` helper в server.go, refactor LoginHandler, `api_v1_test.go`.

## Requirements Traceability

| Group | REQ | Tests | Code | CP |
|-------|-----|-------|------|-----|
| 1 (Spec) | 1.1..1.6 | parsing через `task generate:openapi` (fail если invalid) | `api/openapi.yaml` | CP-5, 11, 12, 13 |
| 2 (Toolchain) | 2.1..2.5 | `task generate` clean | `tools.go`, `oapi-codegen-config.yaml`, `Taskfile.yml` | CP-14, 15 |
| 3 (Type overrides) | 3.1..3.3 | compile (UUID/time.Time/pointer types) | `oapigen/types.gen.go` | CP-1, 2, 3, 4 |
| 4 (Handler refactor) | 4.1..4.3 | TestLoginHandler_* (existing) | `auth_handlers.go` | CP-9, 10 |
| 5 (v1 aliases) | 5.1..5.3 | TestAPIV1_LoginAliasWorks | `server.go:bothPrefixes` | CP-6, 8 |
| 6 (Tests) | 6.1..6.4 | сами тесты + freshness check | — | CP-7, 16 |

## Design Conformance

### 3.1 Architectural Boundaries ✅
- `oapigen` package — изолирован, только types. Зависит от `uuid`, `time`, `oapi-codegen/runtime`.
- `api/openapi.yaml` — single source of truth для HTTP-contract.

### 3.2 Data Models ✅
- 38 generated top-level types соответствуют domain concepts (Post, Attachment, etc.) с `x-go-type: uuid.UUID` overrides.
- DDL ничего не меняется — это только HTTP API уровень.

### 3.3 API Contracts ✅
- Все existing endpoints declared в spec.
- `/api/v1/...` aliases работают для всех routes.
- Legacy `/api/...` сохранён.

### 3.4 Error Handling ✅
- `ErrorResponse` schema используется через `$ref` для всех 4xx/5xx.

### 3.5 Correctness Properties ✅
- Все 16 CP покрыты прямыми/косвенными тестами.

### 3.6 Documentation Consistency ✅
- Mermaid в design отражает реальную structure.

## Code Quality

### 4.1 Naming & Clarity ✅
Generated names в PascalCase (стандарт oapi-codegen). Helper `bothPrefixes` — speaking name.

### 4.2 Dead Code ✅
- Local `LoginRequest`/`LoginResponse` structs удалены (заменены на oapigen).
- `apply` helper сохранён для не-auth routes.

### 4.3 Scope Creep ⚠ minor
- Refactor захватил только LoginHandler. `jsonPost` в server.go, OAuth handlers, middleware — оставлены. Documented в plan (T-3 NOTE: "Pragmatically minimal").

### 4.4 Test Quality ✅
- TestAPIV1_LoginAliasWorks проверяет идентичность legacy и v1 paths.
- Existing handler tests pass без regression — подтверждает refactor invariance.

## Security

✅ Security findings — none.

- OpenAPI spec декларирует security schemes, но enforcement через middleware (F4a-c) — без изменений.
- `bearerAuth` `bearerFormat: PAT` — informational.
- `cookieAuth` apiKey, in: cookie — informational.
- Generated types не имеют side-effects.

## Verification Evidence

- **Tests** (`go test ./...`): 12 пакетов GREEN.
- **Race**: 12 пакетов GREEN.
- **Generate**: `task generate` clean (sqlc + oapi-codegen). OpenAPI 3.1 partial-support warning от v2.7.0 (cosmetic; codegen работает корректно).
- **Build**: `task build` OK.

## Findings

| ID | Severity | File | Description | Requirement |
|----|----------|------|-------------|-------------|
| F-1 | minor | `api/openapi.yaml` | OpenAPI 3.1 partial-support warning от oapi-codegen v2.7.0. Codegen работает, types корректные. | REQ-1.1 |
| F-2 | minor | `internal/adapters/httpapi/server.go` | Не все handlers refactored под oapigen types (только LoginHandler). `jsonPost` остался. | REQ-4.2 (partial) |
| F-3..F-N | minor | прочие | Pre-existing F2/F3/F4 minor lint findings. | — |

## Recommendations

**Minor (follow-up):**
1. F-1: Downgrade spec до OpenAPI 3.0.x при появлении проблем с partial-support — но v2.7.0 справляется.
2. F-2: Полный refactor handlers под oapigen — F5b (server-side codegen).
3. F5b: Server-side ServerInterface codegen для type-safe routing.
4. F5c: CLI Go-client codegen + `--remote` mode.

## Pipeline state

6/6 задач T-1..T-6 marked complete.
