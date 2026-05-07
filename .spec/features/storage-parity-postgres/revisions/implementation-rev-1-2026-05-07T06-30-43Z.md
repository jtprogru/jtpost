# Implementation Report: Storage Parity — SQLite + Postgres (F2)

## Summary

Реализация F2 ведётся инкрементально. На данный момент завершены **T-1** (foundation: новые ошибки + DSN-валидации в Config) и **T-2** (SQL toolchain: sqlc.yaml, Taskfile-задачи, Go-зависимости). Билд и unit-сьют — GREEN.

Оставшиеся задачи (T-3 .. T-8) масштабны (Postgres-адаптер + переписанный SQLite + factory + контракт-сьют + CLI v2) и будут выполняться в последующих сессиях/итерациях через `pipeline.sh task T-N` для resume.

## Commands Used

- **Test:** `task test` → `go test -v -coverprofile=cover.out ./...`
- **Build:** `task build` → `CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go`
- **Lint:** `task lint` → `golangci-lint run -v`
- **Generate:** `task generate` → `sqlc generate` (sqlc v1.31.1 установлен через `brew install sqlc`; `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` упал на macOS SDK incompatibility в `pg_query_go`)

## Task Execution

- [x] **T-1** Foundation: новые ошибки и расширенная валидация конфига — GREEN
  - Добавлены `core.ErrConflict`, `core.ErrMigrationFailed` в `internal/core/errors.go`.
  - Создан `internal/core/errors_test.go` (`TestErrorsExist`, `TestErrorsAreDistinct`).
  - В `internal/adapters/config/config.go` `Validate()` усилен:
    - `Storage.Type=sqlite` без DSN (с fallback `c.SQLite.DSN`) → `ErrConfigInvalid`.
    - `Storage.Type=postgres` без `Postgres.DSN` → `ErrConfigInvalid`.
    - `Postgres.MaxOpenConns<0` или `MaxIdleConns<0` → `ErrConfigInvalid`.
    - Добавлен helper `Config.SQLiteDSN()` (storage > legacy).
  - В `config_test.go` обновлён `TestConfig_Validate_AcceptsAllStorageTypes` (postgres-кейс получил DSN); добавлены table-driven `TestConfig_Validate_StorageDSN` (8 кейсов) и `TestConfig_SQLiteDSN_Priority`.
  - Все existing tests остались GREEN.

- [x] **T-2** SQL toolchain: sqlc + goose + Taskfile — GREEN
  - Создан `sqlc.yaml` в корне (engines: `sqlite`, `postgresql`; out: `internal/adapters/sqlite/sqlitedb`, `internal/adapters/postgres/pgdb`).
  - В `Taskfile.yml` добавлены задачи `test:integration`, `generate`, `db:up`, `db:status`.
  - Созданы скелетные директории с `.gitkeep`: `internal/adapters/sqlite/{migrations,queries,sqlitedb}`, `internal/adapters/postgres`, `internal/adapters/postgres/{migrations,queries,pgdb}`, `internal/adapters/storage`, `internal/adapters/repotest`.
  - В `go.mod` добавлены: `github.com/jackc/pgx/v5`, `github.com/pressly/goose/v3`, `github.com/testcontainers/testcontainers-go`, `github.com/testcontainers/testcontainers-go/modules/postgres`. Транзитивно подтянуты `golang.org/x/crypto`, `go.opentelemetry.io/*`, `shirou/gopsutil/v4`, `sirupsen/logrus`, и др.
  - Версии `golang.org/x/{exp,sync,sys,text}` и `modernc.org/{libc,sqlite}` обновлены через transitive dep resolution (без breaking change для проекта).
  - `task build`, `task test` GREEN. `sqlc generate` отрабатывает корректно (ожидаемая ошибка "no queries contained" для пустых query-директорий).

- [ ] **T-3** SQLite v2: миграции + sqlc-запросы + переписанный repository — pending
- [ ] **T-4** Postgres адаптер: миграции + sqlc + pgxpool + integration tests — pending
- [ ] **T-5** Storage factory + CLI rewire — pending
- [ ] **T-6** Контракт-сьют + integration tests + CI — pending
- [ ] **T-7** CLI: `migrate db`, `migrate --from --to`, `doctor` v2 — pending
- [ ] **T-8** GATE — pending

## Final Verification (после T-1, T-2)

- **Tests** (`go test ./...`):
```
ok  	github.com/jtprogru/jtpost/internal/adapters/config	0.345s
ok  	github.com/jtprogru/jtpost/internal/adapters/fsrepo	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/httpapi	0.982s
ok  	github.com/jtprogru/jtpost/internal/adapters/sqlite	0.676s
ok  	github.com/jtprogru/jtpost/internal/adapters/telegram	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegramconv	(cached)
ok  	github.com/jtprogru/jtpost/internal/cli	1.294s
ok  	github.com/jtprogru/jtpost/internal/core	(cached)
ok  	github.com/jtprogru/jtpost/internal/logger	(cached)
```

- **Build** (`task build`):
```
task: [tidy] go mod tidy
task: [build] go mod download
task: [build] CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go
```

- **sqlc** (`sqlc version`):
```
v1.31.1
```

- **Lint:** не запускался отдельно — full lint будет в T-8 (GATE).

## Files Changed (T-1 + T-2)

**Modified:**
- `internal/core/errors.go` — `ErrConflict`, `ErrMigrationFailed`
- `internal/adapters/config/config.go` — `Validate()` extension, `SQLiteDSN()` helper
- `internal/adapters/config/config_test.go` — новые/обновлённые тесты
- `Taskfile.yml` — `test:integration`, `generate`, `db:up`, `db:status`
- `go.mod`, `go.sum` — pgx/v5, goose/v3, testcontainers-go и transitive deps

**Created:**
- `internal/core/errors_test.go`
- `sqlc.yaml`
- `internal/adapters/{sqlite,postgres}/{migrations,queries,sqlitedb|pgdb}/.gitkeep`
- `internal/adapters/postgres/.gitkeep`
- `internal/adapters/storage/.gitkeep`
- `internal/adapters/repotest/.gitkeep`

## Notes

### Environment
- **sqlc:** установлен через `brew install sqlc` (v1.31.1). `go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.27.0` падает на macOS Tahoe из-за `pg_query_go/v5` C-source несовместимости с современным Xcode SDK. Контрибьюторам Linux это не касается; macOS-инструкция в `CONTRIBUTING.md` будет добавлена в T-8.
- **Docker:** не запущен на этой машине. Integration-тесты Postgres (T-4, T-6) при `task test:integration` будут пропускаться через `t.Skip("docker not available")`. Полная верификация integration-сьюта — в окружении с запущенным Docker (Linux CI runner).

### Pending scope для T-3..T-8 (значительный объём)
- **T-3 SQLite v2** требует:
  - DDL миграции `0001_initial.sql` (DROP+CREATE по F1-схеме)
  - 8–10 sqlc-запросов (`CreatePost`, `GetPostByID`, `GetPostBySlug`, `ListPosts` с динамической сортировкой, `UpdatePost` с optimistic lock, `DeletePost`, `ImportPost`, `CountPosts`, `PostExistsByID`)
  - Переписанный `repository.go` (~400 строк): JSON-маршалинг `attachments`/`publish_history`/`cover_image`, scope из `core.TenantFromContext`, `errors.Join(core.ErrMigrationFailed, ...)`, `ErrConflict` mapping.
- **T-4 Postgres** аналогично + `pgxpool` lifecycle, eager Ping, `pgtype.UUID/Timestamptz/Jsonb` mapping, integration tests c testcontainers.
- **T-5 Storage factory** + перевязка ~10 CLI-команд + `internal/adapters/httpapi/server.go` wiring.
- **T-6 RunContract** в `internal/adapters/repotest` (15+ subtests с capability gating) + 3 backend wiring + CI workflow update.
- **T-7 CLI** — новые `migrate db {up,status}`, переписанный `migrate --from --to`, `doctor` v2 c DSN-маскированием.
- **T-8 GATE** — full verification + CHANGELOG + `.jtpost.example.yaml` обновление.

### Next steps
Продолжить можно через `/loop` либо в следующих сессиях. `pipeline.sh status` покажет прогресс; задачи маркируются через `pipeline.sh task T-N`.
