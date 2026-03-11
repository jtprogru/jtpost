# jtpost — Контекст для AI-ассистента

## Обзор проекта

**jtpost** — CLI-инструмент для управления контент-пайплайном (Telegram).

- **Модуль:** `github.com/jtprogru/jtpost`
- **Go версия:** 1.25.5
- **Архитектура:** Hexagonal/Clean Architecture (`cmd/` + `internal/core/` + `internal/adapters/`)
- **Статус:** Ранняя стадия разработки (скелет проекта)

## Структура проекта

```
jtpost/
├── cmd/jtpost/main.go          # Точка входа CLI
├── internal/
│   └── core/
│       └── core.go             # Доменные типы (PostStatus, Platform)
├── .golangci.yaml              # Конфигурация линтера
├── .goreleaser.yaml            # Конфигурация релизов
├── Taskfile.yml                # Задачи сборки/тестирования
├── mcp.json                    # MCP серверы
├── AGENTS.md                   # Руководство для AI-ассистентов
├── ROADMAP.md                  # Детальное описание архитектуры
├── NextSteps.md                # Структура пакетов и интерфейсы
└── MCP_SETUP.md                # Настройка MCP
```

## Целевая архитектура

```
internal/
├── core/                       # Доменная модель и use-case'ы
│   ├── post.go                 # Тип Post, PostID, ExternalLinks
│   ├── status.go               # PostStatus, Platform константы
│   ├── service.go              # PostService, PlanningService
│   ├── publisher.go            # Интерфейс Publisher
│   ├── repository.go           # Интерфейс PostRepository
│   └── errors.go               # Доменные ошибки
├── adapters/                   # Реализации портов
│   ├── fsrepo/                 # FilesystemPostRepository
│   ├── config/                 # Конфигурация (.jtpost.yaml)
│   ├── telegram/               # TelegramPublisher
│   ├── blog/                   # BlogPublisher (Hugo/статик)
│   └── httpapi/                # HTTP API (позже)
└── cli/                        # Cobra команды
    ├── root.go
    ├── init.go
    ├── new.go
    ├── list.go
    ├── status.go
    └── publish.go
```

## Сборка и запуск

### Требования

- Go 1.25.5+
- [Task](https://taskfile.dev/) (опционально, для удобства)
- golangci-lint (для линтинга)

### Команды (Taskfile.yml)

| Команда | Описание |
|---------|----------|
| `task run:cmd` | Запуск через `go run cmd/jtpost/main.go` |
| `task build:bin` | Сборка бинарника в `./dist/` |
| `task tidy` | `go mod tidy` |
| `task fmt` | `gofmt -s -w .` |
| `task vet` | `go vet ./...` |
| `task test` | Все тесты с coverage |
| `task test:race` | Тесты с race detector |
| `task test:coverage` | Отчёт по покрытию |
| `task lint` | golangci-lint |
| `task install:global` | Установка глобально (`go install`) |

### Ручные команды (без Task)

```bash
# Запуск
go run cmd/jtpost/main.go

# Сборка
CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go

# Тесты
go test -coverprofile=cover.out -v ./...
go test -race -v ./...

# Линтинг
golangci-lint -v run

# Форматирование
gofmt -s -w .

# Модули
go mod tidy
```

## Доменная модель

### Статусы поста

```go
PostStatus: "idea" → "draft" → "ready" → "scheduled" → "published"
```

### Платформы

```go
Platform: "blog", "telegram"
```

### Формат поста (Markdown + Frontmatter)

```yaml
---
title: "Заголовок"
slug: "slug-name"
status: "draft"
platforms: ["blog", "telegram"]
deadline: "2026-02-01"
scheduled_at: "2026-02-03T10:00:00+03:00"
tags: ["golang", "cli"]
external:
  blog_url: ""
  telegram_url: ""
---
Тело поста в Markdown...
```

## Стандарты кода

### Стиль Go

- Следовать [Effective Go](https://go.dev/doc/effective_go)
- Имена переменных: `camelCase`, константы: `PascalCase`
- Интерфейсы: односложные имена (`Reader`, `Writer`, `Publisher`)
- Ошибки: возвращать последним значением, проверять сразу

### Импорт

- Группировка импортов: стандартные → внешние → локальные
- Использовать `go fmt` или `gofmt -s -w .`

### Комментарии

- Документировать все публичные экспорты
- Формат: `// FunctionName делает что-то.`
- Не комментировать очевидное

## Линтинг

Проект использует **golangci-lint** с расширенным набором линтеров:

- **Основные:** `staticcheck`, `errcheck`, `gosec`, `ineffassign`, `unused`
- **Стиль:** `gochecknoglobals`, `gochecknoinits`, `godot`, `misspell`
- **Тесты:** `testifylint`, `thelper`, `tparallel`

**Важно:** Перед коммитом всегда запускать `task lint`

## Тестирование

### Практики

- Юнит-тесты на сервисы и репозитории
- Использовать `t.TempDir()` для файловых операций
- Mock интерфейсов через `internal/core/mocks` (создать при необходимости)
- Покрытие: запускать `task test:coverage`

### Запрещено

- ❌ Игнорировать ошибки (использовать `_` только с комментарием)
- ❌ Использовать `panic` для бизнес-ошибок
- ❌ Писать бизнес-логику в `cmd/` или `internal/cli/`

## Roadmap (приоритеты)

### Этап 0: Скелет CLI ✅
- [x] Инициализация проекта
- [ ] Команда `post init` (создание `.jtpost.yaml`)
- [ ] Команда `post new` (создание поста по шаблону)

### Этап 1: Жизненный цикл поста
- [ ] `post list` (фильтры: статус, теги, платформы)
- [ ] `post status <id> --set <status>` (смена статуса)
- [ ] `post show <id>` (просмотр метаданных)

### Этап 2: Интеграция с блогом
- [ ] `post publish --to blog <id>`
- [ ] Поддержка Hugo-структуры
- [ ] Опциональный git-commit

### Этап 3: Telegram
- [ ] `post publish --to telegram <id>`
- [ ] Конвертация Markdown → Telegram HTML/Markdown
- [ ] Сохранение `telegram_url` в frontmatter

### Этап 4: Планирование
- [ ] `post plan` (календарь/дедлайны)
- [ ] `post next` (рекомендация следующего поста)
- [ ] `post stats` (статистика по постам)

### Этап 5: HTTP API + Web UI
- [ ] `post serve` (встроенный сервер)
- [ ] REST API (`/posts`, `/posts/{id}`)
- [ ] Простой HTML/htmx UI

## MCP серверы

Проект настроен для работы с MCP (Model Context Protocol):

- **filesystem** — доступ к файловой системе
- **git** — Git операции
- **golangci-lint** — линтинг кода
- **go-test** — запуск тестов
- **context7** — документация по Go пакетам
- **telegram** — Telegram Bot API (для этапа 3)
- **github** — GitHub API (для code review)

Конфигурация в `mcp.json` и `~/.qwen/mcp.json`.

## Принципы разработки

### 1. Разделение ответственности

- `cmd/` — только парсинг флагов и передача управления
- `internal/core/` — бизнес-логика, интерфейсы
- `internal/adapters/` — реализации (FS, HTTP, Telegram API)
- `internal/cli/` — Cobra команды, склейка

### 2. Обработка ошибок

- Не использовать `panic` для бизнес-ошибок
- Создавать доменные ошибки в `internal/core/errors.go`
- Использовать `errors.Is` / `errors.As` для проверки

### 3. Конфигурация

- Конфиг через `.jtpost.yaml` в корне проекта
- Переопределение через env vars и CLI флаги
- Валидация конфига при инициализации

## Генерация кода

При создании новых файлов:

1. **Доменные типы** → `internal/core/`
2. **Интерфейсы** → `internal/core/` (имя файла по интерфейсу)
3. **Реализации** → `internal/adapters/<name>/`
4. **CLI команды** → `internal/cli/<command>.go`
5. **Тесты** → `<имя_файла>_test.go` в той же директории

## Примеры кода

### Создание нового сервиса

```go
// internal/core/post_service.go
package core

import "context"

type PostService struct {
    repo  PostRepository
    clock Clock
}

func NewPostService(repo PostRepository, clock Clock) *PostService {
    return &PostService{repo: repo, clock: clock}
}

func (s *PostService) CreatePost(ctx context.Context, title string) (*Post, error) {
    // ...
}
```

### Создание адаптера

```go
// internal/adapters/fsrepo/repository.go
package fsrepo

import "github.com/jtprogru/jtpost/internal/core"

type FileSystemRepository struct {
    rootPath string
}

func NewFileSystemRepository(rootPath string) *FileSystemRepository {
    return &FileSystemRepository{rootPath: rootPath}
}

func (r *FileSystemRepository) GetByID(ctx context.Context, id core.PostID) (*core.Post, error) {
    // ...
}
```

### CLI команда

```go
// internal/cli/new.go
package cli

import (
    "github.com/spf13/cobra"
)

var newCmd = &cobra.Command{
    Use:   "new [title]",
    Short: "Создать новый пост",
    RunE: func(cmd *cobra.Command, args []string) error {
        // ...
    },
}
```

## Ресурсы

- [README.md](./README.md) — краткое описание
- [ROADMAP.md](./ROADMAP.md) — детальное описание архитектуры
- [NextSteps.md](./NextSteps.md) — структура пакетов и интерфейсы
- [AGENTS.md](./AGENTS.md) — руководство для AI-ассистентов
- [Taskfile.yml](./Taskfile.yml) — доступные команды
- [.golangci.yaml](./.golangci.yaml) — правила линтинга
- [MCP_SETUP.md](./MCP_SETUP.md) — настройка MCP
