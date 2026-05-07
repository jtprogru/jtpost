# Storage Parity — SQLite + Postgres (F2) — Requirements

**Status:** Draft
**Author:** Claude (Opus 4.7) + Mikhail Savin
**Date:** 2026-05-07
**Feature:** storage-parity-postgres (F2)
**Branch:** `feature/storage-parity-postgres`

## Overview

F2 закрывает разрыв между расширенной в F1 доменной моделью и реальными адаптерами хранения. SQLite-адаптер приводится к полному F1-контракту (все обязательные/опциональные поля Post, scope из context, расширенный PostFilter). Появляется новый Postgres-адаптер на pgx с тем же контрактом. Вводится единая миграционная система (goose, embed-FS, авто-применение при `Open()`), типизированная генерация запросов (sqlc, по диалекту), storage factory `internal/adapters/storage` и переключение runtime-backend через `cfg.Storage.Type ∈ {fs|sqlite|postgres}`. Optimistic locking реализуется на уровне репозитория через колонку `revision` и новую ошибку `core.ErrConflict`. Команда `jtpost migrate` переписывается под `--from`/`--to`, старый формат `--db` удаляется. Постгресс-интеграционные тесты гонятся через testcontainers-go на Linux-CI. Контракт-сьют `internal/adapters/repotest` гарантирует семантическую идентичность fs/sqlite/postgres.

## Glossary

| Term | Definition | Code Artifact |
|------|------------|---------------|
| `StorageFactory` | Конструктор репозиториев по `cfg.Storage.Type` | `internal/adapters/storage/factory.go` |
| `ContractSuite` | Параметризованный набор поведенческих тестов, гоняемый против любого `core.PostRepository` | `internal/adapters/repotest/contract.go` |
| `ErrConflict` | Доменная ошибка optimistic-lock конфликта при Update | `internal/core/errors.go` |
| `MigrationFS` | embed.FS с goose-миграциями, упакованный в бинарник | `internal/adapters/sqlite/migrations/`, `internal/adapters/postgres/migrations/` |
| `pgxpool.Pool` | Пул подключений Postgres из `jackc/pgx/v5` | `internal/adapters/postgres/repository.go` |
| `JSONColumn` | Сериализованный JSON в одной колонке (`jsonb` в Postgres, `TEXT` в SQLite) для `attachments` и `publish_history` | `internal/adapters/{sqlite,postgres}/repository.go` |
| `PostgresContainer` | Тестовый контейнер Postgres 16 через `testcontainers-go/modules/postgres` | `internal/adapters/postgres/integration_test.go` |
| `IntegrationBuildTag` | Build-tag `integration` для тестов, требующих Docker | `//go:build integration` |

## User Stories

- Как **владелец канала**, я хочу выбирать backend хранения через `storage.type` в `.jtpost.yaml`, чтобы один и тот же бинарник работал и в single-user режиме (FS/SQLite), и в team-режиме (Postgres).
- Как **разработчик-сопровождающий**, я хочу единый поведенческий контракт всех адаптеров, чтобы не приходилось чинить баг отдельно в каждом backend.
- Как **API-консьюмер**, я хочу детерминированный порядок и пагинацию `List` в любом backend, чтобы клиентский код не ветвился по storage.type.
- Как **владелец канала**, я хочу команду `jtpost migrate --from fs --to postgres`, чтобы переехать с FS на Postgres без ручного экспорта/импорта.
- Как **разработчик**, я хочу автоматическое применение миграций при `Open()` адаптера и отдельную команду `jtpost migrate db` для ручного контроля, чтобы не править схему руками в проде.
- Как **владелец канала**, я хочу, чтобы конкурентное обновление поста не теряло чужие изменения молча, чтобы получать понятную ошибку при гонке.

## Requirements

### Group 1 — SQLite адаптер: F1-парность

**REQ-1.1** WHEN репозиторий `internal/adapters/sqlite` вызывает `Create(ctx, post)`, the system SHALL записать в БД все обязательные поля Post (`id`, `tenant_id`, `author_id`, `title`, `slug`, `status`, `created_at`, `updated_at`, `revision`) и не перезаписывать значения `created_at`, `updated_at`, `revision`, переданные вызывающим кодом.

**REQ-1.2** WHEN репозиторий `internal/adapters/sqlite` вызывает `Create(ctx, post)`, the system SHALL сериализовать `attachments` и `publish_history` как JSON-строки в колонки `attachments_json` и `publish_history_json` (тип `TEXT`), `cover_image` — в колонку `cover_image_json` (`TEXT`), `excerpt` — в колонку `excerpt` (`TEXT`, NULLable), `revision_sha` — в колонку `revision_sha` (`TEXT`, NULLable).

**REQ-1.3** WHEN репозиторий `internal/adapters/sqlite` вызывает `GetByID(ctx, id)` или `GetBySlug(ctx, slug)` и в `ctx` отсутствует `TenantID` (через `core.TenantFromContext`), the system SHALL вернуть ошибку `core.ErrTenantMismatch`.

**REQ-1.4** WHEN репозиторий `internal/adapters/sqlite` вызывает `GetByID(ctx, id)` и пост существует, но его `tenant_id` отличается от значения из контекста, the system SHALL вернуть ошибку `core.ErrNotFound`.

**REQ-1.5** WHEN репозиторий `internal/adapters/sqlite` вызывает `List(ctx, filter)` и `filter.TenantID == uuid.Nil`, the system SHALL вернуть ошибку `core.ErrValidation`.

**REQ-1.6** WHEN репозиторий `internal/adapters/sqlite` вызывает `List(ctx, filter)` с непустыми `filter.TenantID`, the system SHALL фильтровать результаты `WHERE tenant_id = ?`, и при непустом `filter.AuthorID` — дополнительно `AND author_id = ?`.

**REQ-1.7** WHEN репозиторий `internal/adapters/sqlite` вызывает `List(ctx, filter)` с непустым `filter.SortBy`, the system SHALL применить `ORDER BY <column> <SortOrder>`, где `column ∈ {created_at, updated_at, deadline, scheduled_at, title, status}` и `SortOrder ∈ {asc, desc}`; при отсутствии `SortBy` — `ORDER BY created_at DESC`.

**REQ-1.8** WHEN репозиторий `internal/adapters/sqlite` вызывает `List(ctx, filter)` и `filter.SortBy` имеет иное значение, чем разрешённые, the system SHALL вернуть ошибку `core.ErrValidation`.

**REQ-1.9** WHEN репозиторий `internal/adapters/sqlite` вызывает `List(ctx, filter)` и `filter.Limit > 0`, the system SHALL применить `LIMIT ?`; при `filter.Offset > 0` — дополнительно `OFFSET ?`.

**REQ-1.10** WHEN репозиторий `internal/adapters/sqlite` вызывает `Delete(ctx, id)` и `tenant_id` поста не совпадает с `TenantID` из контекста, the system SHALL вернуть ошибку `core.ErrNotFound` и не удалять запись.

**REQ-1.11** WHEN репозиторий `internal/adapters/sqlite` десериализует пост из строки БД, the system SHALL заполнить все поля Post из соответствующих колонок и распарсить JSON-колонки `attachments_json`, `publish_history_json`, `cover_image_json` в типизированные значения; при ошибке парсинга JSON — вернуть `core.ErrValidation`.

### Group 2 — Postgres адаптер

**REQ-2.1** WHEN бинарник запускается с `cfg.Storage.Type = "postgres"`, the system SHALL открыть пул подключений `pgxpool.Pool` к Postgres по `cfg.Storage.Postgres.DSN` с применённой настройкой `MaxConns = MaxOpenConns`, `MinConns = MaxIdleConns`, `MaxConnLifetime = ConnMaxLifetime`.

**REQ-2.2** WHEN репозиторий `internal/adapters/postgres` вызывает `Create/GetByID/GetBySlug/List/Update/Delete`, the system SHALL соблюдать тот же поведенческий контракт, что и SQLite-адаптер (REQ-1.3 .. REQ-1.10), включая возврат `core.ErrTenantMismatch`/`core.ErrNotFound`/`core.ErrValidation` в идентичных условиях.

**REQ-2.3** WHEN репозиторий `internal/adapters/postgres` сериализует пост, the system SHALL хранить `tenant_id` и `author_id` в колонках типа `uuid`, `created_at`/`updated_at`/`deadline`/`scheduled_at`/`published_at` — в колонках типа `timestamptz`, `attachments`/`publish_history`/`cover_image` — в колонках типа `jsonb`.

**REQ-2.4** WHEN репозиторий `internal/adapters/postgres` вызывает `Open()` и в `cfg.Storage.Postgres.DSN` указан невалидный/недоступный адрес, the system SHALL вернуть ошибку, обёрнутую через `errors.Join(core.ErrConfigInvalid, <pgx-error>)`, и не создавать пул.

**REQ-2.5** WHEN репозиторий `internal/adapters/postgres` вызывает `Close()`, the system SHALL закрыть `pgxpool.Pool` и освободить все подключения.

### Group 3 — Миграции (goose)

**REQ-3.1** WHEN бинарник скомпилирован, the system SHALL содержать встроенные через `embed.FS` файлы миграций по путям `internal/adapters/sqlite/migrations/*.sql` и `internal/adapters/postgres/migrations/*.sql`.

**REQ-3.2** WHEN репозиторий `internal/adapters/sqlite` или `internal/adapters/postgres` вызывает `Open(cfg)`, the system SHALL применить все pending-миграции через `goose.Up` против встроенной `MigrationFS` до возврата готового репозитория.

**REQ-3.3** WHEN миграция падает с ошибкой при `Open()`, the system SHALL вернуть ошибку, обёрнутую `errors.Join(core.ErrMigrationFailed, <goose-error>)`, и не возвращать частично сконфигурированный репозиторий.

**REQ-3.4** WHEN команда `jtpost migrate db --to <sqlite|postgres>` запускается, the system SHALL применить миграции выбранного диалекта (без открытия PostRepository) и вывести в stdout текущую версию схемы после применения.

**REQ-3.5** WHEN команда `jtpost migrate db status --to <sqlite|postgres>` запускается, the system SHALL вывести список применённых и pending-миграций без изменения схемы.

**REQ-3.6** WHEN первая (initial) миграция SQLite применяется к существующей БД с устаревшей F0-схемой, the system SHALL пересоздать таблицу `posts` под F1-схему. (Допустимо потому, что F1 M1 фиксирует отсутствие prod-данных.)

### Group 4 — Storage factory

**REQ-4.1** WHEN пакет `internal/adapters/storage` экспортирует функцию `Open(cfg *config.Config) (core.PostRepository, io.Closer, error)`, the system SHALL возвращать конкретный репозиторий по значению `cfg.Storage.Type`: `"fs"` → `fsrepo.NewFileSystemRepository(cfg.PostsDir)`, `"sqlite"` → `sqlite.NewSQLitePostRepository(...)`, `"postgres"` → `postgres.NewPostgresRepository(...)`.

**REQ-4.2** WHEN `storage.Open` вызывается с `cfg.Storage.Type = ""`, the system SHALL обработать значение как `"fs"` (default).

**REQ-4.3** WHEN `storage.Open` вызывается с любым иным значением `cfg.Storage.Type`, the system SHALL вернуть ошибку `core.ErrConfigInvalid` (без открытия соединений).

**REQ-4.4** WHEN `storage.Open` вызывается с `cfg.Storage.Type = "sqlite"` и пустым `cfg.Storage.SQLite.DSN`, the system SHALL вернуть ошибку `errors.Join(core.ErrConfigInvalid, errors.New("sqlite dsn required"))`.

**REQ-4.5** WHEN `storage.Open` вызывается с `cfg.Storage.Type = "postgres"` и пустым `cfg.Storage.Postgres.DSN`, the system SHALL вернуть ошибку `errors.Join(core.ErrConfigInvalid, errors.New("postgres dsn required"))`.

**REQ-4.6** WHEN CLI-команды (`new`, `list`, `edit`, `delete`, `plan`, `next`, `import`, `migrate_ids`) и HTTP-сервер `serve` инициализируются, the system SHALL получать репозиторий через `storage.Open(cfg)` (а не через прямой вызов `fsrepo.NewFileSystemRepository`).

**REQ-4.7** WHEN репозиторий, возвращённый `storage.Open`, имеет тип, реализующий `core.TransactionalRepository`, the system SHALL предоставлять рабочий метод `BeginTx` (для SQLite и Postgres). Для FS-адаптера интерфейс `TransactionalRepository` не обязателен.

### Group 5 — Optimistic locking

**REQ-5.1** WHEN модуль `core` определяет ошибки, the system SHALL экспортировать новую `ErrConflict = errors.New("conflict")` в `internal/core/errors.go`.

**REQ-5.2** WHEN SQL-адаптер (sqlite или postgres) вызывает `Update(ctx, post)`, the system SHALL выполнить `UPDATE posts SET ... WHERE id = ? AND revision = ?` с `revision = post.Revision` и проверить количество затронутых строк.

**REQ-5.3** WHEN `UPDATE` из REQ-5.2 затрагивает 0 строк и пост с таким `id` существует, the system SHALL вернуть ошибку `core.ErrConflict` (optimistic-lock collision).

**REQ-5.4** WHEN `UPDATE` из REQ-5.2 затрагивает 0 строк и пост с таким `id` не существует в данном tenant'е, the system SHALL вернуть ошибку `core.ErrNotFound`.

**REQ-5.5** WHEN сервис `core.PostService.UpdatePost(ctx, post)` вызывается, the system SHALL передавать в репозиторий `post` с уже инкрементированным `Revision` (как в F1 REQ-1.4); SQL-адаптер использует значение `post.Revision - 1` в `WHERE`-условии для проверки конфликта.

**REQ-5.6** WHEN FS-репозиторий вызывает `Update(ctx, post)`, the system SHALL писать файл атомарно (`os.WriteFile` через temp+rename) и не возвращать `ErrConflict` (FS не поддерживает optimistic lock в F2).

### Group 6 — Команда `jtpost migrate` (data migration)

**REQ-6.1** WHEN команда `jtpost migrate --from <fs|sqlite|postgres> --to <fs|sqlite|postgres>` запускается, the system SHALL открыть оба репозитория через `storage.Open` (с временно подменённой `cfg.Storage.Type` для каждого), вычитать все посты из source и записать их в target через `core.MigratableRepository.ImportPosts`.

**REQ-6.2** WHEN команда `jtpost migrate` запускается без `--from` или `--to`, the system SHALL вывести ошибку с примером использования и завершиться с кодом `1`.

**REQ-6.3** WHEN команда `jtpost migrate --from X --to X` запускается (одинаковые backend'ы), the system SHALL вывести ошибку `source and target must differ` и завершиться с кодом `1`.

**REQ-6.4** WHEN команда `jtpost migrate --dry-run --from X --to Y` запускается, the system SHALL вывести в stdout список постов, которые были бы перенесены, без записи в target.

**REQ-6.5** WHEN команда `jtpost migrate` обрабатывает старый флаг `--db <path>`, the system SHALL вернуть ошибку `flag --db is no longer supported, use --from/--to and storage.sqlite.dsn in config` и завершиться с кодом `2`. (Greenfield: alias не поддерживается.)

**REQ-6.6** WHEN команда `jtpost migrate` запускается и target backend уже содержит посты, the system SHALL вернуть ошибку, если не передан флаг `--overwrite`.

### Group 7 — `jtpost doctor` расширение

**REQ-7.1** WHEN команда `jtpost doctor` запускается с `cfg.Storage.Type = "sqlite"`, the system SHALL заменить существующую SQLite-проверку на универсальный `Storage` check, выполнив `Open` и `Count` против sqlite-backend и сообщив `OK`/`FAIL` с количеством постов.

**REQ-7.2** WHEN команда `jtpost doctor` запускается с `cfg.Storage.Type = "postgres"`, the system SHALL выполнить `Open` против Postgres и `SELECT 1` ping-запрос, замаскировав в выводе пароль из DSN (через regex `://<user>:***@`).

**REQ-7.3** WHEN команда `jtpost doctor` запускается с `cfg.Storage.Type = "fs"`, the system SHALL проверить существование `cfg.PostsDir` и наличие подкаталога `<tenant_short_id>/`, как в F1.

### Group 8 — Контракт-сьют и тесты

**REQ-8.1** WHEN пакет `internal/adapters/repotest` существует, the system SHALL экспортировать функцию `RunContract(t *testing.T, factory func(t *testing.T) core.PostRepository)`, которая запускает не менее 15 поведенческих сценариев (Create+GetByID happy/tenant-mismatch, GetBySlug, List filter+sort+limit+offset, Update+revision-conflict для SQL, Delete tenant-isolation, etc.).

**REQ-8.2** WHEN тесты `internal/adapters/fsrepo`, `internal/adapters/sqlite`, `internal/adapters/postgres` запускаются, the system SHALL вызывать `repotest.RunContract` со своей factory, обеспечивая идентичный поведенческий контракт.

**REQ-8.3** WHEN тесты `internal/adapters/postgres/*_test.go` объявлены под build-tag `integration`, the system SHALL пропускать их при `task test` (без `-tags=integration`) и запускать их через `task test:integration`.

**REQ-8.4** WHEN integration-тесты Postgres запускаются и Docker недоступен, the system SHALL завершиться с `t.Skip("docker not available")` без падения сьюта.

**REQ-8.5** WHEN CI gh-actions workflow запускается, the system SHALL выполнить `task test:integration` только на ubuntu-latest runner; на macos-runner job-step `Skip integration` пропускает их.

### Group 9 — Конфиг: дополнительная валидация

**REQ-9.1** WHEN `Config.Validate()` вызывается с `Storage.Type = "sqlite"` и `Storage.SQLite.DSN == ""`, the system SHALL вернуть ошибку `errors.Join(core.ErrConfigInvalid, errors.New("storage.sqlite.dsn required"))`.

**REQ-9.2** WHEN `Config.Validate()` вызывается с `Storage.Type = "postgres"` и `Storage.Postgres.DSN == ""`, the system SHALL вернуть ошибку `errors.Join(core.ErrConfigInvalid, errors.New("storage.postgres.dsn required"))`.

**REQ-9.3** WHEN `Config.Validate()` вызывается с `Storage.Postgres.MaxOpenConns < 0` или `Storage.Postgres.MaxIdleConns < 0`, the system SHALL вернуть ошибку `core.ErrConfigInvalid`.

**REQ-9.4** WHEN модуль `internal/core/errors.go` экспортирует `ErrMigrationFailed`, the system SHALL предоставлять её для оборачивания ошибок применения миграций (используется в REQ-3.3).

### Group 10 — sqlc и тулинг

**REQ-10.1** WHEN репозиторий проекта собирается, the system SHALL содержать файл `sqlc.yaml` в корне с двумя package-секциями: `sqlite` и `postgres`, каждая со своей `engine`, `queries`, `schema`, `out`.

**REQ-10.2** WHEN команда `task generate` запускается, the system SHALL вызывать `sqlc generate` и обновлять `internal/adapters/sqlite/queries/*.sql.go` и `internal/adapters/postgres/queries/*.sql.go`.

**REQ-10.3** WHEN сгенерированный sqlc-код добавляется в репозиторий, the system SHALL коммитить `*.sql.go` файлы (no-generate-on-build), а в CI выполнять `task generate && git diff --exit-code` для проверки актуальности.

## Topological Order

```
Group 9 (REQ-9.x)         Конфиг-валидация (минимальный фундамент)
       ↓
Group 5 (REQ-5.1, 5.6)    core.ErrConflict, ErrMigrationFailed (новые ошибки)
       ↓
Group 10 (REQ-10.x)        sqlc.yaml + task generate
       ↓
Group 3 (REQ-3.1 → 3.6)    goose-миграции SQLite/Postgres
       ↓
Group 1 (REQ-1.1 → 1.11)   SQLite адаптер на новой схеме
       ↓
Group 5 (REQ-5.2 → 5.5)    Optimistic lock в SQL (после Group 1, до Group 2)
       ↓
Group 2 (REQ-2.1 → 2.5)    Postgres адаптер
       ↓
Group 4 (REQ-4.1 → 4.7)    Storage factory (после Groups 1, 2)
       ↓
Group 8 (REQ-8.1 → 8.5)    Контракт-сьют + integration tests
       ↓
Group 6 (REQ-6.1 → 6.6)    jtpost migrate v2 (зависит от Group 4)
       ↓
Group 7 (REQ-7.1 → 7.3)    jtpost doctor v2 (последний потребитель Group 4)
```

Reason: новые ошибки и валидация конфига нужны всем последующим группам. Миграции (Group 3) предшествуют адаптерам (Groups 1, 2), потому что `Open()` адаптера их применяет. Storage factory (Group 4) собирается после готовности обоих SQL-адаптеров. Контракт-сьют (Group 8) использует все три адаптера. Команды `migrate` и `doctor` — финальные потребители factory.

## Conflict Priority

**Конфликт 1.** REQ-3.6 (initial-миграция SQLite пересоздаёт таблицу `posts`) vs неявное ожидание сохранности данных в существующих dev-БД.

**Resolution:** REQ-3.6 действует благодаря F1-M1 (отсутствие prod-данных). Локальные dev-БД пользователей теряются при первом `Open()`; этот эффект документируется в CHANGELOG как breaking change. Реальные данные мигрируются через `jtpost migrate --from sqlite --to <X>` до апгрейда (если они были).

**Конфликт 2.** REQ-3.2 (auto-`goose up` при `Open`) vs REQ-3.4 (отдельная команда `jtpost migrate db`).

**Resolution:** оба контракта существуют параллельно. Авто-применение — для удобства разработчика и single-user сценариев; ручная команда — для DBA в prod-сценариях, где автомиграция нежелательна. В будущем может появиться флаг `--no-auto-migrate`, но не в F2.

**Конфликт 3.** REQ-5.6 (FS не возвращает `ErrConflict`) vs REQ-8.1 (контракт-сьют гоняется по всем адаптерам).

**Resolution:** контракт-сьют делит сценарии на «общие» и «SQL-only». Тесты optimistic lock включены в SQL-only группу, FS их пропускает через явный признак capability в factory-параметре. Это не нарушает поведенческой парности — FS физически не может одновременно открыть один файл двумя процессами без дополнительной координации, что выходит за scope F2.

## Open Design Questions

| Question | Why It Matters | Impacted Requirements |
|----------|---------------|----------------------|
| Структура `posts`-таблицы в обоих диалектах: единый список колонок или диалект-специфичные расширения (`jsonb` vs `TEXT`)? | Влияет на схему `sqlc.yaml` и контент initial-миграций. | REQ-1.2, REQ-2.3, REQ-3.1 |
| Как именно sqlc обрабатывает разные диалекты — два пакета или один с условной компиляцией? | Влияет на структуру `internal/adapters/{sqlite,postgres}/queries/` и `sqlc.yaml`. | REQ-10.1, REQ-10.2 |
| Сигнатура `RunContract`: принимать `factory func() PostRepository` или `factory func() (PostRepository, cleanup)`? Как тестировать optimistic lock в SQL без захардкоженного `BeginTx`? | Влияет на API контракт-сьюта и его расширяемость. | REQ-8.1, REQ-8.2 |
| Транзакции в Postgres адаптере — через `BeginTx` (database/sql-style) или native pgx `pool.BeginTx`? Влияет на возможность общего интерфейса с SQLite. | Влияет на реализацию `core.TransactionalRepository`. | REQ-4.7 |
| Маскирование пароля в DSN для `doctor`: regex или парсинг через `pgx.ParseConfig`? | Влияет на корректность маскирования при разных форматах DSN (postgres://, postgresql://, key=value). | REQ-7.2 |
| Где хранится goose-таблица версионирования (`goose_db_version`) — в одной schema с `posts` или в отдельной? В Postgres — `public` или `jtpost`-namespaced? | Влияет на dev-инструменты и возможные коллизии при шаринге БД. | REQ-3.1, REQ-3.4 |
| Должен ли `storage.Open` запускать `pgxpool.Pool.Ping(ctx)` синхронно для fail-fast при недоступной БД, или возвращать пул и проверять connection лениво? | Влияет на стартап-латентность и UX `jtpost serve`. | REQ-2.1, REQ-2.4 |
| Имя контейнерного образа Postgres в integration-тестах: `postgres:16` или `postgres:16-alpine`? Зафиксировать tag для воспроизводимости CI. | Влияет на скорость pull в CI и стабильность тестов. | REQ-8.3, REQ-8.5 |

## Verification Commands

| Action               | Command                                              | Source         |
|----------------------|------------------------------------------------------|----------------|
| Test (unit)          | `task test`                                          | `Taskfile.yml` |
| Test (race)          | `task test:race`                                     | `Taskfile.yml` |
| Test (integration)   | `task test:integration`                              | `Taskfile.yml` (новое в F2) |
| Coverage             | `task test:coverage`                                 | `Taskfile.yml` |
| Build                | `task build`                                         | `Taskfile.yml` |
| Lint                 | `task lint`                                          | `Taskfile.yml` |
| Format               | `task fmt`                                           | `Taskfile.yml` |
| Vet                  | `task vet`                                           | `Taskfile.yml` |
| Generate (sqlc)      | `task generate`                                      | `Taskfile.yml` (новое в F2) |
| Migrate up (dev)     | `task db:up -- --to sqlite` (или `postgres`)         | `Taskfile.yml` (новое в F2) |
| Migrate status (dev) | `task db:status -- --to sqlite` (или `postgres`)     | `Taskfile.yml` (новое в F2) |
