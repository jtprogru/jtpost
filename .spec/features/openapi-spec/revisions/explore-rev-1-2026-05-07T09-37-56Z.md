# Exploration: OpenAPI 3.1 Spec + types codegen (F5)

## Intent

B.3 из DEVELOPMENT_PLAN: «OpenAPI 3.1 спецификация + кодген». Из 4-х подпунктов B.3:

1. Объявить OpenAPI 3.1 спецификацию в `api/openapi.yaml` ← **F5 scope**
2. Кодген сервера (`oapi-codegen`) и клиента для CLI ← **F5: types-only**, server/client routing — отложено
3. CLI начинает работать через REST API сервиса (флаг `--remote`) ← **F5b отложено**
4. Версионирование API: `/api/v1/...` ← **F5: добавляем `/api/v1/` alias**

**Scope F5:**

1. Объявить полную OpenAPI 3.1 спецификацию для всех existing public endpoints в `api/openapi.yaml`:
   - `GET /api/v1/posts` (list)
   - `POST /api/v1/posts` (create)
   - `GET /api/v1/posts/{id}` (get)
   - `PATCH /api/v1/posts/{id}` (update)
   - `DELETE /api/v1/posts/{id}` (delete)
   - `POST /api/v1/posts/{id}/publish`
   - `GET /api/v1/stats`, `/api/v1/plan`, `/api/v1/tags`, `/api/v1/next`
   - `POST /api/v1/auth/login`, `/auth/logout`, `/auth/csrf`
   - `GET /api/v1/auth/oauth/{provider}`, `/callback`
   - schemas: Post, Attachment, PublishAttempt, ErrorResponse, LoginRequest, LoginResponse, etc.
2. Установить `oapi-codegen` в `tools.go` (стандартный Go-pattern для CLI-инструментов).
3. Кодген ТИПОВ (request/response models) в `internal/adapters/httpapi/oapigen/`. НЕ генерируем server/client routing.
4. Обновить existing handlers использовать generated types для JSON marshalling (refactor without behavior change).
5. Добавить `/api/v1/` aliasy для всех endpoints (старые `/api/...` остаются для backward-compat в этой фиче, deprecation в follow-up).
6. `Taskfile.yml`: задача `task generate:openapi` (отдельная от sqlc); `task generate` запускает обе.

**Чего F5 НЕ делает:**
- НЕ генерируем server-side handler interface (`oapi-codegen` server stubs) — это would require полной замены existing routes. Отложено в F5b.
- НЕ генерируем CLI Go-client. F5b.
- НЕ удаляем legacy `/api/...` routes (только добавляем `/api/v1/` aliases). Полное удаление — F5c.
- НЕ меняем internal model `core.Post` — спецификация описывает HTTP-contract, не domain.
- НЕ добавляем новые endpoints — спецификация описывает только existing.
- НЕ делаем authentication-схему security parameters в спецификации (Bearer + Cookie уже работают через middleware) — описываем как securitySchemes в OpenAPI, но без enforcement через codegen.

**Триггер:** B.2 закрыт; следующий блок B.3.

---

## Investigation

### Existing HTTP endpoints

`internal/adapters/httpapi/server.go` (line 91-105):

| Method | Path | Handler |
|--------|------|---------|
| GET, POST | /api/posts | handlePosts (list/create) |
| GET, PATCH, DELETE, POST | /api/posts/{id}[/publish] | handlePostByID |
| GET | /api/stats | handleStats |
| GET | /api/plan | handlePlan |
| GET | /api/tags | handleTags |
| POST | /api/auth/login | LoginHandler |
| POST | /api/auth/logout | LogoutHandler |
| POST | /api/auth/csrf | CSRFHandler |
| GET | /api/auth/oauth/{provider} | OAuthHandler.handleInitiate |
| GET | /api/auth/oauth/{provider}/callback | OAuthHandler.handleCallback |
| GET | / | handleIndex (HTML) |

JSON-types в существующем коде: `jsonPost` struct в `server.go` (existing manual definition), `LoginRequest`/`LoginResponse` в `auth_handlers.go`.

### oapi-codegen toolchain

- **`github.com/deepmap/oapi-codegen/v2/cmd/oapi-codegen`** или **`github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen`** — стандарт.
- Pure-Go (без CGO).
- Конфиг через `oapi-codegen-config.yaml` файлы.
- Поддерживает types-only generation (`generate: types`).

### Спецификация в Go проектах

- `tools.go` pattern: blank-imports build-tools чтобы они попадали в `go mod tidy`.
- Альтернатива: `go install` ad-hoc.
- Для F5 — `tools.go` с `_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"` под `//go:build tools`.

### Тестовый контекст

- OpenAPI yaml валидируется через `oapi-codegen` сам (parsing fail → error).
- Сгенерированные types — тестируются косвенно через handler tests (которые их используют).
- Smoke тесты: `task generate` clean, `task build` OK.

---

## Build Tooling

- **Generate:** `task generate` (sqlc) → расширяется до `task generate:sqlc + task generate:openapi`. `task generate` — alias на оба.
- **Test/Build/Lint:** без изменений.

---

## Options Considered

### Option A: Types-only codegen + manual handlers (recommended)

Сгенерировать только types из openapi.yaml; handler-routing оставить как есть. Use generated types для JSON marshalling.

- **Pros:** минимальное breaking change; handlers не переписываются; постепенно можно добавлять server-side codegen позже.
- **Cons:** не извлекаем все benefits OpenAPI (auto-validation, request binding).
- **Сложность:** Medium.

### Option B: Full server codegen (oapi-codegen ServerInterface)

Сгенерировать server interface; адаптировать существующие handlers под него.

- **Pros:** полная типизация request/response; auto-validation.
- **Cons:** большой rewrite; нужно адаптировать все handlers; conflict с существующими middleware-chain (Bearer/Session/CSRF).
- **Сложность:** High.

### Option C: OpenAPI как документация, без codegen

Только yaml, без `oapi-codegen`.

- **Pros:** zero new tooling.
- **Cons:** drift между spec и code; никакого type-safety; defeat purpose (B.3 specifies codegen).
- **Сложность:** Low. Не соответствует плану.

### Option D: gRPC + grpc-gateway

Альтернативный API contract.

- **Pros:** строже, native streaming.
- **Cons:** не в плане; HTTP/JSON остаётся ожидаемым форматом.
- **Сложность:** High.

---

## Constraints & Risks

### Backward compatibility

- Existing `/api/...` routes НЕ удаляются — добавляются `/api/v1/...` aliases. Web UI и существующие интеграции продолжают работать.
- Generated types в новом package `internal/adapters/httpapi/oapigen` — не конфликтует с существующим `jsonPost` struct (его при refactor можно удалить или сохранить как alias).
- spec файл — новый артефакт, не ломает existing.

### Performance

- Codegen — build-time, не runtime.
- Refactored handlers используют generated types вместо struct literals — performance equivalence.

### Security

- В openapi.yaml объявляем `securitySchemes`: Bearer (PAT) + cookieAuth (session) + параметрические OAuth.
- Codegen types НЕ валидируют security; это middleware (F4a-c) — без изменений.

### Edge cases

- **Field naming**: Go-style PascalCase в generated types; JSON tag — snake_case (через `additionalProperties: x-go-name` или `x-go-json-tag`).
- **UUID type**: openapi spec `format: uuid` → string в Go. Подмена на `uuid.UUID` через `x-go-type: github.com/google/uuid.UUID` requires careful imports.
- **time.Time**: format `date-time` → `time.Time`, marshalling RFC3339.
- **`*string` для optional**: openapi `nullable: true` или `required: false`. Решение: `nullable: true` для optional pointer fields.

---

## Recommended Direction

**Option A**: types-only codegen + handler refactor + `/api/v1/` alias.

Steps:
1. Создать `api/openapi.yaml` с полной спецификацией existing endpoints.
2. Создать `tools.go` с `oapi-codegen` blank-import под `//go:build tools`.
3. Создать `oapi-codegen-config.yaml` (generate types).
4. Обновить `Taskfile.yml`: `task generate:openapi` + alias.
5. Запустить codegen → `internal/adapters/httpapi/oapigen/types.gen.go`.
6. Refactor `server.go:jsonPost` → использовать `oapigen.Post`. То же для Stats/Plan/Tags handlers.
7. Refactor `auth_handlers.go:LoginRequest/LoginResponse` → `oapigen.LoginRequest`/`LoginResponse`.
8. Add `/api/v1/...` mux registrations as aliases (same handlers).
9. CHANGELOG + .jtpost.example.yaml documentation.

---

## Scope Boundaries

### Must-have (F5)

- `api/openapi.yaml` 3.1.0 со всеми existing endpoints (post CRUD, auth, stats, plan, tags, oauth).
- `tools.go` + `oapi-codegen-config.yaml`.
- `task generate:openapi` task.
- Generated types в `internal/adapters/httpapi/oapigen/types.gen.go` (commitable).
- Existing handlers используют generated types (минимум — Posts, Stats, Plan, Tags, Login, Logout, CSRF).
- `/api/v1/...` aliases для всех endpoints (legacy `/api/...` сохранён).
- Tests без regression.

### Deferred

- **F5b**: server-side codegen (ServerInterface) + handler replacement.
- **F5c**: CLI Go-client codegen + `--remote` flag.
- **F5d**: legacy `/api/...` deprecation period and removal.
- **OpenAPI** документация: ReDoc/Swagger UI на `/api/docs` — отложено.
- **Validation middleware**: kin-openapi + request validation — отложено.

### Needs spike

- **`x-go-type` для `uuid.UUID`**: oapi-codegen v2 поддерживает; нужен правильный config-syntax. Спайк в коде.
- **`additionalProperties: false` vs strictness**: для request-body ужесточить, для response — ослабить. Решение в спецификации.

---

## Assumptions & Open Questions

### Assumptions

- `[ASSUMPTION: oapi-codegen v2 (github.com/oapi-codegen/oapi-codegen/v2) — стандарт; v1 устарел]`
- `[ASSUMPTION: types-only codegen, без server-side ServerInterface]` — меньше breaking change
- `[ASSUMPTION: `/api/v1/...` — alias к `/api/...`, не replacement]` — backward-compat
- `[ASSUMPTION: UUID через x-go-type override]` — стандарт oapi-codegen
- `[ASSUMPTION: spec file — single openapi.yaml, без splitting]` — простота

### Open Questions

1. Schema split: один `openapi.yaml` или multiple files (`paths/`, `components/`)? Предложение: один файл в F5; split — в F5b при росте.
2. `securitySchemes` enforcement: только декларация или middleware-validation? Декларация (F5).
3. Versioning strategy: одновременная поддержка v1 + legacy `/api/`, или сразу force `/api/v1/` и legacy → 301? Предложение: alias (оба работают), без redirect.
4. Generated types — separate package `oapigen` или в `httpapi` напрямую? Предложение: `oapigen` — изолировано.

---

## Done When

- [x] Codebase прочитан.
- [x] 4 опции.
- [x] Trade-offs.
- [x] Scope.
- [x] Assumptions.
- [x] Open Questions (4).
- [x] Build tooling.
- [ ] Артефакт зарегистрирован.
