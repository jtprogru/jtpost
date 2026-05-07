# OpenAPI 3.1 Spec + types codegen (F5) — Requirements

**Status:** Draft · **Date:** 2026-05-07 · **Branch:** `feature/openapi-spec`

## Overview

F5 — первая часть B.3: формальная OpenAPI 3.1 спецификация всех existing public HTTP-endpoints + generated types через `oapi-codegen` (types-only). Refactor existing handlers использовать generated types для JSON marshalling. Добавлены `/api/v1/...` aliases (legacy `/api/...` остаются для backward-compat). Server-side ServerInterface codegen и CLI Go-client отложены в F5b/c.

## Glossary

| Term | Definition | Code Artifact |
|------|------------|---------------|
| `openapi.yaml` | OpenAPI 3.1 спецификация | `api/openapi.yaml` |
| `oapi-codegen` | Go OpenAPI code generator (types-only mode) | `tools.go`, `oapi-codegen-config.yaml` |
| `oapigen` | Package с generated types | `internal/adapters/httpapi/oapigen/` |
| `task generate:openapi` | Taskfile-команда для запуска oapi-codegen | `Taskfile.yml` |
| `apiV1Prefix` | Константа `/api/v1` для новых aliases | `internal/adapters/httpapi/server.go` |
| `securitySchemes` | OpenAPI security declarations: bearerAuth (PAT), cookieAuth (session) | `api/openapi.yaml` |

## User Stories

- Как **API-консьюмер**, я хочу single source of truth (`openapi.yaml`) для всех endpoints, чтобы генерировать клиенты на любом языке.
- Как **разработчик-сопровождающий**, я хочу strongly-typed Go-структуры request/response — чтобы compiler ловил mismatch.
- Как **integration developer**, я хочу `/api/v1/...` чтобы депендиться на стабильную версию API.
- Как **DevOps**, я хочу что бы `task generate` обновлял всё (sqlc + oapi-codegen) одной командой.

## Requirements

### Group 1 — OpenAPI Specification

**REQ-1.1** WHEN репозиторий собирается, the system SHALL содержать файл `api/openapi.yaml` с заголовком `openapi: 3.1.0`, `info.title: jtpost API`, `info.version: 0.10.0`.

**REQ-1.2** WHEN спецификация парсится, the system SHALL содержать paths для всех existing public endpoints под prefix `/api/v1/` (в spec — без `/api/` legacy):
- `/posts` (GET list, POST create)
- `/posts/{id}` (GET, PATCH, DELETE)
- `/posts/{id}/publish` (POST)
- `/stats`, `/plan`, `/tags`, `/next` (GET)
- `/auth/login`, `/auth/logout`, `/auth/csrf` (POST)
- `/auth/oauth/{provider}` (GET, parameter provider in path)
- `/auth/oauth/{provider}/callback` (GET, query parameters code, state)

**REQ-1.3** WHEN спецификация определяет components.schemas, the system SHALL содержать: `Post`, `PostFilter`, `Attachment`, `PublishAttempt`, `ExternalLinks`, `LoginRequest`, `LoginResponse`, `OAuthCallbackQuery`, `Stats`, `ErrorResponse`.

**REQ-1.4** WHEN спецификация определяет `Post` schema, the system SHALL включать поля: id (uuid), tenant_id (uuid), author_id (uuid), title, slug, status (enum), tags (array), excerpt (nullable), cover_image (nullable Attachment), attachments (array Attachment), publish_history (array PublishAttempt), revision (integer), revision_sha (nullable), content, deadline/scheduled_at/published_at (nullable date-time), created_at/updated_at (date-time), external (ExternalLinks).

**REQ-1.5** WHEN спецификация определяет securitySchemes, the system SHALL содержать `bearerAuth` (HTTP Bearer scheme, `bearerFormat: PAT`) и `cookieAuth` (apiKey, in: cookie, name: jtpost_session). Каждый защищённый endpoint декларирует `security: [{bearerAuth: []}, {cookieAuth: []}]`.

**REQ-1.6** WHEN спецификация определяет error responses, the system SHALL использовать `$ref: '#/components/schemas/ErrorResponse'` для всех 4xx/5xx с body `{error: string}`.

### Group 2 — Codegen toolchain

**REQ-2.1** WHEN репозиторий содержит `tools.go`, the system SHALL включать `_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"` под build-tag `//go:build tools`.

**REQ-2.2** WHEN репозиторий содержит `oapi-codegen-config.yaml`, the system SHALL декларировать generate types (только types, не server/client) с output `internal/adapters/httpapi/oapigen/types.gen.go`, package `oapigen`.

**REQ-2.3** WHEN команда `task generate:openapi` запускается, the system SHALL вызвать `go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen-config.yaml api/openapi.yaml`.

**REQ-2.4** WHEN команда `task generate` запускается, the system SHALL запустить и `generate:sqlc`, и `generate:openapi`.

**REQ-2.5** WHEN сгенерированный файл `types.gen.go` появляется, the system SHALL содержать types для всех components.schemas из openapi.yaml: `Post`, `Attachment`, `LoginRequest`, и т.д. с PascalCase Go-именами.

### Group 3 — Generated types: type overrides

**REQ-3.1** WHEN сгенерированные types содержат UUID-fields, the system SHALL использовать `github.com/google/uuid.UUID` тип (через x-go-type override в spec ИЛИ через oapi-codegen-config compatibility mappings).

**REQ-3.2** WHEN сгенерированные types содержат date-time fields, the system SHALL использовать `time.Time`.

**REQ-3.3** WHEN сгенерированные types содержат nullable optional fields, the system SHALL использовать pointer types (`*string`, `*time.Time`, и т.д.).

### Group 4 — Handler refactor

**REQ-4.1** WHEN HTTP-handler `LoginHandler` обрабатывает request, the system SHALL декодировать body как `oapigen.LoginRequest` (вместо local struct) и кодировать response как `oapigen.LoginResponse`.

**REQ-4.2** WHEN HTTP-handler `handlePosts` (list) сериализует response, the system SHALL использовать массив `oapigen.Post` (или legacy `jsonPost` через адаптер). Поведение byte-equivalent.

**REQ-4.3** WHEN существующие handler-tests запускаются, the system SHALL все pass без изменений (refactor не меняет behavior).

### Group 5 — `/api/v1/` aliases

**REQ-5.1** WHEN HTTP-сервер запускается, the system SHALL зарегистрировать routes под `/api/v1/` префиксом для всех endpoints из REQ-1.2: `/api/v1/posts`, `/api/v1/posts/`, `/api/v1/stats`, `/api/v1/plan`, `/api/v1/tags`, `/api/v1/auth/...`.

**REQ-5.2** WHEN HTTP-запрос приходит на legacy `/api/posts`, the system SHALL продолжить обрабатывать (backward-compat). НЕ возвращает 301/410.

**REQ-5.3** WHEN HTTP-запрос приходит на `/api/v1/posts`, the system SHALL обработать identical legacy `/api/posts` (same handler).

### Group 6 — Тесты

**REQ-6.1** WHEN запускается `task test`, the system SHALL пройти existing handler-тесты без regression.

**REQ-6.2** WHEN запускается `task generate && git diff --exit-code -- internal/adapters/httpapi/oapigen`, the system SHALL быть clean (generated code актуален).

**REQ-6.3** WHEN запускается тест `TestAPIV1_AliasRoutes`, the system SHALL проверить что endpoint `/api/v1/posts` возвращает identical response к `/api/posts`.

**REQ-6.4** WHEN запускается тест `TestOpenAPISpec_Parses`, the system SHALL парсить `api/openapi.yaml` через openapi-parser библиотеку (или просто прогнать через oapi-codegen — fail если spec invalid).

## Topological Order

```
Group 1 (Spec) → Group 2 (Toolchain) → Group 3 (Type overrides; внутри generated)
       ↓
Group 4 (Handler refactor) → Group 5 (v1 aliases) → Group 6 (Tests)
```

## Verification Commands

| Action | Command |
|--------|---------|
| Test | `task test` |
| Race | `task test:race` |
| Build | `task build` |
| Lint | `task lint` |
| Generate | `task generate` |
| Generate (openapi only) | `task generate:openapi` |
