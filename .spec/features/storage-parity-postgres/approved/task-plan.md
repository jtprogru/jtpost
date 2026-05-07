# Storage Parity — SQLite + Postgres (F2) — Task Plan

**Test Style Source:** Tier 2
- Reference test files: `internal/adapters/sqlite/repository_test.go`, `internal/adapters/fsrepo/repository_test.go`, `internal/core/post_test.go`, `internal/adapters/config/config_test.go`.
- Key patterns: native `testing` package (no testify), table-driven через `tt := []struct{...}{...}` slices, `t.Run(tt.name, ...)`, `t.TempDir()` для эфемерных файлов, `t.Cleanup` для teardown, helper-функции локально в `*_test.go` (например `mustParsePostID`), фикстуры в `testdata/` для frontmatter-тестов.
- PBT note: PBT-библиотек в проекте нет. Substitute: targeted unit tests с явным cartesian product (используется `tt := []struct{...}` с >5 кейсов на propery-тест).

**Commands:**

| Action               | Command                          | Source         |
|----------------------|----------------------------------|----------------|
| Test (unit)          | `task test`                      | `Taskfile.yml` |
| Test (race)          | `task test:race`                 | `Taskfile.yml` |
| Test (integration)   | `task test:integration`          | `Taskfile.yml` (новое в F2) |
| Build                | `task build`                     | `Taskfile.yml` |
| Lint                 | `task lint`                      | `Taskfile.yml` |
| Format               | `task fmt`                       | `Taskfile.yml` |
| Vet                  | `task vet`                       | `Taskfile.yml` |
| Generate             | `task generate`                  | `Taskfile.yml` (новое в F2) |
| Migrate up (dev)     | `task db:up -- --to sqlite`      | `Taskfile.yml` (новое в F2) |

---

## Coverage Matrix

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1     | T-3     | CP-5 (round-trip) |
| REQ-1.2     | T-3     | CP-5 (round-trip) |
| REQ-1.3     | T-3     | CP-1 (exclusion) |
| REQ-1.4     | T-3     | CP-1 (exclusion) |
| REQ-1.5     | T-3     | CP-2 (equivalence) |
| REQ-1.6     | T-3     | CP-2 (equivalence) |
| REQ-1.7     | T-3     | CP-2 (equivalence) |
| REQ-1.8     | T-3     | CP-3 (absence) |
| REQ-1.9     | T-3     | CP-2 (equivalence) |
| REQ-1.10    | T-3     | CP-1 (exclusion) |
| REQ-1.11    | T-3     | CP-5 (round-trip) |
| REQ-2.1     | T-4     | CP-9, CP-16 |
| REQ-2.2     | T-4, T-6 | CP-1, CP-2, CP-5, CP-6 |
| REQ-2.3     | T-4     | CP-5 (round-trip) |
| REQ-2.4     | T-4     | CP-9 (absence) |
| REQ-2.5     | T-4     | CP-16 (round-trip) |
| REQ-3.1     | T-2, T-3, T-4 | CP-7 (equivalence) |
| REQ-3.2     | T-3, T-4 | CP-7 (equivalence) |
| REQ-3.3     | T-1, T-3, T-4 | CP-15 (propagation) |
| REQ-3.4     | T-7     | CP-7 |
| REQ-3.5     | T-7     | CP-7 |
| REQ-3.6     | T-3     | CP-7 |
| REQ-4.1     | T-5     | CP-8 (propagation) |
| REQ-4.2     | T-5     | CP-8 |
| REQ-4.3     | T-5     | CP-8 |
| REQ-4.4     | T-5     | CP-9 |
| REQ-4.5     | T-5     | CP-9 |
| REQ-4.6     | T-5     | CP-8 |
| REQ-4.7     | T-5     | CP-8 |
| REQ-5.1     | T-1     | CP-4 |
| REQ-5.2     | T-3, T-4 | CP-4 |
| REQ-5.3     | T-3, T-4 | CP-4 |
| REQ-5.4     | T-3, T-4 | CP-4 |
| REQ-5.5     | T-3, T-4 | CP-4 |
| REQ-5.6     | T-6     | CP-13 |
| REQ-6.1     | T-7     | CP-11 |
| REQ-6.2     | T-7     | CP-11 |
| REQ-6.3     | T-7     | CP-11 |
| REQ-6.4     | T-7     | CP-11 |
| REQ-6.5     | T-7     | CP-10 |
| REQ-6.6     | T-7     | CP-11 |
| REQ-7.1     | T-7     | CP-12 |
| REQ-7.2     | T-7     | CP-12 |
| REQ-7.3     | T-7     | CP-12 |
| REQ-8.1     | T-6     | CP-13 |
| REQ-8.2     | T-6     | CP-13 |
| REQ-8.3     | T-6     | CP-14 |
| REQ-8.4     | T-6     | CP-14 |
| REQ-8.5     | T-6     | CP-14 |
| REQ-9.1     | T-1     | CP-9 |
| REQ-9.2     | T-1     | CP-9 |
| REQ-9.3     | T-1     | CP-9 |
| REQ-9.4     | T-1     | CP-15 |
| REQ-10.1    | T-2     | CP-7 |
| REQ-10.2    | T-2     | CP-7 |
| REQ-10.3    | T-2, T-8 | CP-14 |

Каждое требование покрыто ≥1 задачей; каждое CP-N привязано к ≥1 задаче.

---

## Work Type Classification

**Pure feature** (с migration-компонентом по SQLite-rewrite).

Большинство работы — greenfield: Postgres-адаптер, storage factory, contract-suite, sqlc/goose тулинг, новые CLI-команды (`migrate db`, `migrate --from --to`). Перезапись SQLite — migration-стиль внутри одного top-level T-3, с baseline-тестами для проверки совместимости поведенческого контракта (через `repotest.RunContract`).

Task order: GREEN (test stubs) → CODE (bottom-up по слоям) → GREEN (full tests) → GATE.

---

## T-1 — Foundation: новые ошибки и расширенная валидация конфига

***_Complexity: mechanical_***
***_Requirements: REQ-3.3, REQ-5.1, REQ-9.1, REQ-9.2, REQ-9.3, REQ-9.4_***
***_Preservation: CP-9 (DSN-required validation), CP-15 (migration error wrap)_***

GOAL: подготовить фундамент — ошибки `ErrConflict`/`ErrMigrationFailed` и DSN-валидации в `Config.Validate()` — до того как любой адаптер начнёт их использовать.

Subtasks:
- [ ] 1. **CODE** В `internal/core/errors.go` добавить `ErrConflict = errors.New("conflict")` и `ErrMigrationFailed = errors.New("migration failed")`. Запустить `task test`.
- [ ] 2. **GREEN** В `internal/core/errors_test.go` (создать если отсутствует) — table-driven тест с проверкой `errors.Is(ErrConflict, ErrConflict)` и не-равенство с `ErrNotFound`. Запустить `task test`.
- [ ] 3. **CODE** В `internal/adapters/config/config.go` функция `Validate()` — добавить:
  - При `c.Storage.Type == "sqlite"` и пустом `c.Storage.SQLite.DSN` (с fallback на `c.SQLite.DSN`) → `errors.Join(core.ErrConfigInvalid, errors.New("storage.sqlite.dsn required"))`.
  - При `c.Storage.Type == "postgres"` и пустом `c.Storage.Postgres.DSN` → `errors.Join(core.ErrConfigInvalid, errors.New("storage.postgres.dsn required"))`.
  - При `c.Storage.Postgres.MaxOpenConns < 0` или `c.Storage.Postgres.MaxIdleConns < 0` → `core.ErrConfigInvalid`.
- [ ] 4. **GREEN** В `internal/adapters/config/config_test.go` добавить table-driven тест `TestConfigValidate_StorageDSN`:
  - 6 кейсов: `{Type=fs, DSN=*}`, `{Type=sqlite, DSN=""}` → fail, `{Type=sqlite, DSN="x"}` → ok, `{Type=postgres, DSN=""}` → fail, `{Type=postgres, DSN="x"}` → ok, `{Type=postgres, MaxOpenConns=-1}` → fail.
  - Запустить `task test`.
- [ ] 5. **VERIFY** `task lint && task test`.

NOTE: Эта задача не требует RED-теста — это pure-feature группа, чисто аддитивные изменения.

---

## T-2 — SQL toolchain: sqlc + goose + Taskfile

***_Complexity: standard_***
***_Requirements: REQ-3.1, REQ-10.1, REQ-10.2, REQ-10.3_***
***_Preservation: CP-7 (migration idempotency), CP-14 (build-tag isolation)_***

GOAL: ввести в проект sqlc и goose, создать infrastructure для миграций и кодгена. После этой задачи проект собирается, но реальные миграции пустые.

Subtasks:
- [ ] 1. **CODE** В корне создать `sqlc.yaml` со структурой из design §2.5 (два пакета — `sqlite` и `postgresql`).
- [ ] 2. **CODE** В `Taskfile.yml` добавить задачу `generate` (`sqlc generate`), `test:integration` (`go test -tags=integration ./internal/adapters/postgres/...`), `db:up` и `db:status` (вызовы `goose -dir <dir> <up|status>` через `cmd/jtpost migrate db`).
- [ ] 3. **CODE** Создать пустые директории-скелеты с `.gitkeep`: `internal/adapters/sqlite/migrations/`, `internal/adapters/sqlite/queries/`, `internal/adapters/sqlite/sqlitedb/`, `internal/adapters/postgres/migrations/`, `internal/adapters/postgres/queries/`, `internal/adapters/postgres/pgdb/`. Закоммитить.
- [ ] 4. **CODE** В `go.mod` добавить через `go get`:
  - `github.com/jackc/pgx/v5` (для адаптера Postgres).
  - `github.com/pressly/goose/v3` (для миграций).
  - `github.com/testcontainers/testcontainers-go` и `github.com/testcontainers/testcontainers-go/modules/postgres` (для integration-тестов).
- [ ] 5. **VERIFY** `task build` (должен пройти даже с пустыми migration-директориями), `task lint`, `task test`. Запустить `task generate` — должна отработать без ошибок (no queries → no codegen).

NOTE: sqlc CLI должен быть установлен отдельно (`go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest`). Документировать в `CONTRIBUTING.md`.

---

## T-3 — SQLite v2: миграции, sqlc-запросы, переписанный репозиторий

***_Complexity: complex_***
***_Requirements: REQ-1.1 .. REQ-1.11, REQ-3.2, REQ-3.3, REQ-3.6, REQ-5.2, REQ-5.3, REQ-5.4, REQ-5.5_***
***_Preservation: CP-1 (tenant isolation), CP-2 (filter+sort+page), CP-3 (sort validation), CP-4 (optimistic lock), CP-5 (round-trip), CP-7 (migration idempotency), CP-15 (migration error wrap)_***

GOAL: привести SQLite-адаптер к F1-парности. После этой задачи `internal/adapters/sqlite` соответствует контракту fsrepo по поведению (тестируется в T-6 контракт-сьютом).

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/sqlite/migrations/0001_initial.sql` с goose-разметкой `-- +goose Up` / `-- +goose Down` по DDL из design §2.5 (включая `DROP TABLE IF EXISTS posts` в Up). Запустить `task build`.
- [ ] 2. **CODE** Создать `internal/adapters/sqlite/queries/posts.sql` со sqlc-аннотациями:
  - `-- name: CreatePost :exec`, `GetPostByID :one`, `GetPostBySlug :one`, `ListPosts :many` (с filter by tenant + author + status[]+ tags + search + sortBy/order + limit/offset через CASE-выражения), `UpdatePost :execrows` (с `WHERE id=? AND tenant_id=? AND revision=?`), `DeletePost :execrows` (`WHERE id=? AND tenant_id=?`), `ImportPost :exec` (UPSERT), `CountPosts :one`, `PostExistsByID :one`.
  - Запустить `task generate` — генерируется `internal/adapters/sqlite/sqlitedb/*.sql.go`.
- [ ] 3. **CODE** Переписать `internal/adapters/sqlite/repository.go`:
  - `NewSQLitePostRepository(cfg Config)` открывает `*sql.DB`, делает `PRAGMA foreign_keys=ON`, применяет миграции через `goose.Up` с `embed.FS` миграций (обёрнуто в `errors.Join(core.ErrMigrationFailed, ...)` при ошибке).
  - Все CRUD-методы — тонкие обёртки над `sqlitedb.Queries`. JSON-сериализация `Tags`, `Attachments`, `PublishHistory`, `CoverImage` в обёртке через `json.Marshal/Unmarshal` (ошибка decode → `errors.Join(core.ErrValidation, ...)`).
  - `GetByID/GetBySlug/Update/Delete/List` извлекают `tenant_id` из контекста через `core.TenantFromContext`; отсутствие → `core.ErrTenantMismatch`. `List` с `filter.TenantID == uuid.Nil` → `core.ErrValidation`. Невалидный `filter.SortBy` (через `core.IsValidSortKey`) → `core.ErrValidation` без обращения к БД.
  - `Update` смотрит `RowsAffected`: 0 → если post существует (через `PostExistsByID`) → `core.ErrConflict`, иначе → `core.ErrNotFound`.
  - `BeginTx` сохранён.
- [ ] 4. **CODE** Удалить устаревший `migrate()` метод, `scanPost`, `scanPostRow`, `joinStrings` из `repository.go`. Обновить `Close()` если нужно.
- [ ] 5. **GREEN** Переписать `internal/adapters/sqlite/repository_test.go`:
  - Хелпер `newRepo(t)` — `t.TempDir()` + `NewSQLitePostRepository`.
  - Хелпер `withTenant(ctx, post)` — `core.WithTenant(ctx, post.TenantID)`.
  - Тесты по дизайн §2.8 (CreateGetRoundtrip, TenantIsolation, List_FilterSortLimit, List_InvalidSort, Update_RevisionConflict, Update_NotFound, Migrate_Idempotent, JSONDecodeError). Все table-driven, 1 файл.
- [ ] 6. **VERIFY** `task generate && task lint && task test ./internal/adapters/sqlite/...`. Все тесты GREEN.

CRITICAL: Если `task generate` падает на sqlc-аннотациях, не править код адаптера до исправления queries — sqlc — source of truth.
DO NOT: Сохранять обратную совместимость с F0-схемой; M1 фиксирует отсутствие prod-данных.

---

## T-4 — Postgres-адаптер: миграции, sqlc-запросы, репозиторий, integration tests

***_Complexity: complex_***
***_Requirements: REQ-2.1, REQ-2.2, REQ-2.3, REQ-2.4, REQ-2.5, REQ-3.2, REQ-3.3, REQ-5.2..5.5_***
***_Preservation: CP-1, CP-2, CP-3, CP-4, CP-5, CP-7, CP-9, CP-15, CP-16_***

GOAL: создать `internal/adapters/postgres` с полной парностью SQLite-адаптеру.

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/postgres/migrations/0001_initial.sql` по DDL из design §2.5 (Postgres-вариант: `uuid`, `jsonb`, `timestamptz`).
- [ ] 2. **CODE** Создать `internal/adapters/postgres/queries/posts.sql` — те же имена что в SQLite, но с placeholders `$1..$N`, операторами `jsonb`, типами `pgtype.UUID`/`pgtype.Timestamptz`. Запустить `task generate` — генерируется `internal/adapters/postgres/pgdb/*.sql.go`.
- [ ] 3. **CODE** Создать `internal/adapters/postgres/repository.go`:
  - `Config{DSN, MaxOpenConns, MaxIdleConns, ConnMaxLifetime}`.
  - `NewPostgresRepository(ctx, cfg)`:
    - Парсит DSN через `pgxpool.ParseConfig`.
    - `MaxConns = cfg.MaxOpenConns`, `MinConns = cfg.MaxIdleConns`, `MaxConnLifetime = cfg.ConnMaxLifetime`.
    - Открывает пул; синхронно делает `pool.Ping(ctx)` (eager fail-fast).
    - При ошибке Ping/connect — `errors.Join(core.ErrConfigInvalid, err)`, пул закрывается.
    - Применяет goose-миграции через `goose.Up` поверх `*sql.DB` обёртки `pgxstd.OpenDBFromPool(pool)` (или прямой `sql.Open("pgx", dsn)` для миграций — выбрать второй вариант т.к. goose требует `*sql.DB`). При ошибке — `errors.Join(core.ErrMigrationFailed, err)`.
  - Все CRUD-методы — обёртки над `pgdb.Queries(pool)`; mapping `core.Post` ↔ pgdb-типы; JSON-маршалинг `attachments`/`publish_history`/`cover_image` в `[]byte` для `jsonb`-колонок.
  - Контракт ошибок идентичен SQLite (`ErrTenantMismatch`/`ErrNotFound`/`ErrValidation`/`ErrConflict`).
  - `Close()` закрывает пул.
- [ ] 4. **GREEN** Создать `internal/adapters/postgres/repository_test.go` под build-tag `integration`:
  - В начале файла: `//go:build integration` + пустая строка + `package postgres`.
  - Хелпер `newPostgresContainer(t)` — `tcpostgres.Run(ctx, "postgres:16-alpine", ...)` с `t.Cleanup`. Если container.Run возвращает ошибку, содержащую "Cannot connect to Docker" — `t.Skip("docker not available")`.
  - Хелпер `newRepo(t)` — берёт connection-string у container, вызывает `NewPostgresRepository`, `t.Cleanup(repo.Close)`.
  - Тесты: PostgresRepo_CreateGetRoundtrip, TenantIsolation, List_FilterSortLimit, List_InvalidSort, Update_RevisionConflict, Update_NotFound, PoolLifecycle (Close → method → error), PingFailFast (bad DSN → быстрая ошибка), MigrateIdempotent.
- [ ] 5. **VERIFY** `task generate && task lint && task build`. Запустить `task test` (без integration tag) — Postgres-тесты должны быть пропущены. Запустить `task test:integration` — все тесты GREEN на Linux/macOS с Docker. Если Docker нет → t.Skip без падения сьюта.

NOTE: Маппинг `core.Post.TenantID uuid.UUID` ↔ `pgtype.UUID` требует helper-функций. Положить в `internal/adapters/postgres/conv.go` (отдельный файл с конвертерами).

---

## T-5 — Storage factory: единая точка входа адаптеров + CLI rewire

***_Complexity: standard_***
***_Requirements: REQ-4.1, REQ-4.2, REQ-4.3, REQ-4.4, REQ-4.5, REQ-4.6, REQ-4.7_***
***_Preservation: CP-8 (factory dispatch), CP-9 (DSN-required), CP-1, CP-2 (tenant/filter в существующих CLI-командах)_***

GOAL: ввести `internal/adapters/storage` для runtime-выбора backend по `cfg.Storage.Type`; перевязать все CLI-команды и httpapi-сервер.

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/storage/factory.go`:
  - `Open(cfg *config.Config) (core.PostRepository, io.Closer, error)`.
  - `switch cfg.Storage.Type`: `""` → fs (default), `"fs"` → `fsrepo.NewFileSystemRepository(cfg.PostsDir)` (fs возвращает `nopCloser{}` как `io.Closer`).
  - `"sqlite"` → проверка DSN (с fallback `cfg.Storage.SQLite.DSN || cfg.SQLite.DSN`); если пусто → `errors.Join(core.ErrConfigInvalid, errors.New("storage.sqlite.dsn required"))`. Иначе `sqlite.NewSQLitePostRepository`.
  - `"postgres"` → проверка DSN; пусто → ошибка как выше. Иначе `postgres.NewPostgresRepository(context.Background(), postgres.Config{...})`.
  - default → `core.ErrConfigInvalid`.
- [ ] 2. **GREEN** Создать `internal/adapters/storage/factory_test.go`:
  - Table-driven тест `TestOpen_Dispatch` — 6 кейсов: fs/sqlite/postgres c валидным DSN (postgres-кейс пропускаем без integration-tag через `t.Skip`), пустой Type, "invalid", sqlite без DSN, postgres без DSN.
  - Для проверки type assertion использовать `_, ok := repo.(*fsrepo.FileSystemPostRepository)`, etc.
  - Запустить `task test`.
- [ ] 3. **CODE** В `internal/cli/root.go` добавить экспортируемый helper `openRepo(cfg)` — обёртка над `storage.Open`. Добавить нужные импорты.
- [ ] 4. **CODE** В каждом из файлов `internal/cli/{new,list,edit,delete,plan,next,import,migrate_ids}.go` заменить `fsrepo.NewFileSystemRepository(cfg.PostsDir)` на `openRepo(cfg)`. Обновить deferred close (`defer closer.Close()` где раньше был `_`-ignored). По одному файлу за subtask-внутри (одна правка ≈ один search-and-replace), запустить `task test ./internal/cli/...` после каждого файла.
- [ ] 5. **CODE** В `internal/adapters/httpapi/server.go` — где сейчас FS-репозиторий передаётся в конструктор `PostService`, переключить на `openRepo(cfg)` (либо принимать репо снаружи и оставить wiring в CLI — выбрать второй путь, так конструктор не меняется). Обновить cli-команду `serve` (если есть отдельный файл `internal/cli/serve.go`).
- [ ] 6. **VERIFY** `task lint && task build && task test`. Все CLI-тесты — GREEN (FS-режим работает как раньше).

NOTE: `nopCloser` для fs — внутренний тип в `storage`-пакете: `type nopCloser struct{}; func (nopCloser) Close() error { return nil }`.

---

## T-6 — Контракт-сьют + integration tests + CI workflow

***_Complexity: standard_***
***_Requirements: REQ-5.6, REQ-8.1, REQ-8.2, REQ-8.3, REQ-8.4, REQ-8.5, REQ-10.3_***
***_Preservation: CP-6 (backend equivalence), CP-13 (contract suite coverage), CP-14 (build-tag isolation)_***

GOAL: ввести `internal/adapters/repotest` — переиспользуемые поведенческие тесты. Гонять против всех трёх backend'ов. Подключить к CI.

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/repotest/contract.go`:
  - Тип `Capabilities{OptimisticLock bool; Transactions bool}`.
  - Тип `Factory func(t *testing.T) (core.PostRepository, Capabilities, func())`.
  - Функция `RunContract(t *testing.T, factory Factory)`.
  - Внутри 15+ named subtests через `t.Run`: tenant isolation (cross-tenant Get/Update/Delete), filter+sort+limit+offset (10 кейсов в table-driven), invalid sort key, GetByID/Slug not found, Create+Get round-trip с CoverImage/Attachments/PublishHistory, Update with valid revision, Update with stale revision (gated by `caps.OptimisticLock`), Delete tenant isolation, ImportPosts+Count, BeginTx+Commit (gated by `caps.Transactions`).
- [ ] 2. **CODE** В `internal/adapters/fsrepo/repository_test.go` оставить FS-специфичные тесты (frontmatter parsing, tenant подкаталоги в FS); основные сценарии заменить на `repotest.RunContract(t, fsFactory)` где `fsFactory` создаёт `FileSystemPostRepository` в `t.TempDir()` и возвращает `Capabilities{OptimisticLock: false, Transactions: false}`.
- [ ] 3. **CODE** В `internal/adapters/sqlite/repository_test.go` добавить `TestSQLite_RunContract` через `repotest.RunContract(t, sqliteFactory)` с `Capabilities{OptimisticLock: true, Transactions: true}`. Локальные тесты из T-3 могут остаться рядом или быть удалены если полностью покрыты контрактом.
- [ ] 4. **CODE** В `internal/adapters/postgres/repository_test.go` (build-tag integration) добавить `TestPostgres_RunContract` аналогично.
- [ ] 5. **CODE** В `.github/workflows/ci.yml`:
  - Существующая job `test` остаётся; убедиться что она НЕ передаёт `-tags=integration`.
  - Добавить новую job `integration-tests`: `runs-on: ubuntu-latest`, шаг setup-go, шаг `task test:integration`. macOS-runner — пропускается через `if: matrix.os == 'ubuntu-latest'` или separate job.
  - Добавить шаг `task generate && git diff --exit-code` в job `lint` или `test` для проверки актуальности sqlc-кода (REQ-10.3).
- [ ] 6. **VERIFY** `task test` (FS+SQLite RunContract — GREEN; Postgres skip из-за build-tag). `task test:integration` (все три RunContract — GREEN, требует Docker). `task lint`. Локально запустить `task generate && git diff --exit-code` — должен быть clean.

---

## T-7 — CLI: `migrate db`, `migrate --from --to`, `doctor` v2

***_Complexity: standard_***
***_Requirements: REQ-3.4, REQ-3.5, REQ-6.1..REQ-6.6, REQ-7.1, REQ-7.2, REQ-7.3_***
***_Preservation: CP-7 (migration idempotency), CP-10 (legacy flag), CP-11 (migrate symmetry), CP-12 (doctor coverage)_***

GOAL: пользовательские CLI-команды для управления схемой и данными.

Subtasks:
- [ ] 1. **CODE** Создать `internal/cli/migrate_db.go` — новая cobra-команда `jtpost migrate db` с подкомандами `up` и `status`:
  - `--to <sqlite|postgres>` (required).
  - Внутри: открывает соответствующий `*sql.DB` для нужного диалекта (для sqlite — `sql.Open("sqlite", cfg.Storage.SQLite.DSN)`; для postgres — `sql.Open("pgx", cfg.Storage.Postgres.DSN)`), вызывает `goose.Up`/`goose.Status` поверх embed.FS из соответствующего адаптера. Печатает результат в stdout.
- [ ] 2. **CODE** Полностью переписать `internal/cli/migrate.go`:
  - Удалить флаг `--db`. Вместо него — `--from <fs|sqlite|postgres>` и `--to <fs|sqlite|postgres>` (оба required через `MarkFlagRequired`).
  - Если в args/flags обнаружен старый `--db` — сразу `os.Exit(2)` с stderr-сообщением `flag --db is no longer supported, use --from/--to and storage.sqlite.dsn in config`.
  - Если `from == to` → exit 1 с `source and target must differ`.
  - Открывает оба репозитория через временно-подменённую `cfg.Storage.Type` (создать функцию `openRepoAs(cfg, storageType)` рядом с `openRepo` в root.go).
  - Использует `core.MigratableRepository.ImportPosts`. При `--dry-run` — печать списка постов без записи. Если target.Count > 0 без `--overwrite` → exit 1.
  - `--from` и `--to` не должны быть пустыми; `MarkFlagRequired` обработает.
- [ ] 3. **CODE** В `internal/cli/doctor.go` — заменить SQLite-проверку на универсальную `Storage` секцию:
  - При `cfg.Storage.Type == "fs"` — проверка существования `cfg.PostsDir` и подкаталога `<tenant_short_id>/`.
  - При `cfg.Storage.Type == "sqlite"` — `storage.Open(cfg)`, `repo.(core.MigratableRepository).Count(ctx)`. Сообщение: `OK: SQLite, N posts`.
  - При `cfg.Storage.Type == "postgres"` — `storage.Open(cfg)` (включая Ping), `Count`. В выводе DSN маскируется через `pgx.ParseConfig` + `cfg.ConnConfig.Password = ""`, потом форматируется обратно. Если DSN не парсится — fallback regex `://([^:]+):([^@]+)@` → `://$1:***@`.
- [ ] 4. **GREEN** Тесты:
  - `internal/cli/migrate_db_test.go` — sqlite+ tempdir сценарий: `migrate db up --to sqlite` → схема создана; `migrate db status` показывает applied.
  - `internal/cli/migrate_test.go` (обновить или создать) — кейсы `LegacyDBFlag`, `SameSourceTarget`, `FSToSQLite` (data round-trip), `TargetNotEmpty`, `DryRun`.
  - `internal/cli/doctor_test.go` (если есть) — добавить `Doctor_StorageFS`, `Doctor_StorageSQLite`, `Doctor_StoragePostgres_DSNMasking` (последний запускается с реальным `pgx.ParseConfig` без подключения, либо мокается через test helper — проверяется только маскирование).
- [ ] 5. **VERIFY** `task test ./internal/cli/...` — все GREEN. `task build`. `task lint`.

---

## T-8 — GATE: проверка полного покрытия

***_Complexity: mechanical_***
***_Requirements: ALL_***

CRITICAL: Эта задача — последняя в плане. Не выполнять до завершения T-1 .. T-7.

Instructions:
1. Запустить `task fmt && task vet && task lint`. Все checks — pass.
2. Запустить `task test` (unit suite). 100% GREEN.
3. Запустить `task test:race`. 100% GREEN, no data races.
4. Запустить `task test:integration` (требуется Docker). 100% GREEN. Если Docker нет — `t.Skip` срабатывает без fail.
5. Запустить `task generate && git diff --exit-code` — sqlc-код актуален.
6. Запустить `task build` — бинарник собирается.
7. Ручной smoke-test: создать `.jtpost.yaml` с `storage.type=sqlite`, запустить `jtpost init && jtpost new --title "smoke" && jtpost list --format json` — пост создан и виден.
8. Проверить coverage matrix: каждое REQ-X.Y из requirements.md имеет ≥1 GREEN-тест в `task test` или `task test:integration`.
9. Обновить `CHANGELOG.md` секцию `## [Unreleased]`: storage factory, новые backend'ы, breaking change `migrate --db`.
10. Обновить `.jtpost.example.yaml` с раскомментированными секциями `storage.postgres.dsn`, `storage.sqlite.dsn`.
11. Если хоть один шаг fail — вернуться к ответственной задаче, не закрывать GATE.
