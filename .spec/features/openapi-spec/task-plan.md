# OpenAPI 3.1 Spec + types codegen (F5) — Task Plan

**Test Style Source:** Tier 2.
**Commands:** `task test`, `task test:race`, `task build`, `task lint`, `task generate`.

## Coverage Matrix

| REQ | Tasks | CP |
|-----|-------|-----|
| 1.x (Spec) | T-1 | CP-5, 11, 12, 13 |
| 2.x (Toolchain) | T-2 | CP-14, 15 |
| 3.x (Type overrides) | T-2 | CP-1, 2, 3, 4 |
| 4.x (Handler refactor) | T-3 | CP-9, 10 |
| 5.x (v1 aliases) | T-4 | CP-6, 8 |
| 6.x (Tests) | T-5 | CP-7, 16 |

## Work Type: Pure feature (additive — без breaking).

---

## T-1 — OpenAPI specification

***_Complexity: complex_*** ***_Requirements: REQ-1.x_*** ***_Preservation: CP-5, 11, 12, 13_***

Subtasks:
- [ ] 1. **CODE** Создать `api/openapi.yaml` 3.1.0 с полной спецификацией:
  - openapi/info/servers (servers: `/api/v1`).
  - paths: `/posts` (get, post), `/posts/{id}` (get, patch, delete), `/posts/{id}/publish` (post), `/stats`, `/plan`, `/tags`, `/next` (get), `/auth/login`, `/auth/logout`, `/auth/csrf` (post), `/auth/oauth/{provider}` (get), `/auth/oauth/{provider}/callback` (get).
  - components.schemas: Post, Attachment (with type enum: photo|video|document|audio), PublishAttempt, ExternalLinks, LoginRequest, LoginResponse, Stats (postsByStatus, totalPosts), PlanItem, ErrorResponse, OAuthInitiateResponse (302 redirect — пометить response как 302 без body), AuthCSRFResponse.
  - components.securitySchemes: bearerAuth (HTTP Bearer "PAT"), cookieAuth (apiKey, in: cookie, name: jtpost_session).
  - Каждый защищённый endpoint: `security: [{bearerAuth: []}, {cookieAuth: []}]`. Login/logout/csrf/oauth — без security ИЛИ с security: [].
  - Все 4xx/5xx → `$ref: #/components/responses/ErrorResponse`.

- [ ] 2. **VERIFY** YAML parser-валидность через manual run `oapi-codegen` (после T-2 toolchain).

NOTE: размер этого файла ≈ 400-600 строк. Tedious но straightforward.

---

## T-2 — oapi-codegen toolchain + first generation

***_Complexity: standard_*** ***_Requirements: REQ-2.x, REQ-3.x_*** ***_Preservation: CP-1, 2, 3, 4, 14_***

Subtasks:
- [ ] 1. **CODE** Создать `tools.go` (top-level project) с:
```go
//go:build tools
package tools
import _ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
```
- [ ] 2. **CODE** `go get github.com/oapi-codegen/oapi-codegen/v2@latest && go mod tidy`.
- [ ] 3. **CODE** Создать `oapi-codegen-config.yaml`:
```yaml
package: oapigen
output: internal/adapters/httpapi/oapigen/types.gen.go
generate:
  models: true
  embedded-spec: false
output-options:
  skip-prune: true
import-mapping:
  ' #/components/schemas/uuid.UUID': github.com/google/uuid.UUID
  '#/components/schemas/Time': time.Time
compatibility:
  always-prefix-enum-values: true
```
ВАЖНО: для UUID — использовать `x-go-type` extension в openapi.yaml на UUID-fields:
```yaml
  uuid_field:
    type: string
    format: uuid
    x-go-type: uuid.UUID
    x-go-type-import:
      path: github.com/google/uuid
```
Это применять для всех `format: uuid` fields.

- [ ] 4. **CODE** В `Taskfile.yml` обновить:
```yaml
  generate:
    cmds:
      - task: generate:sqlc
      - task: generate:openapi
  generate:sqlc:
    cmds:
      - "sqlc generate"
  generate:openapi:
    cmds:
      - "go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen-config.yaml api/openapi.yaml"
```

- [ ] 5. **CODE** `mkdir -p internal/adapters/httpapi/oapigen` + `.gitkeep`.

- [ ] 6. **VERIFY** `task generate:openapi` → создаётся `internal/adapters/httpapi/oapigen/types.gen.go`. `go build ./internal/adapters/httpapi/oapigen/...` — clean.

---

## T-3 — Refactor handlers использовать generated types

***_Complexity: standard_*** ***_Requirements: REQ-4.x_*** ***_Preservation: CP-9, 10, 16_***

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/httpapi/auth_handlers.go` заменить:
  - `LoginRequest` struct → удалить, использовать `oapigen.LoginRequest` (если такой generated).
  - `LoginResponse` struct → `oapigen.LoginResponse`.
  - При несовпадении generated имён — adapt (e.g. оставить local + добавить TODO).
- [ ] 2. **CODE** В `internal/adapters/httpapi/server.go`:
  - Если generated `oapigen.Post` имеет identical поля что local `jsonPost` — заменить.
  - Если есть mismatch (generated ожидает что-то отличное) — оставить local types (TODO: alignment в follow-up).
- [ ] 3. **VERIFY** `go test ./internal/adapters/httpapi/...` — все tests pass без изменений (refactor не меняет behavior).
- [ ] 4. **VERIFY** `task build` — clean.

NOTE: Если refactor приводит к непропорциональному coupling-y, можно ограничиться usage в new code (oauth_handlers и т.д. остаются как есть). Главная цель F5 — единый source of truth (spec), не полный rewrite handlers. Pragmatically minimal в этой подзадаче.

---

## T-4 — `/api/v1/` aliases

***_Complexity: mechanical_*** ***_Requirements: REQ-5.x_*** ***_Preservation: CP-6, 8_***

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/httpapi/server.go:registerRoutes`:
  - Helper `func (s *Server) bothPrefixes(legacy string, h http.Handler)` который регистрирует `mux.Handle(legacy, h)` и `mux.Handle("/api/v1"+strings.TrimPrefix(legacy, "/api"), h)`.
  - Применить ко всем routes (posts, stats, plan, tags, auth/* — кроме `/`).
- [ ] 2. **GREEN** Создать `internal/adapters/httpapi/api_v1_test.go`:
  - TestAPIV1_PostsListAlias: 2 GET requests (на /api/posts и /api/v1/posts) → identical body.
  - TestAPIV1_AuthLoginAlias: POST /api/v1/auth/login work как POST /api/auth/login.
- [ ] 3. **VERIFY** `task test ./internal/adapters/httpapi/...` GREEN.

---

## T-5 — Spec parse test + finalization

***_Complexity: mechanical_*** ***_Requirements: REQ-6.x_*** ***_Preservation: CP-5, 7_***

Subtasks:
- [ ] 1. **GREEN** Создать `internal/adapters/httpapi/openapi_spec_test.go`:
  - TestOpenAPISpec_Exists: `os.Stat("../../../api/openapi.yaml")` not error.
  - TestOpenAPISpec_HasV3.1Header: read file, проверить `openapi: 3.1` substring.
  (Без kin-openapi parsing — это смысл `task generate:openapi` который должен fail если spec invalid.)
- [ ] 2. **CODE** Обновить `CHANGELOG.md` секцией F5.
- [ ] 3. **CODE** Обновить `.jtpost.example.yaml` или README ref к `api/openapi.yaml`.
- [ ] 4. **VERIFY** `task fmt && task vet && task test && task test:race && task generate && git diff --exit-code -- internal/adapters/httpapi/oapigen` GREEN.
- [ ] 5. **VERIFY** `task build` OK.

---

## T-6 — GATE

***_Complexity: mechanical_*** ***_Requirements: ALL_***

CRITICAL: финальная задача. T-1..T-5 complete; full sweep GREEN.
