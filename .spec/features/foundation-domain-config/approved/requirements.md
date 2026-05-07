# Foundation — Domain Model & Configuration (F1) — Requirements

**Status:** Draft
**Author:** Claude (Opus 4.7) + Mikhail Savin
**Date:** 2026-05-06
**Feature:** foundation-domain-config (F1)
**Branch:** `feature/foundation-domain-config`

## Overview

F1 — фундаментальная фича программы доведения jtpost до финала. Расширяет доменную модель `core.Post` под нужды последующих фич (multi-tenant, авто-публикация, git-история, медиа в Telegram, история публикаций с retry), расширяет жизненный цикл статусов (`archived`, `failed`, контролируемые откаты), вводит новую схему конфигурации (`storage`, `auth`, `worker`, `server`), генерирует `tenant_default` и `author_default` UUIDv7 при `jtpost init`, переводит файловое хранилище на `content/posts/<tenant_short_id>/<slug>.md`, обновляет HTTP API (`jsonPost`) и CLI (`list --format json`, удаление legacy `getService`). Фича не реализует Postgres/Git/auth/worker/Telegram-media — только готовит фундамент для них.

## Glossary

| Term | Definition | Code Artifact |
|------|------------|---------------|
| `TenantID` | UUIDv7 идентификатор арендатора (изолятор данных в multi-tenant-ready модели) | `internal/core/post.go` (`Post.TenantID`) |
| `AuthorID` | UUIDv7 идентификатор автора поста (отдельная от TenantID сущность) | `internal/core/post.go` (`Post.AuthorID`) |
| `TenantShortID` | Первые 8 hex-символов `TenantID` (используются как имя подкаталога FS) | `internal/core/post.go` (`Post.TenantShortID()`), `internal/adapters/fsrepo/repository.go` |
| `Attachment` | Медиафайл, прикреплённый к посту (фото/видео/документ) | `internal/core/post.go` (`Attachment`) |
| `CoverImage` | Обложка поста — отдельное опциональное поле типа `*Attachment` | `internal/core/post.go` (`Post.CoverImage`) |
| `PublishAttempt` | Запись попытки публикации (успех/провал, retry, длительность) | `internal/core/post.go` (`PublishAttempt`) |
| `PublishHistory` | Список последних `PublishAttempt` (в FS-frontmatter ограничен 10 записями) | `internal/core/post.go` (`Post.PublishHistory`) |
| `Revision` | Монотонный счётчик ревизий поста (++1 при каждом Update) | `internal/core/post.go` (`Post.Revision`) |
| `RevisionSHA` | Опциональный SHA-идентификатор ревизии (заполняется только в git-режиме) | `internal/core/post.go` (`Post.RevisionSHA`) |
| `AllowedTransitions` | Явная таблица допустимых переходов между статусами (заменяет индексное вычисление) | `internal/core/core.go` (`AllowedTransitions`) |
| `StorageType` | Тип хранилища: `fs`, `sqlite`, `postgres` (выбирается через `storage.type`) | `internal/adapters/config/config.go` (`StorageConfig.Type`) |
| `TenantContextKey` | Ключ для извлечения `TenantID` из `context.Context` в HTTP-хендлерах | `internal/adapters/httpapi/middleware.go` |

## User Stories

- Как **владелец канала**, я хочу при `jtpost init` получать готовый `.jtpost.yaml` со всеми разделами (storage/auth/worker/server) и автогенерированными `tenant_default`/`author_default`, чтобы не настраивать вручную.
- Как **владелец канала**, я хочу, чтобы `jtpost init` не молча перезаписывал существующий конфиг, а просил подтверждение, чтобы не потерять ручные настройки.
- Как **разработчик-сопровождающий**, я хочу обязательные поля `TenantID`, `AuthorID`, `CreatedAt`, `UpdatedAt` в `Post`, чтобы все последующие фичи (Postgres, auth, worker) могли опираться на единую модель.
- Как **владелец канала**, я хочу архивировать опубликованные посты (`archived`) и помечать неудачные публикации (`failed`), чтобы план и статистика не засорялись.
- Как **API-консьюмер**, я хочу получать через `jtpost list --format json` валидный JSON-массив, а не плейсхолдер, чтобы интегрировать CLI в скрипты.
- Как **разработчик**, я хочу, чтобы `tenant_id` поста был неизменен после создания, чтобы избежать сложных миграций данных и race conditions.

## Requirements

### Group 1 — Расширение `Post` (обязательные поля)

**REQ-1.1** WHEN репозиторий вызывает `Create(ctx, post)`, the system SHALL отклонить запрос с ошибкой `ErrValidation`, если у `post` пустое поле `TenantID` (нулевой UUID).

**REQ-1.2** WHEN репозиторий вызывает `Create(ctx, post)`, the system SHALL отклонить запрос с ошибкой `ErrValidation`, если у `post` пустое поле `AuthorID` (нулевой UUID).

**REQ-1.3** WHEN сервис вызывает `CreatePost(ctx, input)`, the system SHALL установить `Post.CreatedAt` и `Post.UpdatedAt` равными `Clock.Now()` и `Post.Revision` равным `1`.

**REQ-1.4** WHEN сервис вызывает `UpdatePost(ctx, post)`, the system SHALL установить `Post.UpdatedAt = Clock.Now()` и инкрементировать `Post.Revision` на 1.

**REQ-1.5** WHEN сервис вызывает `UpdatePost(ctx, post)` и значение `post.TenantID` отличается от значения, хранящегося в репозитории по `post.ID`, the system SHALL вернуть ошибку `ErrTenantMismatch` и не изменять данные.

### Group 2 — Расширение `Post` (опциональные поля)

**REQ-2.1** WHEN сервис сериализует `Post` (в YAML, JSON или БД), the system SHALL включить поле `Excerpt *string`, omitting его из вывода если значение `nil`.

**REQ-2.2** WHEN сервис сериализует `Post`, the system SHALL включить поле `CoverImage *Attachment`, omitting его из вывода если значение `nil`.

**REQ-2.3** WHEN сервис сериализует `Post`, the system SHALL включить поле `Attachments []Attachment` со структурой `{ID uuid.UUID, Type AttachmentType, Path string, URL string, Caption string, MimeType string, Size int64}`, omitting его из вывода если массив пуст.

**REQ-2.4** WHEN сервис сериализует `Post`, the system SHALL включить поле `PublishHistory []PublishAttempt` со структурой `{ID uuid.UUID, At time.Time, Target string, Status string, MessageID string, ResponsePayload json.RawMessage, Error string, RetryAttempt int, Duration time.Duration}`, omitting его из вывода если массив пуст.

**REQ-2.5** WHEN сервис сериализует `Post` в YAML-frontmatter (FS-режим) и `len(PublishHistory) > 10`, the system SHALL записать только последние 10 элементов массива (отсортированы по `At` desc).

**REQ-2.6** WHEN репозитор десериализует `Post` (FS, SQLite, Postgres), the system SHALL заполнять `Revision int` и опциональное `RevisionSHA *string`, где `RevisionSHA == nil` для не-git-режимов.

**REQ-2.7** WHEN тип `AttachmentType` сериализуется, the system SHALL поддерживать значения `photo`, `video`, `document`, `audio` и возвращать ошибку `ErrValidation` для иных значений.

### Group 3 — Жизненный цикл статусов

**REQ-3.1** WHEN модуль `core` определяет константы статусов, the system SHALL объявить значения `idea`, `draft`, `ready`, `scheduled`, `published`, `archived`, `failed` (7 статусов).

**REQ-3.2** WHEN сервис вызывает `IsStatusTransitionValid(from, to)`, the system SHALL разрешить переходы согласно таблице: `idea→draft`, `draft→ready`, `ready→scheduled`, `ready→published`, `scheduled→published`, `scheduled→failed`, `scheduled→ready`, `failed→ready`, `failed→archived`, `published→archived` и запретить все остальные.

**REQ-3.3** WHEN сервис вызывает `UpdateStatus(ctx, id, newStatus)` и переход не разрешён согласно `AllowedTransitions`, the system SHALL вернуть ошибку `ErrInvalidTransition` (новая ошибка, отличная от `ErrInvalidStatus`).

**REQ-3.4** WHEN сервис переводит пост в `published`, the system SHALL установить `PublishedAt = Clock.Now()`, если поле было `nil`.

**REQ-3.5** WHEN сервис переводит пост из `scheduled` в `failed`, the system SHALL добавить запись в `PublishHistory` с `Status="failed"` и непустым полем `Error`.

### Group 4 — `PostFilter` и операции репозитория

**REQ-4.1** WHEN репозиторий вызывает `List(ctx, filter)`, the system SHALL фильтровать результаты по `filter.TenantID` (обязательное поле; нулевое значение запрещено и приводит к ошибке `ErrValidation`).

**REQ-4.2** WHEN репозиторий вызывает `List(ctx, filter)` с непустым `filter.AuthorID`, the system SHALL вернуть только посты с совпадающим `AuthorID`.

**REQ-4.3** WHEN `filter.SortBy` равно `"created_at"`, `"updated_at"`, `"deadline"`, `"scheduled_at"`, `"title"`, или `"status"`, the system SHALL отсортировать результаты по указанному полю с порядком `filter.SortOrder` (`asc` или `desc`, default — `asc`).

**REQ-4.4** WHEN `filter.SortBy` имеет иное значение, the system SHALL вернуть ошибку `ErrValidation`.

**REQ-4.5** WHEN `filter.Limit > 0`, the system SHALL ограничить количество возвращённых постов значением `Limit`; WHEN `filter.Offset > 0`, the system SHALL пропустить первые `Offset` записей.

**REQ-4.6** WHEN репозиторий вызывает `GetByID(ctx, id)` и пост существует, но его `TenantID` отличается от `tenant_id` в контексте (см. REQ-7.2), the system SHALL вернуть ошибку `ErrNotFound` (а не `ErrTenantMismatch`, чтобы не утекать факт существования).

### Group 5 — Конфигурация: схема

**REQ-5.1** WHEN загружается `.jtpost.yaml`, the system SHALL принимать секцию `storage` со структурой `{type string, git GitStorageConfig, sqlite SQLiteConfig, postgres PostgresConfig}`, где `type` — одно из `fs`, `sqlite`, `postgres` (default `fs`).

**REQ-5.2** WHEN значение `storage.type` отсутствует в файле и в env, the system SHALL установить его равным `fs`.

**REQ-5.3** WHEN значение `storage.type` имеет иное значение, чем `fs`, `sqlite`, `postgres`, the system SHALL вернуть `ErrConfigInvalid` при `Validate()`.

**REQ-5.4** WHEN загружается конфиг, the system SHALL принимать секцию `storage.git` со структурой `{enabled bool, auto_commit bool, auto_push bool, remote string, branch string, commit_template string}` (все поля опциональны; default `enabled=false`, `auto_commit=true`, `auto_push=false`, `branch="main"`, `commit_template="chore: update post {{.Slug}}"`).

**REQ-5.5** WHEN загружается конфиг, the system SHALL принимать секцию `storage.postgres` со структурой `{dsn string, max_open_conns int, max_idle_conns int, conn_max_lifetime time.Duration}` (default `max_open_conns=10`, `max_idle_conns=5`, `conn_max_lifetime=30m`).

**REQ-5.6** WHEN загружается конфиг, the system SHALL принимать секцию `auth` со структурой `{type string, secret string, tenant_default uuid.UUID, author_default uuid.UUID, oauth OAuthConfig, token_ttl time.Duration}`, где `type` — одно из `none`, `basic`, `oauth`, `token` (default `none`).

**REQ-5.7** WHEN загружается конфиг, the system SHALL принимать секцию `auth.oauth` со структурой `{provider string, client_id string, client_secret string, redirect_url string}`. Все поля опциональны и используются только в F4.

**REQ-5.8** WHEN загружается конфиг, the system SHALL принимать секцию `worker` со структурой `{enabled bool, interval time.Duration, max_retries int, retry_backoff time.Duration}` (default `enabled=false`, `interval=1m`, `max_retries=3`, `retry_backoff=30s`).

**REQ-5.9** WHEN загружается конфиг, the system SHALL принимать секцию `server` со структурой `{addr string, port int, base_url string, read_timeout time.Duration, write_timeout time.Duration}` (default `addr="localhost"`, `port=8080`, `read_timeout=15s`, `write_timeout=15s`).

**REQ-5.10** WHEN env-переменная с префиксом `JTPOST_` соответствует ключу конфига (с заменой `.` на `_`), the system SHALL переопределить значение из YAML значением из env (приоритет env > yaml > defaults).

**REQ-5.11** WHEN загружается конфиг, the system SHALL сохранить поле `defaults.platforms []string` (deferred for future platform extensions; not removed in F1).

**REQ-5.12** WHEN `Validate()` вызывается, the system SHALL вернуть ошибку, если `auth.tenant_default` или `auth.author_default` равны `uuid.Nil`.

### Group 6 — `jtpost init`

**REQ-6.1** WHEN команда `jtpost init` запускается и файл по пути `--config` (default `.jtpost.yaml`) **не существует**, the system SHALL создать его с дефолтными значениями всех секций (REQ-5.x), сгенерированными `auth.tenant_default = uuid.NewV7()` и `auth.author_default = uuid.NewV7()`.

**REQ-6.2** WHEN команда `jtpost init` запускается и файл по пути `--config` **существует**, the system SHALL вывести в stdout строку `Config already exists at <path>. Overwrite? [y/N]:` и читать ответ из stdin.

**REQ-6.3** WHEN ответ на REQ-6.2 пустой или начинается не с `y` или `Y`, the system SHALL завершиться с кодом `0` без изменения файла, выводом `Aborted` в stderr.

**REQ-6.4** WHEN ответ начинается с `y` или `Y`, the system SHALL перезаписать файл с новыми сгенерированными UUIDv7 для `tenant_default` и `author_default`.

**REQ-6.5** WHEN запускается `jtpost init --force`, the system SHALL перезаписать существующий файл без интерактивного запроса.

**REQ-6.6** WHEN команда `jtpost init` создаёт `.jtpost.yaml`, the system SHALL также создать директории `content/posts/<tenant_short_id>/` и `templates/`, где `tenant_short_id` — первые 8 hex-символов `tenant_default`.

**REQ-6.7** WHEN сгенерированные `tenant_default` и `author_default` приводят к одинаковому `tenant_short_id`, the system SHALL перегенерировать UUID до получения уникального префикса (защита от теоретической коллизии).

### Group 7 — FS-репозиторий и tenant-подкаталоги

**REQ-7.1** WHEN FS-репозиторий вызывает `Create(ctx, post)`, the system SHALL записать файл по пути `<posts_dir>/<post.TenantShortID()>/<post.Slug>.md`, создав отсутствующий подкаталог.

**REQ-7.2** WHEN FS-репозиторий вызывает `List(ctx, filter)`, the system SHALL читать только файлы из `<posts_dir>/<filter.TenantShortID()>/*.md` (не сканировать чужие подкаталоги).

**REQ-7.3** WHEN FS-репозиторий вызывает `GetByID(ctx, id)` без указания `TenantID` через context, the system SHALL вернуть ошибку `ErrTenantMismatch` (требование наличия tenant scope).

**REQ-7.4** WHEN frontmatter поста сериализуется в YAML, the system SHALL включить поля `tenant_id`, `author_id`, `created_at`, `updated_at`, `revision` (всегда) и `excerpt`, `cover_image`, `attachments`, `publish_history`, `revision_sha` (если не пустые).

**REQ-7.5** WHEN frontmatter поста десериализуется, the system SHALL вернуть ошибку `ErrValidation`, если отсутствует любое из обязательных полей: `id`, `tenant_id`, `author_id`, `title`, `slug`, `status`, `created_at`, `updated_at`, `revision`.

**REQ-7.6** WHEN метод `Post.TenantShortID()` вызывается, the system SHALL вернуть первые 8 hex-символов строкового представления `TenantID` без дефисов.

### Group 8 — HTTP API

**REQ-8.1** WHEN HTTP-сервер `jtpost serve` запускается, the system SHALL зарегистрировать middleware `tenantFromConfig`, который читает `auth.tenant_default` и `auth.author_default` из конфига и кладёт оба значения в `context.Context` под ключами `TenantContextKey` и `AuthorContextKey`.

**REQ-8.2** WHEN HTTP-хендлер обрабатывает запрос к `/api/posts/*`, the system SHALL извлекать `TenantID` из контекста и передавать его в `PostFilter.TenantID` или сравнивать с `Post.TenantID` при `GetByID`.

**REQ-8.3** WHEN HTTP-хендлер сериализует пост в JSON (`jsonPost`), the system SHALL включить все обязательные и опциональные поля Post (см. REQ-1.x, REQ-2.x), используя теги `json:"..."` с `omitempty` для опциональных.

**REQ-8.4** WHEN HTTP-запрос `POST /api/posts` или `PATCH /api/posts/{id}` содержит поле `tenant_id` отличное от `TenantID` в контексте, the system SHALL вернуть HTTP 403 с телом `{"error": "tenant_mismatch"}`.

**REQ-8.5** WHEN HTTP-запрос `PATCH /api/posts/{id}` пытается изменить поле `tenant_id` существующего поста, the system SHALL вернуть HTTP 400 с телом `{"error": "tenant_id_immutable"}`.

### Group 9 — CLI-чистка и расширение

**REQ-9.1** WHEN команда `jtpost list --format json` запускается, the system SHALL вывести в stdout валидный JSON-массив объектов `jsonPost` (закрытие TODO в `internal/cli/list.go:66`).

**REQ-9.2** WHEN команда `jtpost list --format json` запускается и список пуст, the system SHALL вывести `[]` (а не `null`).

**REQ-9.3** WHEN сборка проекта проходит, the system SHALL не содержать функции `getService` в `internal/cli/root.go` (legacy-функция удаляется).

**REQ-9.4** WHEN команда `jtpost new` запускается, the system SHALL автоматически устанавливать `Post.TenantID = config.Auth.TenantDefault` и `Post.AuthorID = config.Auth.AuthorDefault`.

**REQ-9.5** WHEN команда `jtpost new --tenant <uuid>` или `--author <uuid>` запускается, the system SHALL переопределить соответствующее поле значением из флага (для будущих multi-tenant сценариев).

**REQ-9.6** WHEN значение `--tenant` или `--author` не парсится как UUID, the system SHALL завершиться с кодом `1` и сообщением `invalid UUID for --tenant/--author`.

### Group 10 — Тесты и обратная совместимость

**REQ-10.1** WHEN запускается `task test`, the system SHALL пройти все существующие тесты `internal/core/`, `internal/adapters/fsrepo/`, `internal/adapters/httpapi/`, `internal/cli/`, обновлённые под новые требования.

**REQ-10.2** WHEN существуют файлы `testdata/posts/*.md`, the system SHALL содержать в них новые обязательные поля frontmatter (`tenant_id`, `author_id`, `created_at`, `updated_at`, `revision`).

**REQ-10.3** WHEN запускается тест пакета `internal/adapters/config/`, the system SHALL покрывать как минимум: загрузку дефолтов, переопределение через env (`JTPOST_AUTH_TENANT_DEFAULT`, `JTPOST_STORAGE_TYPE` и др.), валидацию (`Validate`), невалидный `storage.type`, нулевые `tenant_default`/`author_default`.

## Topological Order

```
Group 1 (REQ-1.1 → 1.5)        Расширение Post (обязательные)
       ↓
Group 2 (REQ-2.1 → 2.7)        Расширение Post (опциональные) и новые типы
       ↓
Group 3 (REQ-3.1 → 3.5)        Статусы и AllowedTransitions
       ↓
Group 4 (REQ-4.1 → 4.6)        PostFilter, сортировка, пагинация
       ↓
Group 5 (REQ-5.1 → 5.12)       Конфигурация (новые секции)
       ↓
Group 6 (REQ-6.1 → 6.7)        jtpost init (зависит от Group 5)
       ↓
Group 7 (REQ-7.1 → 7.6)        FS-репозиторий (зависит от Groups 1, 2, 3, 4, 5)
       ↓
Group 8 (REQ-8.1 → 8.5)        HTTP API (зависит от Groups 1–7)
       ↓
Group 9 (REQ-9.1 → 9.6)        CLI (зависит от Groups 1, 5, 6, 7)
       ↓
Group 10 (REQ-10.1 → 10.3)     Тесты (последний шаг — финализирует все группы)
```

Reason: каждая последующая группа использует типы и контракты, определённые в предыдущей. Group 5 (Config) идёт перед Group 6 (init), потому что init записывает конфиг согласно новой схеме. Group 7 (FS) идёт после Groups 1–5, потому что сериализует расширенный `Post` согласно tenant-подкаталогам из Group 6.

## Conflict Priority

**Конфликт 1.** REQ-4.6 (возвращать `ErrNotFound` для чужого tenant'а) vs REQ-1.5 (возвращать `ErrTenantMismatch` при `Update` с другим `TenantID`).

**Resolution:** для операций чтения (`GetByID`, `List`) приоритет у REQ-4.6 — мы не утечкаем факт существования чужих постов. Для операций модификации (`Update`, `Delete`) приоритет у REQ-1.5 — клиент явно передал данные и должен получить точную ошибку.

**Конфликт 2.** REQ-2.5 (хранить только последние 10 PublishAttempt в FS-frontmatter) vs неявное ожидание полной истории через API.

**Resolution:** REQ-2.5 действует только в FS-режиме. При `storage.type=sqlite|postgres` (F2) полная история сохраняется в отдельной таблице `publish_attempts`. F1 фиксирует контракт «FS = последние 10», полную реализацию SQL-таблиц — F2.

## Open Design Questions

| Question | Why It Matters | Impacted Requirements |
|----------|---------------|----------------------|
| Где хранить `tenant_id`-контекст в HTTP — в `context.Context` через middleware или в `*http.Request.Header`? | Влияет на интерфейс репозиториев и тестируемость. | REQ-7.3, REQ-8.1, REQ-8.2 |
| `PublishHistory` — сериализовать `ResponsePayload` как inline-JSON в YAML или как base64-строку? | Влияет на читаемость frontmatter и размер файла. | REQ-2.4, REQ-7.4 |
| `AllowedTransitions` — публичная переменная (`var`) или функция (`func() map[...][]...`)? | Влияет на возможность мутации в тестах vs immutability. | REQ-3.2, REQ-3.3 |
| Поле `Revision` инкрементируется в сервисе (`UpdatePost`) или в репозитории (на уровне SQL/файла)? | Влияет на распределение ответственности и риск race conditions при concurrent updates. | REQ-1.4 |
| Для FS-режима — сохранять `Attachment.Path` как абсолютный или относительный путь? | Влияет на портативность поста между машинами. | REQ-2.3, REQ-7.4 |
| `defaults.platforms` сохранён как deprecated, но должен ли быть warning при загрузке конфига? | Влияет на UX миграции в будущем. | REQ-5.11 |

## Verification Commands

| Action   | Command                                                | Source         |
|----------|--------------------------------------------------------|----------------|
| Test     | `task test`                                            | `Taskfile.yml` |
| Race     | `task test:race`                                       | `Taskfile.yml` |
| Coverage | `task test:coverage`                                   | `Taskfile.yml` |
| Build    | `task build`                                           | `Taskfile.yml` |
| Lint     | `task lint`                                            | `Taskfile.yml` |
| Format   | `task fmt`                                             | `Taskfile.yml` |
| Vet      | `task vet`                                             | `Taskfile.yml` |
| Generate | (n/a — F1 не использует кодогенерацию; в F2 будет sqlc) | —              |
