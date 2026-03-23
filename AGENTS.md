# AGENTS.md — Руководство для AI-ассистентов по проекту jtpost

## О проекте

**jtpost** — CLI-редактор постов для управления контент-пайплайном (Telegram).

- **Модуль:** `github.com/jtprogru/jtpost`
- **Go версия:** 1.25.5
- **Архитектура:** Hexagonal/Clean Architecture (`cmd/` + `internal/core/` + `internal/adapters/`)
- **Формат ID:** UUID v7

## Структура проекта

```
jtpost/
├── cmd/jtpost/main.go      # Точка входа CLI
├── internal/
│   ├── core/               # Доменная модель (Post, PostID, PostStatus)
│   ├── adapters/           # Реализации (FS, SQLite, Telegram, HTTP)
│   └── cli/                # Cobra команды
├── Taskfile.yml            # Задачи сборки/тестирования
├── .golangci.yaml          # Конфигурация линтера
└── AGENTS.md               # Этот файл
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

## Команды разработки (Taskfile)

| Команда | Описание |
|---------|----------|
| `task run:cmd` | Запуск через `go run` |
| `task build:bin` | Сборка бинарника в `./dist/` |
| `task tidy` | `go mod tidy` |
| `task fmt` | Форматирование кода |
| `task vet` | `go vet ./...` |
| `task test` | Все тесты с coverage |
| `task test:race` | Тесты с race detector |
| `task lint` | golangci-lint |

## Линтинг

Проект использует **golangci-lint** с расширенным набором линтеров:

- Основные: `staticcheck`, `errcheck`, `gosec`, `ineffassign`, `unused`
- Стиль: `gochecknoglobals`, `gochecknoinits`, `godot`, `misspell`
- Тесты: `testifylint`, `thelper`, `tparallel`

**Важно:** Перед коммитом всегда запускать `task lint`

## Доменная модель

### Статусы поста

```go
PostStatus: "idea" → "draft" → "ready" → "scheduled" → "published"
```

### Формат поста (Markdown + Frontmatter)

```yaml
---
id: "0195e8d4-3c7a-7b2e-8f3a-9c5d6e4f2a1b"  # UUID v7
title: "Заголовок"
slug: "slug-name"
status: "draft"
deadline: "2026-02-01"
scheduled_at: "2026-02-03T10:00:00+03:00"
tags: ["golang", "cli"]
external:
  telegram_url: ""
---
Тело поста в Markdown...
```

## Принципы разработки

### 1. Разделение ответственности

- `cmd/` — только парсинг флагов и передача управления
- `internal/core/` — бизнес-логика, интерфейсы
- `internal/adapters/` — реализации (FS, HTTP, Telegram API)
- `internal/cli/` — Cobra команды, склейка

### 2. Тестирование

- Юнит-тесты на сервисы и репозитории
- Использовать `t.TempDir()` для файловых операций
- Mock интерфейсов через `internal/core/mocks` (создать при необходимости)
- Покрытие: запускать `task test:coverage`

### 3. Обработка ошибок

- Не использовать `panic` для бизнес-ошибок
- Создавать доменные ошибки в `internal/core/errors.go`
- Использовать `errors.Is` / `errors.As` для проверки

### 4. Конфигурация

- Конфиг через `.jtpost.yaml` в корне проекта
- Переопределение через env vars и CLI флаги
- Валидация конфига при инициализации

## Roadmap (приоритеты)

### Этап 7: Рефакторинг идентификаторов и упрощение архитектуры ✅
- [x] Переход на UUID v7 (`uuid.UUID` вместо `string`)
- [x] Удаление типа `Platform` и поля `Platforms`
- [x] Обновление команды `publish` (без флага `--to`)
- [ ] Отображение ID в CLI и Web UI
- [ ] Чистка Web UI (удаление раздела публикации)

### Этап 0: Скелет CLI ✅
- [x] Инициализация проекта
- [x] Команда `jtpost init` (создание `.jtpost.yaml`)
- [x] Команда `jtpost new` (создание поста по шаблону)

### Этап 1: Жизненный цикл поста ✅
- [x] `jtpost list` (фильтры: статус, теги)
- [x] `jtpost status <id> --set <status>` (смена статуса)
- [x] `jtpost show <id>` (просмотр метаданных)

### Этап 2: Интеграция с Telegram ✅
- [x] `jtpost publish <id>` (публикация в Telegram)
- [x] Конвертация Markdown → Telegram HTML/Markdown
- [x] Сохранение `telegram_url` в frontmatter

### Этап 3: Импорт постов ✅
- [x] `jtpost import` (импорт из `content/posts/`)

### Этап 4: Альтернативные хранилища ✅
- [x] SQLite хранилище
- [x] `jtpost migrate` (миграция между хранилищами)

### Этап 5: Планирование и статистика ✅
- [x] `jtpost plan` (план публикаций)
- [x] `jtpost stats` (статистика по постам)
- [x] `jtpost next` (рекомендация следующего поста)

### Этап 6: HTTP API + Web UI ✅
- [x] `jtpost serve` (встроенный сервер)
- [x] REST API endpoints
- [x] Web UI (htmx + Bootstrap)

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

## Запрещено

- ❌ Изменять `go.mod` без явной необходимости
- ❌ Добавлять зависимости без согласования
- ❌ Использовать `panic` для бизнес-ошибок
- ❌ Писать бизнес-логику в `cmd/` или `internal/cli/`
- ❌ Игнорировать ошибки (использовать `_` только с комментарием)
- ❌ Коммитить без `task lint` и `task test`
- ❌ Использовать строковые ID вместо `uuid.UUID`

## Ресурсы

- [ROADMAP.md](./ROADMAP.md) — детальное описание архитектуры
- [README.md](./README.md) — основная документация
- [QWEN.md](./QWEN.md) — контекст проекта для AI
- [Taskfile.yml](./Taskfile.yml) — доступные команды
- [.golangci.yaml](./.golangci.yaml) — правила линтинга
