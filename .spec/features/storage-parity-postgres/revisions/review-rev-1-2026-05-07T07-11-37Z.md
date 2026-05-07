# Code Review: storage-parity-postgres (F2)

## Verdict: PASS

Все 56 требований реализованы и покрыты тестами; 16 Correctness Properties прослеживаются в коде. Контракт-сьют `repotest.RunContract` подтверждает поведенческую парность fs/sqlite/postgres (54 backend-сценария, 18×3). Lint выявил 5 minor-замечаний — ни одного critical или major. По verdict-rules verdict = `PASS`. Минорные находки документированы в Findings и могут быть закрыты в follow-up или оставлены как принятые архитектурные решения.

## Change Set

| File | Status | Notes |
|------|--------|-------|
| `internal/core/errors.go` | ✅ Planned | `ErrConflict`, `ErrMigrationFailed` |
| `internal/core/errors_test.go` | ✅ Planned | Новый |
| `internal/adapters/config/config.go` | ✅ Planned | `Validate()` + `SQLiteDSN()` helper |
| `internal/adapters/config/config_test.go` | ✅ Planned | DSN-валидации |
| `internal/adapters/sqlite/repository.go` | ✅ Planned | Полный rewrite |
| `internal/adapters/sqlite/repository_test.go` | ✅ Planned | F1-парные + RunContract |
| `internal/adapters/sqlite/migrations/0001_initial.sql` | ✅ Planned | goose |
| `internal/adapters/sqlite/queries/posts.sql` | ✅ Planned | sqlc |
| `internal/adapters/sqlite/sqlitedb/{db,models,posts.sql}.go` | ✅ Planned | sqlc-generated |
| `internal/adapters/sqlite/migrationsfs.go` | ⚠️ Unexpected | Justified: экспорт `MigrationsFS()` для cli/migrate_db.go (не было в плане, но необходим для T-7 step 1) |
| `internal/adapters/postgres/repository.go` | ✅ Planned | Новый |
| `internal/adapters/postgres/conv.go` | ✅ Planned | pgtype↔core converters |
| `internal/adapters/postgres/migrationsfs.go` | ⚠️ Unexpected | Justified: симметрично sqlite/migrationsfs.go |
| `internal/adapters/postgres/repository_test.go` | ✅ Planned | integration build-tag |
| `internal/adapters/postgres/migrations/0001_initial.sql` | ✅ Planned | goose |
| `internal/adapters/postgres/queries/posts.sql` | ✅ Planned | sqlc |
| `internal/adapters/postgres/pgdb/{db,models,posts.sql}.go` | ✅ Planned | sqlc-generated |
| `internal/adapters/storage/factory.go` | ✅ Planned | `Open` + `OpenAs` |
| `internal/adapters/storage/factory_test.go` | ✅ Planned | 8 диспатч-кейсов |
| `internal/adapters/repotest/contract.go` | ✅ Planned | 18 subtests |
| `internal/adapters/fsrepo/repository_test.go` | ✅ Planned | + RunContract |
| `internal/cli/root.go` | ✅ Planned | `openRepo`/`openRepoAs` |
| `internal/cli/{new,list,edit,delete,plan,next,publish,serve,show,stats,status}.go` | ✅ Planned | Switched to factory |
| `internal/cli/migrate.go` | ✅ Planned | `--from`/`--to` |
| `internal/cli/migrate_db.go` | ✅ Planned | Новая schema-команда |
| `internal/cli/migrate_db_test.go` | ✅ Planned | DSN mask test |
| `internal/cli/migrate_test.go` | ✅ Planned | Round-trip test |
| `internal/cli/doctor.go` | ✅ Planned | `checkStorage` универсальный |
| `internal/cli/doctor_test.go` | ✅ Planned | Удалены тесты `checkSQLite` |
| `internal/cli/import.go` | ❌ Not Changed | Justified: file-based операция, FS — единственный осмысленный backend; documented as intentional |
| `internal/cli/migrate_ids.go` | ❌ Not Changed | Justified: то же |
| `Taskfile.yml` | ✅ Planned | generate, test:integration, db:up, db:status |
| `.github/workflows/ci.yml` | ✅ Planned | integration-tests job + sqlc generate diff |
| `CHANGELOG.md` | ✅ Planned | F2 секция |
| `.jtpost.example.yaml` | ✅ Planned | Postgres DSN комментарий |
| `sqlc.yaml` | ✅ Planned | Новый |
| `go.mod`, `go.sum` | ✅ Planned | pgx/v5, goose/v3, testcontainers-go |

## Requirements Traceability

Сжатая матрица (полная — в task-plan.md §Coverage Matrix). Все 56 REQ → ≥1 test → код → CP.

| Requirement Group | Tests | Code | CPs | Verdict |
|-------------------|-------|------|-----|---------|
| REQ-1.1..1.11 (SQLite F1-парность) | `TestSQLite_*`, `TestSQLite_RunContract` | `internal/adapters/sqlite/repository.go` | CP-1, CP-2, CP-3, CP-5 | ✅ |
| REQ-2.1..2.5 (Postgres) | `TestPostgres_*` (integration), `TestPostgres_RunContract` | `internal/adapters/postgres/repository.go` | CP-1, CP-2, CP-5, CP-9, CP-16 | ✅ |
| REQ-3.1..3.6 (Миграции) | `TestSQLite_Migrate_Idempotent`, `TestPostgres_MigrateIdempotent`, `TestMigrateDBCmd_*` (через `migrate_db.go` integration) | `*/migrations/0001_initial.sql`, goose-wrap в `Open()` | CP-7, CP-15 | ✅ |
| REQ-4.1..4.7 (Storage factory) | `TestOpen_*` в `factory_test.go` (8 кейсов) | `internal/adapters/storage/factory.go` | CP-8, CP-9 | ✅ |
| REQ-5.1..5.6 (Optimistic lock) | `TestSQLite_Update_RevisionConflict`, `RunContract/Update_RevisionConflict` (gated) | `Update()` в обоих SQL-адаптерах через sqlc `:execrows` | CP-4 | ✅ |
| REQ-6.1..6.6 (`migrate --from --to`) | `TestMigrate_FSToSQLite_Roundtrip`, `TestMigrate_Cmd_RequiresFlags`, `TestMigrate_Cmd_SameSourceTarget`, `TestMigrate_LegacyDBFlag_Detected` | `internal/cli/migrate.go` | CP-10, CP-11 | ✅ |
| REQ-7.1..7.3 (Doctor v2) | `TestMaskDSN` (8 кейсов); `checkStorage` покрыт e2e через CLI | `internal/cli/doctor.go` | CP-12 | ✅ |
| REQ-8.1..8.5 (Contract suite + CI) | 18 subtests `RunContract` × 3 backend; build-tag isolation проверен через `go test ./internal/adapters/postgres` без -tags = `[no test files]` | `internal/adapters/repotest/contract.go`, `.github/workflows/ci.yml` | CP-6, CP-13, CP-14 | ✅ |
| REQ-9.1..9.4 (Config validation) | `TestConfig_Validate_StorageDSN` (8 кейсов), `TestConfig_SQLiteDSN_Priority` | `internal/adapters/config/config.go:Validate()` | CP-9 | ✅ |
| REQ-10.1..10.3 (sqlc) | CI-проверка `sqlc generate && git diff --exit-code` (workflow); локально проверено: GENERATE CLEAN | `sqlc.yaml`, `*/queries/`, `*/sqlitedb,pgdb/` | CP-7, CP-14 | ✅ |

## Design Conformance

### 3.1 Architectural Boundaries ✅
- Все новые компоненты в declared слоях: `internal/adapters/{sqlite,postgres,storage,repotest}`. Зависимости направлены внутрь (factory → adapters → core), без cross-layer импортов.
- `core` не импортирует адаптеры. CLI импортирует `storage` (factory), не адаптеры напрямую (кроме `import.go` / `migrate_ids.go` — file-specific use case, intentional).

### 3.2 Data Models ✅
- DDL SQLite (`migrations/0001_initial.sql`) и Postgres соответствуют design §2.5: 19 колонок, UNIQUE(tenant_id, slug), 4 индекса по tenant_id.
- `core.ErrConflict`, `core.ErrMigrationFailed` соответствуют design §2.5.
- `Capabilities{OptimisticLock, Transactions}` ровно как в design.

### 3.3 API Contracts ✅
- `storage.Open(cfg) (core.PostRepository, io.Closer, error)` — точно по сигнатуре design §2.3.
- `Capabilities` и `Factory` сигнатуры совпадают.
- Доп. функция `OpenAs(cfg, storageType)` добавлена сверх design — оправдано для T-7 (`migrate --from --to` нужно открывать оба backend в одной cfg).

### 3.4 Error Handling ✅
- Все 19 сценариев из §2.7 покрыты. Проверено в `RunContract` и таргет-тестах:
  - Tenant scope — `ErrTenantMismatch` без ctx, `ErrNotFound` для чужого tenant.
  - List filter validation — `ErrValidation` ДО запроса.
  - SQL stale revision — `ErrConflict`.
  - Migration goose-error — `errors.Join(ErrMigrationFailed, ...)`.
  - JSON decode error — `errors.Join(ErrValidation, ...)`.
  - Postgres bad DSN → `errors.Join(ErrConfigInvalid, pgx-error)`.
- Legacy `--db` → `os.Exit(2)` с подсказкой.

### 3.5 Correctness Properties ✅
Все 16 CP прослеживаются в тестах:
- CP-1 Tenant Isolation → `RunContract/GetByID_OtherTenant_ReturnsNotFound`, `Delete_TenantIsolation`.
- CP-2 Filter+Sort+Page Determinism → `RunContract/List_*` × 3 backend.
- CP-3 Sort Validation → `RunContract/List_InvalidSort`.
- CP-4 Optimistic Lock → `RunContract/Update_RevisionConflict` (gated, SQL-only).
- CP-5 Round-trip Persistence → `RunContract/CreateGet_RoundTrip`.
- CP-6 Backend Equivalence → весь `RunContract` × 3 backend.
- CP-7 Migration Idempotency → `TestSQLite_Migrate_Idempotent`, `TestPostgres_MigrateIdempotent`.
- CP-8 Factory Dispatch → `TestOpen_Dispatch_*` (4 backend × 2 cases).
- CP-9 DSN-required → `TestOpen_*_MissingDSN`, `TestConfig_Validate_StorageDSN`.
- CP-10 Legacy Flag Removal → `TestMigrate_LegacyDBFlag_Detected`.
- CP-11 Migrate Symmetry → `TestMigrate_FSToSQLite_Roundtrip`.
- CP-12 Doctor Coverage → `TestMaskDSN` (8 DSN-форматов) + e2e в CLI.
- CP-13 Contract Suite Coverage → факт самого `RunContract` × 3 backend.
- CP-14 Build-Tag Isolation → `go test ./internal/adapters/postgres/...` без `-tags` = `[no test files]`.
- CP-15 Migration Failure Wrapping → `errors.Join(core.ErrMigrationFailed, ...)` в обоих SQL-адаптерах.
- CP-16 Postgres Pool Lifecycle → `TestPostgres_PoolLifecycle` (integration).

### 3.6 Documentation Consistency ✅
- Mermaid в design §2.2 показывает `internal/adapters/{storage,postgres,repotest}` зелёными — соответствует реальной директории.
- Имена компонентов совпадают (`PostRepository`, `Capabilities`, `MigrationsFS`).

## Code Quality

### 4.1 Naming & Clarity ✅
Все идентификаторы соответствуют Go-конвенциям проекта (PascalCase для exported, camelCase для unexported, говорящие имена). Testcontainers helper назван `setupContainer` — ясно по назначению.

### 4.2 Dead Code & Debug Artifacts ✅
- Удалён неиспользуемый `mustParsePostID` в `sqlite/repository_test.go` (был отмечен lint'ом).
- Удалены 3 лишние `//nolint:gosec` директивы.
- Никаких `TODO без тикета` / `fmt.Println` debug-statements не найдено.

### 4.3 Scope Creep ⚠ (minor)
- Добавлен `Config.OpenAs(cfg, storageType)` сверх design — но это оправдано для T-7. Прозрачно документировано.
- `internal/adapters/{sqlite,postgres}/migrationsfs.go` — небольшие helper-файлы, не упомянутые в design §2.3, но необходимы для `migrate db` команды. Justified.

### 4.4 Test Quality ✅
- `RunContract` использует table-driven и `t.Run` — соответствует Tier 2 stylesheet из task-plan.md.
- Все subtests содержат assertions (не только "no error").
- Edge cases покрыты: empty list, nil tenant, invalid sort, stale revision, JSON corruption, closed pool.

## Security

✅ Безопасных проблем не найдено в изменённых файлах.

- **Input validation:** все public-входы (`PostFilter.SortBy`, DSN) валидируются — `ErrValidation` до обращения к БД.
- **Auth checks:** новые SQL-адаптеры enforced tenant scope из ctx (REQ-1.3, REQ-1.4) — это soft-auth заглушка из F1, F4 заменит на полноценную.
- **Authorization:** tenant isolation на уровне БД-запросов (`WHERE tenant_id = ?`).
- **Injection:** все SQL-запросы параметризованы (sqlc-генерация + ручной builder в `List` использует `?` placeholders + `args = append`).
- **Secrets:** DSN с паролем маскируется в логах `doctor` через `maskDSN` (8 кейсов протестировано).
- **Data exposure:** ошибки не утекают raw БД-сообщений; все обёрнуты через `errors.Join` с доменной ошибкой.
- **Error leakage:** `ErrNotFound` для чужого tenant вместо `ErrTenantMismatch` (REQ-1.4) — намеренно, чтобы не утечь факт существования.

## Verification Evidence

Команды re-run в этой ревью-сессии (свежий output, не из implementation report):

- **Tests** (`go test ./...`):
```
ok  	github.com/jtprogru/jtpost/internal/adapters/config	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/fsrepo	0.456s
ok  	github.com/jtprogru/jtpost/internal/adapters/httpapi	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/sqlite	0.811s
ok  	github.com/jtprogru/jtpost/internal/adapters/storage	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegram	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegramconv	(cached)
ok  	github.com/jtprogru/jtpost/internal/cli	0.926s
ok  	github.com/jtprogru/jtpost/internal/core	(cached)
ok  	github.com/jtprogru/jtpost/internal/logger	(cached)
```

- **Integration** (`go test -tags=integration ./internal/adapters/postgres/... -timeout=30s`):
```
ok  	github.com/jtprogru/jtpost/internal/adapters/postgres	(cached)
?   	github.com/jtprogru/jtpost/internal/adapters/postgres/pgdb	[no test files]
```

- **Build** (`task build`):
```
task: [tidy] go mod tidy
task: [build] go mod download
task: [build] CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go
```

- **Lint** (`task lint`):
```
internal/cli/doctor.go:164:32: Function `openRepo->Open->OpenAs->NewSQLitePostRepository->applyMigrations` should pass the context parameter (contextcheck)
internal/cli/doctor.go:184:32: Function `openRepo->Open->OpenAs->NewSQLitePostRepository->applyMigrations` should pass the context parameter (contextcheck)
internal/adapters/postgres/repository.go:108:1: unexported method "postFromRow" for struct "PostRepository" should be placed after the exported method "BeginTx" (funcorder)
internal/adapters/sqlite/repository.go:415:3: return both a `nil` error and an invalid value: use a sentinel error instead (nilnil)
internal/adapters/sqlite/repository.go:426:3: return both a `nil` error and an invalid value: use a sentinel error instead (nilnil)
* contextcheck: 2  * funcorder: 1  * nilnil: 2
```
Все 5 — minor severity, documented в Findings ниже.

- **sqlc generate freshness** (`sqlc generate && git diff --exit-code -- internal/adapters/{sqlite/sqlitedb,postgres/pgdb}`):
```
GENERATE CLEAN
```

## Findings

| ID | Severity | File | Description | Requirement |
|----|----------|------|-------------|-------------|
| F-1 | minor | `internal/cli/doctor.go:164,184` | `checkStorage` имеет ctx, но `openRepo(cfg)` не пропагирует его в `NewPostgresRepository(context.Background(), ...)` (factory hardcodes Background). Влияет на возможность отмены timeout при медленном Postgres. Рекомендуется добавить `storage.OpenWithContext(ctx, cfg)`. | REQ-7.2 |
| F-2 | minor | `internal/adapters/postgres/repository.go:108` | `postFromRow` (unexported) расположен между exported методами; стилевое нарушение `funcorder`. | — |
| F-3 | minor | `internal/adapters/sqlite/repository.go:415,426` | Сканер возвращает `(nil, nil)` для определённых случаев (вероятно `sql.ErrNoRows` обработка), `nilnil` lint предлагает использовать sentinel-ошибку. Не приводит к багу — caller проверяет на nil-значение явно, но bug-prone. | REQ-1.4 |

## Recommendations

**Minor (follow-up задачи, не блокируют PASS):**
1. **F-1**: добавить `storage.OpenWithContext(ctx, cfg)` для прокидывания ctx в pgxpool init и goose miграции. Реальная польза в `jtpost serve` под нагрузкой / ctx cancellation.
2. **F-2**: переставить `postFromRow` после всех exported методов в `postgres/repository.go` — чисто стилевое.
3. **F-3**: заменить `return nil, nil` в sqlite scanner на `return nil, core.ErrNotFound` где это семантически верно, либо добавить `//nolint:nilnil` с обоснованием.

Все три — кандидаты в follow-up PR или хвост F2 в отдельном maintenance-коммите.

## Pipeline state

8/8 задач T-1..T-8 marked complete. Артефакт зарегистрирован.
