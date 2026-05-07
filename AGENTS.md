# AGENTS.md — Руководство для AI-ассистентов по проекту jtpost

## О проекте

**jtpost** — CLI и HTTP-сервис для управления жизненным циклом постов
(idea → draft → ready → scheduled → published) с публикацией в Telegram.

- **Модуль:** `github.com/jtprogru/jtpost`
- **Go версия:** 1.26.2 (CGO_ENABLED=0)
- **Архитектура:** Hexagonal — `internal/core/` (домен) + `internal/adapters/`
  (реализации) + `internal/cli/` (Cobra-команды) + `cmd/jtpost/main.go`
- **ID-формат:** UUID v7
- **Multi-tenancy + RBAC**, audit log, outbox-worker

## Структура

```
jtpost/
├── cmd/jtpost/main.go
├── internal/
│   ├── core/                       # Доменная модель, интерфейсы, сервисы
│   ├── adapters/
│   │   ├── apiclient/              # OpenAPI-codegen client (для --remote CLI)
│   │   ├── config/                 # Загрузка .jtpost.yaml + env-overrides
│   │   ├── fsrepo/                 # FileSystem PostRepository (markdown+frontmatter)
│   │   ├── gitrepo/                # Git decorator над fsrepo (auto-commit/push, history)
│   │   ├── httpapi/                # HTTP REST (oapi-codegen + middleware)
│   │   ├── postgres/               # pgx/v5 + sqlc + goose миграции
│   │   ├── repotest/               # Контрактные тесты PostRepository
│   │   ├── sqlite/                 # SQLite (modernc/sqlite) + goose миграции
│   │   ├── storage/                # Bundle factory: Posts/Users/Tokens/Sessions/OAuth/Outbox/Audit
│   │   ├── telegram/               # Telegram Bot API publisher (text+media+rate-limit)
│   │   ├── telegramconv/           # Markdown → MarkdownV2/HTML конвертер
│   │   └── webui/                  # Web UI v2 (htmx + templ), mount /ui/
│   └── cli/                        # Cobra-команды (15+ commands)
├── api/openapi.yaml                # OpenAPI 3.1 spec (source of truth для REST)
├── docs/cli/                       # Сгенерированный CLI reference (jtpost docs)
├── examples/                       # Минимальный конфиг + sample post
├── .spec/features/<name>/          # Spec-driven dev артефакты
├── ROADMAP.md / CHANGELOG.md
├── Taskfile.yml / .golangci.yaml
└── AGENTS.md (этот файл)
```

## Ключевые capabilities (что уже реализовано)

| Область | Состояние |
|---------|-----------|
| **Storage** | fs / sqlite / git / postgres (все через единый `core.PostRepository`) |
| **Bundle pattern** | `core.Bundle` объединяет Posts/Users/Tokens/Sessions/OAuthAccounts/Outbox/AuditLog |
| **Authn/RBAC** | bcrypt + Argon2id + MultiHasher (auto-detect prefix), PAT, sessions, OAuth GitHub |
| **HTTP API** | OpenAPI 3.1, oapi-codegen (types + client), 20+ endpoints, middleware: logging/CSRF/rate-limit/auth |
| **Web UI v2** | htmx + templ, /ui/ mount: dashboard, login, edit с live-preview, plan, calendar, audit, CRUD, drag-drop upload, SSE, PWA, history/diff/revert |
| **Worker / Outbox** | poll-loop с exponential backoff + recovery sweep, EventBus events |
| **Remote mode** | `--remote` для всех CLI read+write команд (через apiclient) |
| **Audit log** | append-only, hooks на post mutations + auth + token + image upload + revert |
| **Telegram media** | sendMessage / sendPhoto / sendMediaGroup; 429-handling с retry_after |
| **Git history UI** | `/ui/posts/{id}/history` + revision view + side-by-side diff + revert |

## Стандарты кода

### Стиль

- Effective Go; имена: `camelCase` для unexported, `PascalCase` для экспортов.
- Интерфейсы — узкие, по capability (`Publisher`, `HistoryProvider`, `Clock`).
- Ошибки — последним возвращаемым значением; sentinel'ы — в `internal/core/errors.go`
  (`ErrNotFound`, `ErrValidation` и др.); проверка через `errors.Is` / `errors.As`.
- Документировать все публичные экспорты (правило `godot`: предложение точкой).

### Импорты (3 группы, разделённых пустой строкой)

```go
import (
    "context"
    "fmt"

    "github.com/google/uuid"

    "github.com/jtprogru/jtpost/internal/core"
)
```

### Комментарии

- Лаконично; не дублировать имя функции в комментарии.
- Объяснять *почему*, а не *что* (последнее видно из кода).

## Команды разработки

| Команда | Описание |
|---------|----------|
| `task run:cmd` | `go run ./cmd/jtpost` |
| `task build` | сборка `./dist/jtpost` |
| `task tidy` | `go mod tidy` |
| `task fmt` | gofmt + goimports |
| `task vet` | `go vet ./...` |
| `task test` | unit-тесты + coverage |
| `task test:race` | race-detector |
| `task test:integration` | tests с `-tags=integration` (testcontainers postgres) |
| `task lint` | golangci-lint (должен быть **0 issues**) |
| `task generate` | sqlc + oapi-codegen + templ |
| `task generate:templ` | только templ |

## Линтер

`golangci-lint` с расширенным набором (см. `.golangci.yaml`). Любой PR должен
проходить `task lint` без issues. `//nolint` только с явной причиной в комментарии
(пример: detached lifecycle, sentinel error, etc.).

## Доменная модель

### Post statuses

```
idea → draft → ready → scheduled → published
```

### Frontmatter (markdown файл поста)

```yaml
---
id: "0195e8d4-3c7a-7b2e-8f3a-9c5d6e4f2a1b"  # UUID v7
tenant_id: "..."
author_id: "..."
title: "Заголовок"
slug: "slug-name"
status: "draft"
deadline: "2026-02-01"
scheduled_at: "2026-02-03T10:00:00+03:00"
tags: ["golang", "cli"]
revision: 1
created_at: "..."
updated_at: "..."
external:
  telegram_url: ""
---
Тело поста в Markdown...
```

## Принципы разработки

### 1. Слои

- `cmd/` — точка входа, минимум кода.
- `internal/core/` — домен + сервисы + интерфейсы. **Не зависит от adapter'ов.**
- `internal/adapters/` — реализации интерфейсов. Зависит от core, не от другого
  adapter'а (исключение: gitrepo decorator над fsrepo).
- `internal/cli/` — Cobra-команды; склейка config → storage → service →
  publisher; **не должна содержать бизнес-логику**.

### 2. Тестирование

- Юнит-тесты: каждый пакет имеет `*_test.go` рядом.
- Контрактные: `internal/adapters/repotest/` — общий suite, гонится для каждого
  storage backend'а.
- Integration: `//go:build integration`, через testcontainers (postgres).
- `t.Parallel()` где state не shared. Helper-pattern: `t.Helper()` в фабриках.
- Покрытие: целевое >70% per-package.

### 3. Capability interfaces

Когда у части backend'ов есть фича, которой нет у других, делай узкий
capability-интерфейс в core и type-assert'и его на месте использования.
Пример: `core.HistoryProvider` (только gitrepo) — webui рендерит stub если
интерфейс не имплементирован.

### 4. Errors

- `core.ErrNotFound`, `core.ErrValidation`, `core.ErrConflict` и пр.
- Wrap через `fmt.Errorf("ctx: %w", err)`; сравнение через `errors.Is`.
- Для HTTP ошибок — преобразование в `httpapi/errors.go` (status mapping).

### 5. Конфигурация

- `.jtpost.yaml` + env-overrides с префиксом `JTPOST_` (vложенные ключи через `_`).
- Приоритет: CLI flag > env > yaml > defaults.
- Валидация в `config.Config.Validate()`.

## Spec-driven workflow

Для нетривиальных feature'ов:

1. Создать `.spec/features/<name>/requirements.md` (goal, scope, edge cases).
2. Branch `feat/<name>` от main.
3. Реализация + тесты в той же ветке.
4. `task lint` 0 issues, `task test:race` GREEN.
5. Update ROADMAP.md (отметить ✅).
6. Commit с conventional-style сообщением, `git merge --ff-only` в main.
7. Удалить branch.

## Где искать примеры

- **Создание нового CLI-команды:** `internal/cli/docs.go` (компактный, с тестом).
- **HTTP-handler:** `internal/adapters/webui/post.go` (PRG-pattern, audit-logging).
- **Capability interface:** `internal/core/history.go` + `internal/adapters/gitrepo/history.go`.
- **Service-layer:** `internal/core/service.go` (PostService, immutability checks).
- **Repository contract test:** `internal/adapters/repotest/`.

## Запрещено

- ❌ Бизнес-логика в `cmd/` или `internal/cli/`.
- ❌ Cyclic dependencies между adapter'ами (fsrepo ⇄ webui и т.п.).
- ❌ `panic` для бизнес-ошибок.
- ❌ Игнорирование ошибок без `_ = ...` + комментарий с обоснованием.
- ❌ `string` вместо `uuid.UUID` для ID.
- ❌ Коммит без `task lint` (0 issues) и `task test:race` GREEN.
- ❌ Добавление зависимостей без обсуждения с user'ом.
- ❌ Создание `*.md` документации без явного запроса (кроме `.spec/features/`).

## Ресурсы

- [ROADMAP.md](./ROADMAP.md) — текущий план
- [README.md](./README.md) (RU) / [README.en.md](./README.en.md) (EN)
- [docs/cli/](./docs/cli/jtpost.md) — auto-generated CLI reference
- [api/openapi.yaml](./api/openapi.yaml) — OpenAPI 3.1 spec
- [Taskfile.yml](./Taskfile.yml) — task targets
- [.golangci.yaml](./.golangci.yaml) — lint config
