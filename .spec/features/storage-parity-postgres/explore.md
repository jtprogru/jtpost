# Exploration: Storage Parity — SQLite + Postgres (F2)

## Intent

F2 — следующая после F1 фича большой программы доведения jtpost до финала. После F1 доменная модель и схема конфига расширены под multi-tenant, медиа, историю публикаций и пагинацию, но **реальная имплементация хранилищ за этим не поспела**:

- `fsrepo` — приведён к F1-контракту (tenant-подкаталоги, новые поля frontmatter, scope из context).
- `sqlite` — фактически в F0-состоянии: схема знает только базовые поля, `List/Create/Update` игнорируют `tenant_id`, `author_id`, новые опциональные поля, сортировку, пагинацию и тенант-скоуп.
- `postgres` — отсутствует целиком, хотя `PostgresConfig` уже в схеме конфига.
- Селектора хранилища нет — все CLI-команды хардкодят `fsrepo.NewFileSystemRepository(cfg.PostsDir)`, а `cfg.Storage.Type` читается только для валидации.

**Цели F2:**

1. Привести `internal/adapters/sqlite` к полному F1-контракту (все обязательные/опциональные поля Post, tenant scope из context, `PostFilter` с сортировкой/пагинацией/AuthorID, `IsStatusTransitionValid` через сервис, миграции с версией).
2. Реализовать `internal/adapters/postgres` с теми же контрактами и таблицами (изоморфно SQLite, но с использованием `pgx` и Postgres-специфичных типов: `uuid`, `jsonb`, `timestamptz`).
3. Ввести единую миграционную систему (goose) и кодген типизированных запросов (sqlc) — общий SQL слой, два диалекта.
4. Добавить storage factory (`internal/adapters/storage`) и переключение `cfg.Storage.Type ∈ {fs|sqlite|postgres}` на старте CLI/serve.
5. Завести параметризованный contract-test suite, гоняемый против fs/sqlite/postgres для гарантии семантической идентичности.
6. Постгресс-интеграционные тесты — через `testcontainers-go` (Postgres 16).
7. Подготовить миграционный путь данных: extend `jtpost migrate` под `--to fs|sqlite|postgres`.

**Чего F2 не делает:**

- Не реализует git-хранилище (F3) и `storage.git.*`.
- Не реализует RLS (row-level security) в Postgres — это C-этап.
- Не подключает worker (F6) к новой `publish_history`-таблице.
- Не вводит `audit_log`, `users`, `channels`, `tenants` таблицы из B-этапа DEVELOPMENT_PLAN — только `posts`, `post_attachments`, `post_publish_attempts`.

**Триггер:** F1 закрыт (commit `1405fe2`), `storage.type` в конфиге уже валидируется, но не имеет реализации. F2 закрывает этот разрыв.

---

## Investigation

### Что уже есть после F1

**Схема конфига (`internal/adapters/config/config.go`):**
- `StorageConfig{Type, Git, SQLite, Postgres}` со всеми подсекциями.
- Defaults: `Type="fs"`, `Postgres.MaxOpenConns=10`, `MaxIdleConns=5`, `ConnMaxLifetime=30m`.
- `Validate()` ругается только на неизвестное значение `Type` — наличие DSN при `Type=sqlite|postgres` не проверяется.

**Доменная модель (`internal/core/post.go`):**
- `Post` с обязательными `TenantID`, `AuthorID`, `CreatedAt`, `UpdatedAt`, `Revision` и опциональными `Excerpt`, `CoverImage`, `Attachments[]`, `PublishHistory[]`, `RevisionSHA`.
- `PostFilter{TenantID, AuthorID, Statuses, Tags, Search, SortBy, SortOrder, Limit, Offset}`.
- `core.WithTenant`/`TenantFromContext` — context-scope, fsrepo уже им пользуется (`fsrepo/repository.go:39`).
- 7 статусов + `AllowedTransitions` (см. F1 REQ-3.x).

**fsrepo (`internal/adapters/fsrepo/repository.go`):**
- Эталонная имплементация F1: `<root>/<tenant_short>/<slug>.md`, scope из context, новые поля frontmatter, сортировка/пагинация на стороне Go.
- Tests in `repository_test.go`, `frontmatter_parser_test.go` — служат поведенческим референсом.

**sqlite (`internal/adapters/sqlite/repository.go`) — gap-листинг:**
- L80–87 `GetByID`: SELECT не возвращает `tenant_id`, `author_id`, `revision`, `excerpt`, `cover_image`, нет JOIN'ов на attachments/publish_attempts. Сканер `scanPost` (L322) тоже не читает их.
- L101–159 `List`: игнорирует `filter.TenantID`, `filter.AuthorID`, `filter.SortBy`, `filter.SortOrder`, `filter.Limit`, `filter.Offset`. Sort hardcoded `ORDER BY created_at DESC`.
- L162–199 `Create`: не пишет `tenant_id`, `author_id`, `excerpt`, `cover_image`, `attachments`, `publish_history`, `revision`, `revision_sha`. Игнорирует `post.CreatedAt/UpdatedAt`, перезатирая их `time.Now()`.
- L202–243 `Update`: то же. Не инкрементирует revision (это и не должна делать репа, но не пишет переданное значение).
- L456–479 `migrate()`: ad-hoc `CREATE TABLE IF NOT EXISTS` с DEFAULT '' для tenant_id/author_id — миграции одношаговые, без версионирования. `idx_posts_tenant_id` есть, но используется ли — нет, т.к. селекты не фильтруют.
- L246–249 `Delete`: не проверяет tenant scope — любой может удалить чужой пост.

**postgres:** директории нет.

**Storage factory:** отсутствует. Случаев использования `cfg.Storage.Type` в коде — 3 (валидация + 2 теста). CLI-команды (`new.go`, `list.go`, `delete.go`, `edit.go`, `plan.go`, `next.go`, `import.go`, `migrate.go`, `migrate_ids.go`) и `httpapi/server.go` хардкодят fsrepo.

**Миграционная команда (`internal/cli/migrate.go`):**
- Один путь: `fsrepo → sqlite` (через `MigratableRepository.ImportPosts`).
- Не использует storage factory; параметр `--db` хардкодит SQLite.
- В F2 нужно её сделать «storage-агностичной»: `--from fs|sqlite|postgres --to fs|sqlite|postgres`.

### Зависимости и тулинг

- Текущие deps (`go.mod`): `modernc.org/sqlite v1.46.1` (pure-Go, CGO_ENABLED=0 совместим), `google/uuid`, `viper`, `cobra`, `yaml.v3`.
- Postgres драйвер: **`pgx/v5`** (`github.com/jackc/pgx/v5`) — современный стандарт, поддерживает `pgxpool`, native `uuid`, `jsonb`, `timestamptz`, `LISTEN/NOTIFY` (для будущего F6).
- Миграции: **`pressly/goose v3`** — embed via `//go:embed`, поддерживает Postgres + SQLite одним и тем же набором (`-- +goose Up`/`Down`). Альтернатива — `tern` (только Postgres) или `golang-migrate`.
- Кодген: **`sqlc`** — типизированные функции по `*.sql`. Поддерживает оба диалекта, но требует **отдельных query-файлов** под каждый (sqlite vs postgres имеют разный синтаксис JSON-операторов, plpgsql, timestamptz). Альтернатива — писать запросы руками через `pgxscan`/`scanboi`.
- Integration tests: `testcontainers-go v0.32+`, `postgres` модуль; в CI должен быть Docker. macOS-runner GitHub Actions поддерживает только в режиме colima — это риск.

### Тестовый контекст

- Стиль: native `testing` + table-driven + `testdata/`.
- Для Postgres тестов нужна нестандартная схема `internal/adapters/postgres/integration_test.go` с build-тегом `// +build integration` и `t.Skip` если Docker недоступен.
- `task test` гоняет всё подряд — для F2 разумно ввести `task test:integration` отдельно (или флаг `-tags=integration`) чтобы unit-suite оставался быстрым.
- Contract-test suite (Option B+ ниже) — общий пакет `internal/adapters/repotest` с функцией `RunContract(t, factory)`, вызываемой из `fsrepo`, `sqlite`, `postgres` тестов.

### Влияние на другие подсистемы

- `cmd/jtpost/main.go` — без изменений (работает через cobra root).
- `internal/cli/root.go` — после F1 есть `rootCmd` без `getService`. Нужна новая функция `openRepository(cfg) (core.PostRepository, io.Closer, error)`, которую дернёт каждая команда.
- `internal/adapters/httpapi/server.go` — сейчас принимает уже сконструированный `core.PostService`. Конструктор сервиса должен получать `PostRepository`, выбранный из storage factory. Перевязать на этапе CLI `serve`.
- `cmd/jtpost/main.go` ничего не знает о storage. CLI-команды получат helper `cli.OpenRepo(cfg)`.
- `Taskfile.yml` — добавить `task test:integration`, `task migrate:up`, `task migrate:status` (для разработки локально с docker postgres).
- `internal/cli/doctor.go` — проверка соединения с выбранным storage.

### Релевантные файлы кода (чтобы не угадывать в Design)

- `internal/adapters/sqlite/repository.go:80-484` — переписать целиком (либо сгенерировать через sqlc).
- `internal/adapters/sqlite/repository_test.go` — расширить под F1-поля + использовать contract-suite.
- `internal/adapters/config/config.go:117-161` (defaults), `:303-320` (Validate) — добавить проверки `Storage.Postgres.DSN != ""` если Type=postgres.
- `internal/cli/{delete,edit,import,list,migrate,migrate_ids,new,next,plan}.go` — заменить `fsrepo.NewFileSystemRepository(cfg.PostsDir)` на `openRepo(cfg)`.
- `internal/adapters/httpapi/server_test.go` — проверить, что подмена через factory не ломает.

---

## Build Tooling

- **Orchestrator:** [Task](https://taskfile.dev) (`Taskfile.yml`).
- **Test (unit):** `task test` (= `go test -v -coverprofile=cover.out ./...`).
- **Test (race):** `task test:race`.
- **Test (integration, новое в F2):** `task test:integration` (= `go test -tags=integration ./internal/adapters/postgres/...`) — добавляется в этой фиче.
- **Build:** `task build` (CGO_ENABLED=0).
- **Lint:** `task lint` (golangci-lint).
- **Generate (новое в F2):** `task generate` (= `sqlc generate`) — добавляется в этой фиче.
- **Migrate (dev, новое):** `task db:up`, `task db:down`, `task db:status` — обёртки над `goose -dir migrations/<dialect>`.
- **Source:** `Taskfile.yml`, `sqlc.yaml`, `migrations/{sqlite,postgres}/*.sql`.
- CI: GitHub Actions — добавить job `integration-tests` с `services: postgres:16`. Альтернатива — testcontainers, тогда отдельный сервис не нужен.

---

## Options Considered

### Option A: Ручные SQL-запросы в каждом адаптере, без кодгена

Переписать `internal/adapters/sqlite` руками под F1-контракт. Сделать `internal/adapters/postgres` копи-пастой с заменой плейсхолдеров и `pgx`. Миграции — `goose` с раздельными файлами под каждый диалект.

- **Pros:** ноль новых тулзов в pipeline; pgx и database/sql уже хорошо понятны; легко начать.
- **Cons:** дублирование сканнеров и query-builder'ов на ~1000 строк. Любой рефакторинг доменной модели требует двух правок. JSONB/JSON-разница в `attachments`/`publish_history` — каждый раз руками.
- **Сложность:** Medium. Объём кода: High.

### Option B: `goose` + `sqlc` + общий contract-suite

`sqlc` генерирует типизированные функции по `*.sql` файлам, отдельным для каждого диалекта (`internal/adapters/sqlite/queries/`, `internal/adapters/postgres/queries/`). Бизнес-логика адаптера тонкая — оборачивает sqlc-вызовы в реализацию `core.PostRepository`. `goose` — миграции.

- **Pros:** типобезопасные запросы, компилятор ловит mismatch; легко расширять (новая колонка → миграция → `sqlc generate` → `go build` указывает где править); единый `repotest.RunContract(t, factory)` гарантирует семантическую идентичность fs/sqlite/postgres.
- **Cons:** новый тулчейн в pipeline (`sqlc`, `goose`); `sqlc` всё-таки требует двух разных query-наборов из-за диалектных различий (особенно с JSON/UUID/timestamps); чуть выше cold-start стоимость для контрибьютора.
- **Сложность:** Medium-High (входной).

### Option C: ORM (`ent`, `gorm`)

Описать схему один раз через ent-схему, получить и SQLite, и Postgres диалекты бесплатно, плюс декларативные миграции.

- **Pros:** одна модель — два адаптера; миграции автогенерируются.
- **Cons:** жирная зависимость; теряется явный контроль над SQL (важно для будущих RLS/индексов в C-этапе); ent-схема дублирует доменную модель из `core/post.go` (потенциальный source-of-truth конфликт); проект до сих пор «без ORM» — это сознательный архитектурный выбор; миграция на ORM — отдельный архитектурный решение.
- **Сложность:** High (порог входа), Medium (последующая поддержка).

### Option D: Только Postgres, SQLite legacy

Признать SQLite адаптер depreciated в F2; новые фичи добавлять только в Postgres; SQLite оставить «как есть» с пометкой `legacy/single-user`.

- **Pros:** сильно меньший объём работы; sqlc только под Postgres (никаких диалектных различий).
- **Cons:** ломает обещание DEVELOPMENT_PLAN B.1 («Сохранить SQLite как fallback для одиночного режима»); контракт `cfg.Storage.Type=sqlite` в коде есть, в реализации — пусто; CLI-режим без Docker (новички, ноутбуки) ухудшается.
- **Сложность:** Low. Цена для пользователя: High.

---

## Constraints & Risks

### Backward compatibility

- Существующие `.jtpost.db` от старой sqlite-реализации не содержат `tenant_id`/`author_id`/`revision`/новых полей. Миграция goose должна `ALTER TABLE` + бэкфил `auth.tenant_default`/`auth.author_default` из конфига.
- `[ASSUMPTION: «старых» БД в продакшене нет]` — подтверждено пользователем в F1 (M1). Однако `internal/cli/migrate.go` уже создаёт sqlite-файлы в dev-окружениях. Решение: первая goose-миграция = drop+recreate с предупреждением **только если** в env стоит `JTPOST_FORCE_RESET=1`; иначе — `ALTER TABLE` с бэкфилом.

### Транзакционность и tenant scope

- `GetByID(tenantless ctx)` в SQL должен вернуть `ErrTenantMismatch` (как в fsrepo `repository.go:41`) — не `ErrNotFound`. Но `List/GetBySlug` без tenant — `ErrValidation`. Контракт нужно явно зафиксировать в Design.
- В Postgres есть способ через RLS, но это C-этап; в F2 — фильтр `WHERE tenant_id = $1` в каждом запросе.

### Производительность

- `attachments` и `publish_history` — отдельные таблицы или JSONB-колонки в `posts`?
  - Отдельные таблицы → дополнительные JOIN/N+1; чтение поста = 3 запроса минимум.
  - JSONB → один SELECT, но фильтрация/индексация по полям внутри сложнее.
  - Решение в Design. Предложение: **JSONB в Postgres, JSON-text в SQLite** для `attachments`/`publish_history`; отдельные таблицы — в C-этапе при росте требований к media-управлению.
- F1 фиксирует «последние 10 publish_attempts в FS». В SQL-режимах требование снимается (полная история). Контракт: `MigratableRepository.Count` в SQL может включать «исторические» записи, не теряет их.

### Security

- DSN с паролем в логах: явно маскировать в `doctor` и при ошибке подключения.
- SQL-инъекции: только параметризованные запросы (sqlc гарантирует, ручные — через `?`/`$1`).
- testcontainers поднимает локальный Postgres — проверить что не торчит на 0.0.0.0.

### Dependencies (новые)

- `github.com/jackc/pgx/v5` — Postgres driver+pool.
- `github.com/pressly/goose/v3` — миграции (поддержка sqlite+postgres).
- `github.com/sqlc-dev/sqlc` — кодген (CLI tool, не runtime dep).
- `github.com/testcontainers/testcontainers-go` + `testcontainers-go/modules/postgres` — только для test build.
- При Option B `modernc.org/sqlite` остаётся как было.

### CI

- `services: postgres:16` в workflow или testcontainers; macOS-job — colima/не работает; решение — гонять integration только на Linux runner.
- Coverage gate: новые адаптеры считаются как отдельные пакеты.

### Edge cases

- **Postgres `uuid` vs `text`:** хранить `tenant_id`/`author_id` в `uuid`-колонках, не `text` — иначе теряем индекс-поиск и `=` сравнение работает медленнее.
- **`time.Time` precision:** Postgres `timestamptz` (microsecond), SQLite `TEXT` (RFC3339 с T и 'Z'). Нужно явно нормализовать в обоих сканерах.
- **`json.RawMessage` в `PublishAttempt.ResponsePayload`:** в Postgres — `jsonb`; в SQLite — `TEXT` с валидацией на чтение (parse-on-read).
- **`uuid.Nil` в `AuthorID`:** в F1 он обязателен. SQL должен `NOT NULL`. Переход с дефолтных `''` в текущей sqlite-схеме — миграция.
- **Concurrent updates / Revision race:** F2 фиксирует контракт «`UPDATE ... WHERE id=? AND revision=?` или `UPDATE` без проверки + сервис увеличивает Revision»? Решение в Design (предложение: optimistic lock — `UPDATE WHERE revision=? RETURNING revision+1`).
- **`storage.git` ≠ `storage.type=fs`:** git-режим — отдельный декоратор поверх fs (F3), а не отдельный backend type. Зафиксировать.

---

## Recommended Direction

**Option B (goose + sqlc + общий contract-suite) с инкрементальным rollout по слоям:**

1. **Контракт-сьют** `internal/adapters/repotest/` — выделяем поведенческие тесты из текущих `fsrepo/repository_test.go` в переиспользуемую форму `RunContract(t, factory)`. Гоняем против fsrepo сразу — должны зеленеть без изменений.
2. **Миграции SQLite** через goose: первая миграция (`0001_initial.sql`) добавляет F1-поля, отдельные таблицы `post_attachments`, `post_publish_attempts` (или JSON-колонки — solidify в Design).
3. **sqlc для SQLite** — переписываем `sqlite/repository.go` как тонкую обёртку. Гоняем contract-suite.
4. **Миграции Postgres** + sqlc-конфиг (отдельный dialect). `internal/adapters/postgres/repository.go` создаётся аналогично sqlite. testcontainers integration test.
5. **Storage factory** `internal/adapters/storage/factory.go` — `Open(cfg) (core.TransactionalRepository, io.Closer, error)`.
6. **CLI rewire** — все команды читают `cfg.Storage.Type` и делегируют factory. `httpapi.serve` тоже.
7. **`jtpost migrate` v2** — `--from <type> --to <type>` использует factory + `MigratableRepository.ImportPosts` на обоих концах.
8. **`jtpost doctor` v2** — пинг выбранного storage.

Эта последовательность даёт rolling-зелёные тесты: после шагов 1, 3, 4 работает только тот backend, который завершён, но `fs` не ломается ни на одном шаге.

---

## Scope Boundaries

### Must-have (v1, эта фича)

- **SQLite адаптер в F1-парности:** все обязательные/опциональные поля Post, scope из context, full PostFilter (TenantID/AuthorID/SortBy/SortOrder/Limit/Offset), `IsStatusTransitionValid`-aware `Update`.
- **Postgres адаптер** на pgx/pgxpool с тем же контрактом и теми же таблицами/колонками семантически.
- **goose-миграции** в `migrations/sqlite/` и `migrations/postgres/` с embed FS; `Open` в каждом адаптере применяет миграции на старте (до B-этапа отдельный `goose up` не обязателен).
- **sqlc** конфиг (`sqlc.yaml`) и `task generate`. Сгенерированные `*.sql.go` файлы коммитятся.
- **Хранение `attachments` и `publish_history`:** JSONB в Postgres, JSON-text в SQLite (одна таблица `posts` + 2 JSON-колонки, без отдельных таблиц в F2). Финализируется в Design.
- **Storage factory** `internal/adapters/storage` с `Open(cfg)`. CLI-команды + httpapi серверный путь перевязаны на factory.
- **Contract-test suite** `internal/adapters/repotest` — общий, гоняется по fs/sqlite/postgres.
- **Integration test для Postgres** через testcontainers; build-tag `integration`. CI-job `integration-tests` (linux only).
- **`jtpost migrate` обновлён:** `--from <fs|sqlite|postgres> --to <fs|sqlite|postgres>`; `--db` оставить как deprecated alias.
- **`jtpost doctor` обновлён:** `Storage` check вместо/в дополнение к `SQLite` — пинг выбранного backend.
- **Optimistic locking по `revision`:** контракт «UPDATE возвращает ErrConflict если revision не совпадает». Финализируется в Design.
- **Тесты:** unit для sqlite (parity), integration для postgres, contract suite.

### Deferred (v2 / последующие фичи)

- **F3:** `storage.git.*` — git-обёртка над fs adapter (auto-commit, push). Не входит в F2.
- **C-этап:** RLS в Postgres, партиции по tenant_id, `audit_log`, `post_versions` (immutable history table).
- **B.5 worker:** outbox-паттерн на `publish_history`. F2 только готовит таблицу/колонку.
- **Отдельные таблицы `attachments`/`publish_attempts`:** если JSON станет узким горлом.
- **Connection retry/circuit breaker** для Postgres.

### Needs spike

- **Можно ли применять одни и те же sqlc-генерируемые типы к двум диалектам?** Скорее нет (разные plpgsql vs sqlite функции, разный синтаксис JSON). Реалистичный план — два отдельных query-набора, два отдельных пакета `sqlite/queries` и `postgres/queries`. Спайк: написать минимальный SELECT one-row + проверить компиляцию обоих.
- **goose embed vs runtime files:** прятать миграции в бинарник (через `embed.FS`) — стандарт, но усложняет ручной `goose up` для DBA. Решение: оба сценария — embed для авто-применения на старте, отдельная команда `jtpost migrate db --to sqlite|postgres` для ручного контроля.
- **testcontainers на macOS GH Actions runner:** работает ли через colima без хака? Если нет — гонять integration-тесты только на Linux.

---

## Assumptions & Open Questions

### Assumptions

- `[ASSUMPTION: «старых» БД sqlite в продакшене нет — только dev-инстансы]` — подтверждено в F1 (M1). Поэтому первая goose-миграция может смело делать `DROP TABLE IF EXISTS` и пересоздавать с F1-схемой; в проде это будет первичная установка.
- `[ASSUMPTION: SQLite остаётся first-class backend наравне с Postgres до конца B-этапа]` — соответствует DEVELOPMENT_PLAN B.1 («Сохранить SQLite как fallback для одиночного режима»).
- `[ASSUMPTION: attachments и publish_history хранятся как JSON-колонки на posts, не отдельные таблицы]` — упрощает миграцию и mapping; отдельные таблицы откладываем до момента, когда понадобится JOIN/индекс по полю внутри.
- `[ASSUMPTION: pgx/v5 + pgxpool — выбранный Postgres-драйвер]` — современный стандарт; database/sql + lib/pq не используем.
- `[ASSUMPTION: миграции применяются автоматически при Open() адаптера]` — снимает оперативную нагрузку с пользователя CLI; для prod-сценариев в B-этапе будет отдельная команда `jtpost migrate db`.
- `[ASSUMPTION: integration-тесты Postgres — только на Linux runner CI]` — macOS testcontainers нестабильны.
- `[ASSUMPTION: optimistic locking по revision реализуется в репозитории через WHERE revision=? и возвращает core.ErrConflict]` — новая ошибка добавляется в `core/errors.go` в Group-1 Design.
- `[ASSUMPTION: storage factory Open возвращает core.TransactionalRepository, не core.PostRepository]` — все адаптеры в проекте уже транзакционные (sqlite — да; fsrepo — заглушка; postgres — будет через pgx.Tx).

### Open Questions (для фазы Requirements)

1. **JSON vs отдельные таблицы для `attachments` и `publish_history` финально?** Предложение — JSON-колонки в F2, отдельные таблицы в C-этапе. Подтвердить или сразу делать таблицы.
2. **Revision lock семантика:** `UPDATE WHERE revision=?` возвращает 0 строк → `ErrConflict`? Или `Update` принимает `expectedRevision` отдельным параметром, а не из `post.Revision`?
3. **Bootstrap данных:** при `storage.type=postgres` и пустой БД — `jtpost serve` ничего не делает или автоматически создаёт «system» tenant из `auth.tenant_default`? Скорее всего — ничего, посты создаются через CLI/API.
4. **Параллельная работа двух backend'ов:** допустим ли сценарий «fs + sqlite одновременно» (например, fs как human-readable, sqlite как кеш для запросов)? F2 — нет; storage.type — единственный.
5. **CI Postgres-сервис vs testcontainers:** что выбрать? testcontainers более воспроизводим локально, но требует Docker; native `services: postgres` быстрее, но не работает локально без `docker compose`. Предложение — testcontainers.
6. **`jtpost migrate` совместимость:** старый формат `--db <path>` оставить как alias? Или сразу удалить (greenfield)?
7. **Миграции — embed или внешние файлы?** Предложение — embed по умолчанию, плюс команда `jtpost migrate db dump` для экспорта SQL.
8. **Coverage gate для F2:** ставить ли в этой фиче минимум (например 80% для adapters)? Или это в F10 (cross-cutting quality).

---

## Done When (для approve)

Quality Checklist:

- [x] Codebase прочитан (cited: `internal/adapters/sqlite/repository.go:80-484`, `config.go:117-320`, `core/post.go`, `core/repository.go`, `core/scope.go`, `internal/cli/migrate.go:40-130`, `internal/adapters/fsrepo/repository.go:17-70`, `go.mod`, `Taskfile.yml`, `plans/DEVELOPMENT_PLAN.md`).
- [x] Сравнены 4 опции (A/B/C/D) с pros/cons/complexity.
- [x] Trade-off'ы явные.
- [x] Scope boundaries: Must-have / Deferred / Needs spike — категоризированы.
- [x] Assumptions помечены тэгом `[ASSUMPTION: ...]`.
- [x] Open Questions есть (8 пунктов).
- [x] Build Tooling зафиксирован (Task + новые: sqlc, goose, testcontainers).
- [ ] Артефакт зарегистрирован через `pipeline.sh artifact` — следующий шаг после approve.
