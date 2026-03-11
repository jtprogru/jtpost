# Архитектура проекта jtpost

## Обзор

**jtpost** — CLI-инструмент для управления контент-пайплайном Telegram-канала, построенный с использованием **Hexagonal Architecture** (Ports & Adapters).

## Принципы архитектуры

### Hexagonal Architecture

Проект следует принципам гексагональной архитектуры:

```
                    ┌─────────────────┐
                    │   CLI (Cobra)   │
                    └────────┬────────┘
                             │
┌────────────────────────────┼────────────────────────────┐
│                    Core Layer                           │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   Service   │──│   Domain    │──│  Interfaces │     │
│  │             │  │   Types     │  │   (Ports)   │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
└────────────────────────────┬────────────────────────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼────────┐  ┌───────▼────────┐  ┌───────▼────────┐
│  Filesystem    │  │   Telegram     │  │    HTTP API    │
│  Repository    │  │   Publisher    │  │    Server      │
└────────────────┘  └────────────────┘  └────────────────┘
     Adapters           Adapters           Adapters
```

### Слои архитектуры

1. **Core Layer (internal/core/)** — бизнес-логика и доменные типы
   - Не зависит от внешних зависимостей
   - Содержит интерфейсы (порты) для адаптеров
   - Определяет доменные модели и правила

2. **Adapters Layer (internal/adapters/)** — реализации портов
   - Filesystem repository — хранение постов в файлах
   - Telegram publisher — публикация в Telegram
   - HTTP API server — REST API и Web UI
   - Config loader — загрузка конфигурации

3. **CLI Layer (internal/cli/)** — команды Cobra
   - Парсинг аргументов и флагов
   - Вызов сервисов core слоя
   - Форматирование вывода

4. **Application Layer (cmd/)** — точка входа
   - Инициализация приложения
   - Передача управления в CLI

## Структура проекта

```
jtpost/
├── cmd/jtpost/
│   └── main.go                 # Точка входа, версия приложения
│
├── internal/
│   ├── core/                   # Доменный слой
│   │   ├── core.go             # Типы PostStatus, Platform, константы
│   │   ├── post.go             # Тип Post, PostID, ExternalLinks, PostFilter
│   │   ├── service.go          # PostService — бизнес-логика
│   │   ├── repository.go       # Интерфейс PostRepository (порт)
│   │   ├── publisher.go        # Интерфейс Publisher (порт)
│   │   ├── errors.go           # Доменные ошибки
│   │   ├── clock.go            # Интерфейс Clock для тестируемости
│   │   └── slug.go             # Генерация slug из заголовка
│   │
│   ├── adapters/               # Слой адаптеров
│   │   ├── config/
│   │   │   └── config.go       # Загрузка/сохранение .jtpost.yaml
│   │   ├── fsrepo/
│   │   │   └── repository.go   # FilesystemPostRepository
│   │   ├── telegram/
│   │   │   └── publisher.go    # TelegramPublisher
│   │   ├── telegramconv/
│   │   │   └── converter.go    # Конвертация Markdown → Telegram HTML
│   │   └── httpapi/
│   │       ├── server.go       # HTTP сервер, REST API
│   │       └── templates/
│   │           └── index.html  # Web UI (htmx)
│   │
│   └── cli/                    # CLI команды
│       ├── root.go             # Корневая команда, глобальные флаги
│       ├── init.go             # jtpost init
│       ├── new.go              # jtpost new
│       ├── list.go             # jtpost list
│       ├── show.go             # jtpost show
│       ├── status.go           # jtpost status
│       ├── edit.go             # jtpost edit
│       ├── delete.go           # jtpost delete
│       ├── publish.go          # jtpost publish
│       ├── plan.go             # jtpost plan
│       ├── stats.go            # jtpost stats
│       ├── next.go             # jtpost next
│       └── serve.go            # jtpost serve
│
├── templates/                  # Шаблоны постов
├── content/posts/              # Директория с постами (по умолчанию)
├── testdata/                   # Тестовые данные
│
├── .jtpost.yaml                # Конфигурация проекта
├── Taskfile.yml                # Задачи сборки/тестирования
├── .golangci.yaml              # Конфигурация линтера
└── .goreleaser.yaml            # Конфигурация релизов
```

## Доменная модель

### Пост (Post)

```go
type Post struct {
    ID          PostID          // Уникальный идентификатор
    Title       string          // Заголовок
    Slug        string          // URL-дружественный идентификатор
    Status      PostStatus      // Статус в жизненном цикле
    Platforms   []Platform      // Целевые платформы
    Tags        []string        // Теги
    Deadline    *time.Time      // Дедлайн
    ScheduledAt *time.Time      // Запланированное время публикации
    PublishedAt *time.Time      // Фактическое время публикации
    Content     string          // Тело поста (Markdown)
    External    ExternalLinks   // Ссылки на опубликованные посты
}
```

### Жизненный цикл поста

```
┌──────┐    ┌───────┐    ┌───────┐    ┌───────────┐    ┌───────────┐
│ idea │───▶│ draft │───▶│ ready │───▶│ scheduled │───▶│ published │
└──────┘    └───────┘    └───────┘    └───────────┘    └───────────┘
   │            │            │                              ▲
   │            │            └──────────────────────────────┘
   │            │
   └────────────┴───────────────────────────────────────────┘
```

**Статусы:**
- `idea` — идея, требует проработки
- `draft` — активная работа над постом
- `ready` — готов к публикации
- `scheduled` — запланирован на дату
- `published` — опубликован

**Правила переходов:**
- Переход возможен только вперёд по жизненному циклу
- Из любого статуса можно перейти в `published` (минуя промежуточные)

### Платформы

```go
type Platform string

const (
    PlatformTelegram Platform = "telegram"
)
```

### Формат файла поста

Посты хранятся в Markdown с YAML frontmatter:

```yaml
---
id: 1772824850782694000-moy-pervyy-post
title: Мой первый пост
slug: moy-pervyy-post
status: idea
platforms:
  - telegram
tags:
  - go
  - cli
deadline: "2026-02-01T18:00:00+03:00"
scheduled_at: "2026-02-03T10:00:00+03:00"
external:
  telegram_url: ""
---

# Заголовок поста

Тело поста в формате Markdown...
```

## Интерфейсы (Порты)

### PostRepository

Интерфейс для хранения и извлечения постов:

```go
type PostRepository interface {
    Create(ctx context.Context, post *Post) error
    GetByID(ctx context.Context, id PostID) (*Post, error)
    List(ctx context.Context, filter PostFilter) ([]*Post, error)
    Update(ctx context.Context, post *Post) error
    Delete(ctx context.Context, id PostID) error
}
```

**Реализация:** `internal/adapters/fsrepo.FileSystemRepository`

### Publisher

Интерфейс для публикации постов на внешние платформы:

```go
type Publisher interface {
    Platform() Platform
    Publish(ctx context.Context, post *Post) (*Post, error)
}
```

**Реализации:**
- `internal/adapters/telegram.Publisher` — публикация в Telegram

### Clock

Интерфейс для получения текущего времени (для тестируемости):

```go
type Clock interface {
    Now() time.Time
}
```

**Реализация:** `internal/core.SystemClock`

## Сервисы

### PostService

Основной сервис для управления постами:

```go
type PostService struct {
    repo  PostRepository
    clock Clock
}
```

**Методы:**
- `CreatePost()` — создание нового поста
- `GetByID()` — получение поста по ID
- `ListPosts()` — список постов с фильтрами
- `UpdateStatus()` — изменение статуса
- `UpdatePost()` — обновление поста
- `DeletePost()` — удаление поста
- `GetStats()` — статистика по постам
- `GetNextPost()` — рекомендация следующего поста
- `PublishPost()` — публикация на платформы

## Адаптеры

### FilesystemRepository (`internal/adapters/fsrepo/`)

Хранит посты в файловой системе:

- **Создание:** запись нового `.md` файла в директорию постов
- **Чтение:** парсинг frontmatter + content из файла
- **Обновление:** запись изменённого файла
- **Удаление:** удаление файла
- **Список:** сканирование директории, фильтрация по `.md` файлам

**Формат ID:** `<timestamp-nano>-<slug>` (например, `1772824850782694000-moy-pervyy-post`)

### TelegramPublisher (`internal/adapters/telegram/`)

Публикует посты в Telegram:

- Конвертирует Markdown → Telegram HTML
- Отправляет сообщение через Telegram Bot API
- Сохраняет ссылку на опубликованное сообщение
- Поддерживает режим dry-run для предпросмотра

**Конвертация:**
- Заголовки `#` → `<b>bold</b>`
- Ссылки `[text](url)` → `<a href="url">text</a>`
- Код `` `code` `` → `<code>code</code>`
- Блоки кода → `<pre>code</pre>`

### HTTPAPIServer (`internal/adapters/httpapi/`)

REST API сервер с Web UI:

**Endpoints:**
- `GET /api/posts` — список постов (с фильтрами)
- `GET /api/posts/{id}` — получить пост
- `POST /api/posts` — создать пост
- `PATCH /api/posts/{id}` — обновить пост
- `DELETE /api/posts/{id}` — удалить пост
- `POST /api/posts/{id}/publish` — опубликовать пост
- `GET /api/stats` — статистика
- `GET /api/next` — рекомендация
- `GET /api/plan` — план публикаций

**Web UI:**
- Построен на [htmx](https://htmx.org)
- SPA-подобный опыт без JavaScript-фреймворков
- Динамическая фильтрация и обновление
- Модальные окна для редактирования

### ConfigAdapter (`internal/adapters/config/`)

Загрузка и сохранение конфигурации:

- Чтение `.jtpost.yaml`
- Поддержка переменных окружения (`${VAR}`)
- Значения по умолчанию
- Валидация конфигурации

## Взаимодействие компонентов

### Создание поста

```
┌─────┐      ┌─────┐      ┌──────────┐      ┌──────────┐
│ CLI │─────▶│ New │─────▶│   Core   │─────▶│   FS     │
│     │      │ Cmd │      │ Service  │      │ Repo     │
└─────┘      └─────┘      └──────────┘      └──────────┘
                │                                  │
                │                                  ▼
                │                          Запись файла
                │                                  │
                ▼                                  ▼
           Открытие в                        Подтверждение
           редакторе
```

### Публикация в Telegram

```
┌─────┐      ┌─────────┐      ┌──────────┐      ┌──────────┐
│ CLI │─────▶│ Publish │─────▶│   Core   │─────▶│ Telegram │
│     │      │   Cmd   │      │ Service  │      │ Publisher│
└─────┘      └─────────┘      └──────────┘      └──────────┘
                                    │                 │
                                    ▼                 ▼
                              Проверка статуса   Отправка в API
                              Обновление ExternalLinks
```

### HTTP API запрос

```
┌──────────┐      ┌──────────┐      ┌──────────┐      ┌──────────┐
│ Browser  │─────▶│   HTTP   │─────▶│   Core   │─────▶│   FS     │
│          │      │  Server  │      │ Service  │      │ Repo     │
└──────────┘      └──────────┘      └──────────┘      └──────────┘
     │                  │                                    │
     ▼                  ▼                                    ▼
  HTMX запрос     JSON ответ                          Чтение файлов
  Обновление DOM
```

## Тестируемость

Проект спроектирован с учётом тестируемости:

### Интерфейс Clock

Позволяет подменять реальное время в тестах:

```go
type Clock interface {
    Now() time.Time
}

type SystemClock struct{}

func (c SystemClock) Now() time.Time { return time.Now() }
```

### Mock для тестов

Интерфейсы `PostRepository` и `Publisher` позволяют создавать mock-реализации для юнит-тестов сервисов.

### t.TempDir()

Тесты репозитория используют `t.TempDir()` для создания временных директорий, которые автоматически удаляются после теста.

## Зависимости

### Основные

- **cobra** — CLI фреймворк
- **yaml.v3** — парсинг YAML
- **htmx** — Web UI (без сборки)

### Инструменты разработки

- **golangci-lint** — линтинг
- **goreleaser** — сборка релизов
- **Task** — управление задачами

## Расширение архитектуры

### Добавление новой платформы

1. Создать адаптер в `internal/adapters/<platform>/`
2. Реализовать интерфейс `Publisher`
3. Добавить константу `Platform<Name>` в `internal/core/core.go`
4. Обновить `publishCmd` в `internal/cli/publish.go`
5. Добавить publisher в `serveCmd`

### Добавление нового поля поста

1. Добавить поле в `internal/core/post.go`
2. Обновить парсинг frontmatter в `internal/adapters/fsrepo/`
3. Обновить CLI команды при необходимости
4. Обновить API и Web UI

### Добавление HTTP endpoint

1. Добавить обработчик в `internal/adapters/httpapi/server.go`
2. Зарегистрировать маршрут в `registerRoutes()`
3. Обновить документацию API

## Безопасность

### Конфиденциальные данные

- Токены и чувствительные данные хранятся в переменных окружения
- Конфигурация поддерживает синтаксис `${VAR}`
- Файл `.jtpost.yaml` рекомендуется добавлять в `.gitignore`

### Валидация

- Все входные данные валидируются
- Статусы проверяются на допустимость перехода
- Платформы проверяются на поддержку

## Производительность

### Кэширование

В текущей реализации кэширование не используется. Посты читаются из файловой системы при каждом запросе.

### Оптимизации

- Индексация постов по ID для быстрого поиска
- Пузырьковая сортировка для небольших списков (план, рекомендации)
- Lazy loading контента в Web UI

## Масштабируемость

### Ограничения

- Filesystem repository подходит для ≤1000 постов
- Отсутствие базы данных ограничивает возможности фильтрации
- Нет поддержки многопользовательского режима

### Будущие улучшения

- Поддержка SQLite/PostgreSQL репозитория
- Индексация для быстрого поиска
- Фоновые задачи для отложенных операций
- WebSocket для real-time обновлений в Web UI
