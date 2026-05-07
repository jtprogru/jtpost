# Changelog

Все заметные изменения в проекте jtpost будут задокументированы в этом файле.

Формат основан на [Keep a Changelog](https://keepachangelog.com/ru/1.0.0/),
и проект придерживается [Semantic Versioning](https://semver.org/lang/ru/).

## [Неопубликовано]

### F2: Storage parity — SQLite + Postgres адаптеры с полным F1-контрактом

**Breaking changes:**
- `jtpost migrate --db <path>` больше не поддерживается. Используйте `jtpost migrate --from <fs|sqlite|postgres> --to <fs|sqlite|postgres>` и `storage.sqlite.dsn`/`storage.postgres.dsn` в конфиге. При использовании старого флага CLI завершается с кодом 2.
- `internal/adapters/sqlite` полностью переписан под F1-схему. Существующие dev-БД с F0-схемой будут пересозданы при первом `Open()` (миграция через goose делает `DROP+CREATE`). Бэкапьте посты через старую `jtpost migrate` ДО апгрейда (если данные были).
- Колонки `posts` теперь содержат `tenant_id`, `author_id`, `excerpt`, `cover_image`, `attachments`, `publish_history`, `revision`, `revision_sha`. UNIQUE(tenant_id, slug), индексы по tenant_id и составным ключам.

**Добавлено:**
- **Новые ошибки:** `core.ErrConflict` (optimistic-lock коллизия в SQL), `core.ErrMigrationFailed` (обёртка вокруг ошибок goose).
- **Postgres-адаптер** (`internal/adapters/postgres`) на `pgx/v5` + `pgxpool` с типами `uuid`/`jsonb`/`timestamptz`. Eager `Pool.Ping()` при `Open()` для fail-fast. Goose-миграции применяются автоматически.
- **SQLite-адаптер v2** (`internal/adapters/sqlite`) с типизированной кодогенерацией через **sqlc** и автоматическим применением goose-миграций при `Open()`.
- **Storage factory** (`internal/adapters/storage`) — единая точка входа: `storage.Open(cfg)` диспатчит по `cfg.Storage.Type ∈ {fs|sqlite|postgres}`. Все CLI-команды и HTTP-сервер переключены на factory.
- **Optimistic locking**: SQL-адаптеры выполняют `UPDATE ... WHERE id=? AND revision=?`. Если 0 строк затронуто и пост существует — возвращают `core.ErrConflict`. FS-адаптер не поддерживает optimistic lock (документировано).
- **Контракт-сьют** `internal/adapters/repotest.RunContract(t, factory)` — 18 поведенческих subtests гарантируют семантическую парность fs/sqlite/postgres. Capability-флаги `OptimisticLock`/`Transactions` для backend-specific сценариев.
- **`jtpost migrate db <up|status> --to <sqlite|postgres>`** — управление схемой БД через goose поверх embed-FS миграций.
- **`jtpost migrate --from --to`** — миграция данных между любыми двумя backend (через `core.MigratableRepository.ImportPosts`). Поддерживает `--dry-run` и `--overwrite`.
- **`jtpost doctor` v2** — универсальная Storage-проверка: для fs — PostsDir, для sqlite/postgres — `Open` + `Count`. Пароль в DSN маскируется (`postgres://user:***@host/db`).
- **`Config.Validate()`** усилен: при `Storage.Type=sqlite|postgres` требует непустой DSN, отрицательные `MaxOpenConns`/`MaxIdleConns` отклоняются.
- **`Config.SQLiteDSN()`** helper — приоритет `storage.sqlite.dsn` > legacy `sqlite.dsn`.
- **Taskfile**: `task generate` (sqlc), `task test:integration`, `task db:up --to <backend>`, `task db:status --to <backend>`.
- **CI**: новый job `integration-tests` (Linux only) запускает Postgres-тесты через testcontainers; `test` job проверяет актуальность sqlc-генерации (`sqlc generate && git diff --exit-code`).
- **Зависимости**: `github.com/jackc/pgx/v5`, `github.com/pressly/goose/v3`, `github.com/testcontainers/testcontainers-go` (+ `modules/postgres`).
- **Hidden** legacy-флаг `--db` остаётся зарегистрированным только для целевой обработки exit(2) с подсказкой пользователю.

**Путь миграции с F1 → F2:**
1. До апгрейда: если используется SQLite, экспортировать данные через старый `jtpost migrate` (FS → SQLite).
2. Обновить `.jtpost.yaml`: добавить/проверить `storage.sqlite.dsn` или `storage.postgres.dsn` (legacy `sqlite.dsn` тоже работает как fallback).
3. После апгрейда: первый `jtpost <cmd>` с `storage.type=sqlite` пересоздаст таблицу под F1-схему.
4. Импорт данных: `jtpost migrate --from fs --to sqlite` или `--to postgres` (при необходимости с `--overwrite`).

### F1: Foundation — расширение домена, multi-tenant readiness и новая схема конфигурации

**Breaking changes:**
- `Post` получил обязательные поля `tenant_id`, `author_id`, `created_at`, `updated_at`, `revision`. Старые посты без этих полей не загружаются (см. путь миграции ниже).
- FS-репозиторий хранит посты в подкаталогах: `<posts_dir>/<tenant_short_id>/<slug>.md`, где `tenant_short_id` — первые 8 hex-символов `tenant_id` без дефисов.
- Удалена экспортируемая `core.StatusOrder` — заменена приватной `allowedTransitions` + публичной функцией `core.IsTransitionAllowed`.
- Удалена legacy функция `getService` из `internal/cli/root.go`.
- Сигнатуры `PostService.GetStats(ctx, tenantID)` и `PostService.GetNextPost(ctx, tenantID)` теперь требуют `tenant_id`.
- HTTP API: PATCH с попыткой изменить `tenant_id` поста → 400 `{"error":"tenant_id_immutable"}`. POST/PATCH с body.tenant_id ≠ tenant scope → 403 `{"error":"tenant_mismatch"}`.

**Добавлено:**
- В `Post`: опциональные `excerpt`, `cover_image`, `attachments[]`, `publish_history[]`, `revision_sha`. Типы `Attachment{ID,Type,Path,URL,Caption,MimeType,Size}`, `PublishAttempt{ID,At,Target,Status,MessageID,ResponsePayload,Error,RetryAttempt,Duration}`.
- Новые статусы: `archived`, `failed`. Разрешены контролируемые откаты (`scheduled→ready`, `failed→ready`).
- Новые ошибки: `core.ErrInvalidTransition`, `core.ErrTenantMismatch`, `core.ErrPublishRetryExhausted`.
- Метод `Post.TenantShortID()`, `PostFilter.TenantShortID()`, `Attachment.AbsolutePath(postsDir)` (с защитой от path traversal).
- В `PostFilter` — `TenantID` (обязательный), `AuthorID`, `SortBy`, `SortOrder`, `Limit`, `Offset`. Whitelist sort-ключей: `created_at|updated_at|deadline|scheduled_at|title|status`.
- Новые секции конфига `.jtpost.yaml`: `storage.{type,git,sqlite,postgres}`, `auth.{type,secret,tenant_default,author_default,oauth,token_ttl}`, `worker.{enabled,interval,max_retries,retry_backoff}`, `server.{addr,port,base_url,read_timeout,write_timeout}`. Поддержка env-override через `JTPOST_STORAGE_TYPE`, `JTPOST_AUTH_TENANT_DEFAULT`, `JTPOST_WORKER_INTERVAL`, и т. д.
- `jtpost init` стал интерактивным: при существующем конфиге спрашивает подтверждение `[y/N]`. Флаг `--force` пропускает запрос. Генерирует `auth.tenant_default` и `auth.author_default` (UUIDv7) при первом запуске; перегенерирует `author_default` при коллизии 8-символьного префикса с `tenant_default`.
- `jtpost new --tenant <uuid> --author <uuid>` — переопределить tenant/author создаваемого поста.
- `jtpost list --format json` — валидный JSON-массив (`[]` для пустого результата).
- `internal/core/scope.go` — context-keys `WithTenant`/`TenantFromContext`, `WithAuthor`/`AuthorFromContext`.
- HTTP middleware `TenantFromConfigMiddleware(cfg)` — заглушка под F4 (auth), читает `auth.tenant_default`/`author_default` и кладёт в request context.
- `jsonPost` HTTP API расширен всеми новыми полями `Post`.
- Тесты: новый файл `internal/adapters/config/config_test.go`, расширенные `internal/core/{post,core,service}_test.go`, `internal/adapters/fsrepo/{repository,frontmatter_parser}_test.go`, `internal/adapters/httpapi/{server,middleware}_test.go`, `internal/cli/{init,new,list}_test.go`. Покрытие 49 unit-тестов + 15 PBT-substitute (тегированы `Property/N`).
- Минимальная адаптация SQLite-схемы: добавлены колонки `tenant_id`, `author_id`, `revision` (полная имплементация — F2 через sqlc/goose).

**Migration path:**
1. Удалить старый `.jtpost.yaml` (или сделать резервную копию) и запустить `jtpost init --force` — будет создан новый конфиг с автогенерированными `tenant_default`/`author_default`.
2. Если использовался SQLite-режим — удалить `.jtpost.db` (`rm .jtpost.db`) и пересоздать. Полные миграции — F2.
3. Существующие FS-посты должны быть перенесены в `content/posts/<tenant_short_id>/<slug>.md` с добавлением обязательных полей frontmatter (`tenant_id`, `author_id`, `created_at`, `updated_at`, `revision: 1`).

### Добавлено
- CI/CD пайплайн через GitHub Actions (тесты, линтинг, релизы)
- Шаблоны для Issues (bug report, feature request)
- Шаблон для Pull Request
- Руководство для участников (CONTRIBUTING.md)
- Детальный ROADMAP проекта
- GoReleaser: подпись `checksums.txt` GPG-ключом и автопубликация формулы в `jtprogru/homebrew-tap`
- GoReleaser: multi-arch Docker-образы (linux/amd64, linux/arm64) публикуются в `ghcr.io/jtprogru/jtpost` через Buildx + QEMU
- LICENSE (MIT)
- План развития проекта в `plans/DEVELOPMENT_PLAN.md` (этапы CLI Hardening → Self-hosted → SaaS)
- Web UI: колонка ID в таблице постов (короткий префикс UUID + копирование по клику)
- Web UI: поле ID и кнопка «📋 Копировать» в модальном окне редактирования поста
- Команда `jtpost doctor` — диагностика конфигурации, директории постов, SQLite, Telegram (через `getMe`) и переменной редактора. Возвращает ненулевой exit code, если есть критичные ошибки.
- Логгер переведён на `log/slog`. У `jtpost serve` появился флаг `--log-format text|json` для выбора между человекочитаемым и структурированным выводом.
- Загрузка конфигурации переведена на `viper`. Поддерживается переопределение значений переменными окружения с префиксом `JTPOST_` (например, `JTPOST_TELEGRAM_BOT_TOKEN`, `JTPOST_POSTS_DIR`, `JTPOST_SQLITE_DSN`). Приоритет: env > yaml > defaults.

### Изменено в логгировании
- Формат лог-строк: `[INFO] msg` → `level=INFO msg="..."` (text) или `{"level":"INFO","msg":"..."}` (json). Префикс теперь экспонируется как атрибут `component`.

### Изменено
- Обновлён README.md с примерами использования и бейджами
- Создан ROADMAP.md с планом развития до версии 1.0.0
- `jtpost list` теперь по умолчанию показывает колонку ID; флаг `--no-id` её скрывает (раньше логика флага была инвертирована)

### Исправлено
- Web UI: убрано обращение к несуществующему элементу `publish-status` в `closePostModal`, из-за которого при закрытии модалки в консоли возникала ошибка

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
