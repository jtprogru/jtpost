# Changelog

Все заметные изменения в проекте jtpost будут задокументированы в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.0.0/),
и проект придерживается [Semantic Versioning](https://semver.org/lang/ru/).

## [Неопубликовано]

### Добавлено
- CI/CD пайплайн через GitHub Actions (тесты, линтинг, релизы)
- Шаблоны для Issues (bug report, feature request)
- Шаблон для Pull Request
- Руководство для участников (CONTRIBUTING.md)
- Детальный ROADMAP проекта

### Изменено
- Обновлён README.md с примерами использования и бейджами
- Создан ROADMAP.md с планом развития до версии 1.0.0

---

## [0.2.0] — 2026-03-12

### Добавлено
- **Команда `jtpost import`** — импорт постов из Markdown файлов
- **Команда `jtpost migrate`** — миграция между хранилищами (FS ↔ SQLite)
- **SQLite хранилище** (`internal/adapters/sqlite`)
  - Поддержка транзакций
  - Миграции схемы БД
  - Bulk-операции
- **Логгер** (`internal/logger`)
  - Уровни: DEBUG, INFO, WARN, ERROR
  - Флаг `--verbose` для debug режима
- **Middleware** для HTTP API
  - LoggingMiddleware
  - RecoveryMiddleware
- **HTTP API endpoint `/api/next`** — рекомендация следующего поста

### Изменено
- **Удалён функционал рекомендаций** (endpoint `/api/next` удалён в 0.2.1)
- **Удалены упоминания блога** — фокус только на Telegram
- **Переименован тип** `SQLitePostRepository` → `PostRepository`
- **Заменён `interface{}` на `any`** во всех файлах

### Исправлено
- Все предупреждения golangci-lint (25 → 0)
- errcheck, errorlint, noctx, usetesting линтеры

### Документация
- Обновлён ROADMAP.md
- Обновлены CLI docs (docs/cli.md)
- Добавлена документация по SQLite (docs/sqlite.md)
- Добавлена документация по логированию (docs/logging.md)

---

## [0.1.0] — 2026-03-11

### Добавлено
- **CLI команды** (14 команд):
  - `jtpost init` — инициализация проекта
  - `jtpost new` — создание нового поста
  - `jtpost list` — список постов с фильтрацией
  - `jtpost show` — просмотр деталей поста
  - `jtpost status` — смена статуса
  - `jtpost edit` — редактирование в редакторе
  - `jtpost delete` — удаление поста
  - `jtpost publish` — публикация в Telegram
  - `jtpost plan` — план публикаций
  - `jtpost stats` — статистика по постам
  - `jtpost next` — рекомендация следующего поста
  - `jtpost serve` — запуск HTTP API сервера
- **HTTP API** с REST endpoints:
  - `GET /api/posts` — список постов
  - `GET /api/posts/{id}` — получить пост
  - `PATCH /api/posts/{id}` — обновить пост
  - `DELETE /api/posts/{id}` — удалить пост
  - `POST /api/posts` — создать пост
  - `POST /api/posts/{id}/publish` — опубликовать
  - `GET /api/stats` — статистика
  - `GET /api/plan` — план публикаций
- **Web UI** на htmx + Bootstrap
- **FileSystem репозиторий** (`internal/adapters/fsrepo`)
- **Telegram Publisher** (`internal/adapters/telegram`)
- **Markdown конвертер** (`internal/adapters/telegramconv`)
- **Доменная модель** (`internal/core`)
  - Тип `Post`, `PostID`
  - Статусы: `idea`, `draft`, `ready`, `scheduled`, `published`
  - Интерфейсы: `PostRepository`, `Publisher`

### Изменено
- Удалена поддержка блога — фокус только на Telegram
- Удалена константа `PlatformBlog`

---

## [0.0.1] — 2026-03-10

### Добавлено
- Инициализация проекта
- Базовая структура Hexagonal Architecture
- Точка входа CLI (`cmd/jtpost/main.go`)
- Конфигурация проекта (`.jtpost.example.yaml`)
- Taskfile.yml для автоматизации задач
- Настройка линтера (`.golangci.yaml`)
- Настройка релизов (`.goreleaser.yaml`)

---

## Типы изменений

- **Добавлено** — для новых функций.
- **Изменено** — для изменений в существующей функциональности.
- **Устарело** — для скорого удаления функций.
- **Удалено** — для удалённых функций.
- **Исправлено** — для исправления ошибок.
- **Безопасность** — для исправления уязвимостей.

## Версии

- **Мажорная версия** — ломающие изменения (breaking changes)
- **Минорная версия** — новые функции (обратная совместимость)
- **Патч** — исправления ошибок (обратная совместимость)

[Неопубликовано]: https://github.com/jtprogru/jtpost/compare/v0.2.0...HEAD
[0.2.0]: https://github.com/jtprogru/jtpost/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/jtprogru/jtpost/compare/v0.0.1...v0.1.0
[0.0.1]: https://github.com/jtprogru/jtpost/releases/tag/v0.0.1
