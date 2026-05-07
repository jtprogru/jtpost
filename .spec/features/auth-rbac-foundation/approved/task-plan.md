# Auth/RBAC Foundation (F4) — Task Plan

**Test Style Source:** Tier 2
- Reference test files: `internal/adapters/sqlite/repository_test.go`, `internal/core/post_test.go`, `internal/adapters/httpapi/server_test.go`, `internal/adapters/storage/factory_test.go`, `internal/adapters/gitrepo/decorator_test.go`.
- Key patterns: native `testing` + table-driven, `t.TempDir()`, `t.Setenv`, mock-структуры в test-файлах, `bcrypt.MinCost (4)` для скорости тестов.
- PBT note: substitute через targeted unit tests с cartesian product параметров.

**Commands:**

| Action               | Command                  | Source         |
|----------------------|--------------------------|----------------|
| Test (unit)          | `task test`              | `Taskfile.yml` |
| Test (race)          | `task test:race`         | `Taskfile.yml` |
| Test (integration)   | `task test:integration`  | `Taskfile.yml` |
| Build                | `task build`             | `Taskfile.yml` |
| Lint                 | `task lint`              | `Taskfile.yml` |
| Generate             | `task generate`          | `Taskfile.yml` |
| Format / Vet         | `task fmt`, `task vet`   | `Taskfile.yml` |

---

## Coverage Matrix

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1 .. 1.5 | T-1 | CP-9 |
| REQ-2.1 .. 2.10 | T-3 | CP-1, CP-2, CP-4, CP-5, CP-6, CP-7, CP-8, CP-9 |
| REQ-3.1 .. 3.6 | T-2 | CP-3, CP-12, CP-16 |
| REQ-4.1 .. 4.5 | T-4 | CP-12 |
| REQ-5.1 .. 5.4 | T-1 (config) | CP-1, CP-14 |
| REQ-6.1 .. 6.6 | T-5 | CP-10, CP-11 |
| REQ-7.1 .. 7.9 | T-6 | CP-13, CP-15 |
| REQ-8.1 .. 8.5 | T-7 | All |

---

## Work Type Classification

**Pure feature** — новые типы (`User`, `APIToken`, `Role`, `Permission`), новые таблицы (`users`, `tokens`), новый сервис (`AuthService`), новые middleware и CLI. Существующее поведение сохраняется при `auth.type=none`.

Task order: GREEN (test fixtures) → CODE (bottom-up) → GREEN (full tests) → GATE.

---

## T-1 — Foundation: Domain types + Config validation

***_Complexity: mechanical_***
***_Requirements: REQ-1.1, REQ-1.2, REQ-1.3, REQ-1.4, REQ-1.5, REQ-5.1, REQ-5.2, REQ-5.3, REQ-5.4_***
***_Preservation: CP-9, CP-14_***

GOAL: подготовить domain types, error constants, scope helpers, config validation. Без зависимостей на repository/sqlc.

Subtasks:
- [ ] 1. **CODE** Создать `internal/core/user.go`: типы `User`, `APIToken`, `Role`, `Permission`, `CreateUserInput`, `IssuedToken`, константы ролей и permissions, функция `RolePermissions(r Role) []Permission` с table-маппингом из design §2.5. Используй map.
- [ ] 2. **CODE** В `internal/core/errors.go` добавить `ErrUnauthorized = errors.New("unauthorized")`, `ErrForbidden = errors.New("forbidden")`.
- [ ] 3. **CODE** В `internal/core/scope.go` добавить:
  - `userKey ctxKey = iota+2` (после tenantKey, authorKey)
  - `roleKey ctxKey`
  - `WithUser(ctx, *User)`, `UserFromContext(ctx) (*User, bool)`
  - `WithRole(ctx, Role)`, `RoleFromContext(ctx) (Role, bool)`
- [ ] 4. **CODE** В `internal/adapters/config/config.go`:
  - `AuthConfig.BCryptCost int` поле
  - `NewDefaultConfig` устанавливает `BCryptCost = 10`
  - `Validate()` extension:
    - `Auth.Type` ∉ `{"", "none", "token"}` → ErrConfigInvalid
    - `Auth.Type == "token" && Storage.Type == "fs"` → ErrConfigInvalid
    - `Auth.BCryptCost < 4 || Auth.BCryptCost > 14` (только если `Auth.Type == "token"`) → ErrConfigInvalid
    - При `Auth.Type == "" && Auth.BCryptCost == 0` → cfg.Auth.BCryptCost = 10 (default fallback)
- [ ] 5. **GREEN** Создать `internal/core/user_test.go` с `TestRolePermissions` (table-driven, 4 ролей × ожидаемые permissions).
- [ ] 6. **GREEN** В `internal/adapters/config/config_test.go` добавить `TestConfig_Validate_AuthSection` (table-driven, 6 кейсов: token+fs fail, token+sqlite ok, token+postgres ok, basic fail, oauth fail, BCryptCost out-of-range).
- [ ] 7. **VERIFY** `go test ./internal/core/... ./internal/adapters/config/...` GREEN.

NOTE: scope.go использует typed `ctxKey` enum. Не конфликтуй с существующими константами F1.

---

## T-2 — Repository: миграции + sqlc + sqlite/postgres адаптеры

***_Complexity: complex_***
***_Requirements: REQ-3.1, REQ-3.2, REQ-3.3, REQ-3.4, REQ-3.5, REQ-3.6_***
***_Preservation: CP-3, CP-12, CP-16_***

GOAL: реализовать `core.UserRepository` и `core.TokenRepository` для sqlite + postgres. Добавить миграцию `0002_users_tokens.sql` (оба диалекта), sqlc-queries, и адаптеры.

Subtasks:
- [ ] 1. **CODE** Создать `internal/core/user_repository.go` с интерфейсами `UserRepository` и `TokenRepository` (см. design §2.3).
- [ ] 2. **CODE** Создать `internal/adapters/sqlite/migrations/0002_users_tokens.sql` (DDL из design §2.5: `users` и `tokens` с FK CASCADE). Используй goose `-- +goose Up`/`Down`.
- [ ] 3. **CODE** Создать `internal/adapters/postgres/migrations/0002_users_tokens.sql` (Postgres-вариант с `uuid`, `timestamptz`, FK syntax, REFERENCES inline).
- [ ] 4. **CODE** Создать `internal/adapters/sqlite/queries/users.sql`:
  - `CreateUser :exec`, `GetUserByID :one`, `GetUserByEmail :one`, `UpdateUser :exec`, `DeleteUser :exec`, `ListUsersByTenant :many`, `CountUsersByTenant :one`, `CountOwnersByTenant :one` (для last-owner check).
- [ ] 5. **CODE** Создать `internal/adapters/sqlite/queries/tokens.sql`:
  - `CreateToken :exec`, `GetTokenByPrefix :one`, `DeleteToken :exec`, `ListTokensByUser :many`, `UpdateTokenLastUsedAt :exec`.
- [ ] 6. **CODE** Аналогично — `internal/adapters/postgres/queries/{users,tokens}.sql` с $N плейсхолдерами и pgx-типами.
- [ ] 7. **CODE** Запустить `task generate` — проверить что обе пары query-файлов сгенерировали Go-код в `sqlitedb/` и `pgdb/` соответственно.
- [ ] 8. **CODE** Создать `internal/adapters/sqlite/users.go`:
  - Методы UserRepository на `*PostRepository` (multi-interface, см. ADR-2).
  - Конверсия `core.User` ↔ `sqlitedb.User`. UUID/Time-форматирование как в существующем `repository.go`.
  - `GetByEmail`/`GetByPrefix` → `sql.ErrNoRows` → `core.ErrNotFound`.
- [ ] 9. **CODE** Создать `internal/adapters/sqlite/tokens.go`:
  - Методы TokenRepository на `*PostRepository`.
  - Все CRUD + `UpdateLastUsedAt`.
- [ ] 10. **CODE** Создать `internal/adapters/postgres/users.go` и `tokens.go` — аналогично, через `pgdb.Queries` и `pgxpool`.
- [ ] 11. **GREEN** Создать `internal/adapters/sqlite/users_test.go`:
  - newRepo helper (использует существующий из `repository_test.go`).
  - TestSQLiteUserRepo_CRUD (table-driven), TestSQLiteUserRepo_GetByEmail_TenantScope (cross-tenant lookup → ErrNotFound), TestSQLiteUserRepo_EmailCollision (duplicate → SQL constraint), TestSQLiteUserRepo_Count_OwnersByTenant.
- [ ] 12. **GREEN** Создать `internal/adapters/sqlite/tokens_test.go`: TestSQLiteTokenRepo_CRUD, TestSQLiteTokenRepo_CascadeDelete (DeleteUser → tokens пользователя пропадают).
- [ ] 13. **GREEN** Создать `internal/adapters/postgres/users_test.go` и `tokens_test.go` под build-tag `integration` (testcontainers). Зеркало sqlite-тестов.
- [ ] 14. **VERIFY** `task generate && task test ./internal/adapters/sqlite/... && task test:integration ./internal/adapters/postgres/...` GREEN.

DO NOT: трогать ImportPosts/Count существующих PostRepository методов. F4 не меняет F2 контракт.

---

## T-3 — AuthService: ядро auth-логики

***_Complexity: complex_***
***_Requirements: REQ-2.1 .. REQ-2.10_***
***_Preservation: CP-1, CP-2, CP-3, CP-4, CP-5, CP-6, CP-7, CP-8, CP-9_***

GOAL: реализовать `core.AuthService`.

Subtasks:
- [ ] 1. **CODE** Создать `internal/core/auth_service.go`:
  - `AuthService` struct с полями `users UserRepository`, `tokens TokenRepository`, `bcryptCost int`, `clock Clock`.
  - `NewAuthService(...)` конструктор.
  - `CreateUser(ctx, in CreateUserInput) (*User, error)`:
    - Валидация email (через `mail.ParseAddress`), password ≥ 8.
    - bcrypt password (cost = bcryptCost).
    - Сгенерировать UUID v7.
    - users.Create. На SQL `UNIQUE` violation → `core.ErrAlreadyExists`.
  - `VerifyPassword(ctx, tenantID, email, password) (*User, error)`:
    - users.GetByEmail.
    - bcrypt.CompareHashAndPassword. Mismatch → ErrUnauthorized. NotFound → также ErrUnauthorized (не утечка).
  - `IssueToken(ctx, userID, name, expiresIn *time.Duration) (*IssuedToken, error)`:
    - Helper `genTokenParts() (prefix, secret string)` — 8/24 chars из `crypto/rand` + base62 encoder.
    - bcrypt(secret, cost=6).
    - Сохранить APIToken. Retry до 3 раз при `UNIQUE(prefix)` collision.
    - Вернуть `IssuedToken{Raw: "jtpat_<prefix>_<secret>", Token: stored}`.
  - `ValidateToken(ctx, raw) (*User, Role, error)`:
    - regex match `^jtpat_[A-Za-z0-9]{8}_[A-Za-z0-9]{24}$`. Mismatch → ErrUnauthorized без SQL.
    - tokens.GetByPrefix → secret_hash. bcrypt compare. ExpiresAt check.
    - users.GetByID → user. Вернуть `(user, user.Role, nil)`.
    - Async goroutine: `tokens.UpdateLastUsedAt(context.Background(), token.ID, clock.Now())`.
  - `RevokeToken(ctx, id)`: tokens.Delete.
  - `AuthorizeOperation(ctx, perm) error`:
    - role := RoleFromContext(ctx). Если нет → ErrUnauthorized.
    - Если `perm` ∈ `RolePermissions(role)` → nil. Иначе ErrForbidden.
- [ ] 2. **CODE** Helper `genTokenParts()` в `auth_service.go` или отдельном `auth_token.go`:
  ```go
  const tokenAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
  func genTokenParts() (prefix, secret string, err error) { ... }
  ```
  Использует `crypto/rand.Read` + base62.
- [ ] 3. **GREEN** Создать `internal/core/auth_service_test.go`:
  - mock-структуры `mockUserRepo`, `mockTokenRepo` (in-memory map[uuid.UUID]*User).
  - testClock — простая Clock-имплементация с поднимаемым временем.
  - newService(t) helper — создаёт AuthService с моками и `bcrypt.MinCost`.
  - Тесты:
    - TestAuthService_CreateUser_Success
    - TestAuthService_CreateUser_ShortPassword (< 8 → ErrValidation)
    - TestAuthService_CreateUser_InvalidEmail (regex check)
    - TestAuthService_CreateUser_EmailCollision (через mock-возврат ErrAlreadyExists)
    - TestAuthService_VerifyPassword_Success
    - TestAuthService_VerifyPassword_WrongPassword
    - TestAuthService_VerifyPassword_UserNotFound (→ ErrUnauthorized, not ErrNotFound)
    - TestAuthService_IssueToken_Format (regex assert)
    - TestAuthService_IssueToken_HashCost6
    - TestAuthService_ValidateToken_RoundTrip
    - TestAuthService_ValidateToken_Expired
    - TestAuthService_ValidateToken_BadFormat (counter в mock — verify 0 SQL calls)
    - TestAuthService_AuthorizeOperation_OK / Forbidden / NoRoleInCtx
    - TestAuthService_RevokeToken
- [ ] 4. **VERIFY** `task test ./internal/core/...` GREEN.

NOTE: bcrypt cost для unit-тестов — `bcrypt.MinCost` (4). Проверка cost в `TestAuthService_IssueToken_HashCost6` — секрет хешируется с cost=6 hardcoded в IssueToken (не из конфига).

---

## T-4 — Storage Bundle

***_Complexity: standard_***
***_Requirements: REQ-4.1, REQ-4.2, REQ-4.3, REQ-4.4, REQ-4.5_***
***_Preservation: CP-12_***

GOAL: расширить storage-factory до `Bundle`. Сохранить `OpenAs` как F2-shim.

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/storage/factory.go`:
  - Добавить `type Bundle struct { Posts core.PostRepository; Users core.UserRepository; Tokens core.TokenRepository; Closer io.Closer }`.
  - Новая функция `Open(cfg) (*Bundle, error)`:
    - switch cfg.Storage.Type (как раньше).
    - Для `fs`: `Bundle{Posts: fsRepo, Users: nil, Tokens: nil, Closer: nopCloser{} | gitDecorator}`.
    - Для `sqlite`: `Bundle{Posts: sqliteRepo, Users: sqliteRepo, Tokens: sqliteRepo, Closer: sqliteRepo}` (multi-interface на одной структуре).
    - Для `postgres`: аналогично.
  - `OpenAs(cfg, type) (PostRepository, io.Closer, error)` — обёртка: `b, err := Open с подменённой Type; return b.Posts, b.Closer, err`. Сохраняет F2-API.
  - Все CLI-команды кроме новых auth — через `OpenAs`. Новые auth и `serve.go` — через `Open`.
- [ ] 2. **GREEN** В `internal/adapters/storage/factory_test.go`:
  - TestBundleOpen_FS — `fs` → `Posts != nil`, `Users == nil`, `Tokens == nil`.
  - TestBundleOpen_SQLite — все 3 non-nil.
  - TestBundleOpen_Postgres — пропустить через t.Skip без Docker (или integration tag); в default suite — TestBundleOpen_Postgres_MissingDSN → ErrConfigInvalid.
  - TestBundleClose_Idempotent — close дважды → no error.
- [ ] 3. **VERIFY** `task test ./internal/adapters/storage/... && task build` GREEN.

NOTE: `git decorator` через factory остаётся — он оборачивает только Posts. Для git-режима Bundle.Users остаётся как у inner (если inner = sqlite — Users есть; если inner = fs — nil). Но в F4 git+sqlite/postgres не предполагается → проще: при `Type=fs` decorator оборачивает Posts; Users остаётся nil.

---

## T-5 — HTTP middleware (BearerToken)

***_Complexity: standard_***
***_Requirements: REQ-6.1, REQ-6.2, REQ-6.3, REQ-6.4, REQ-6.5, REQ-6.6_***
***_Preservation: CP-10, CP-11_***

GOAL: реализовать `BearerTokenMiddleware`, интегрировать в `serve.go`.

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/httpapi/middleware.go`:
  - `func BearerTokenMiddleware(svc *core.AuthService) func(http.Handler) http.Handler`:
    - Парсит `Authorization` header. Должен быть `Bearer <token>`. Иначе → 401 + `{"error":"unauthorized"}`.
    - `svc.ValidateToken(ctx, raw)` → если err → 401.
    - Положить User/TenantID/Role в ctx через `core.WithUser`/`WithTenant`/`WithRole`.
    - Передать handler.
  - В существующий `LoggingMiddleware` модифицировать: при логировании запроса — заменить `Authorization` header в логах на `***`.
- [ ] 2. **CODE** В `internal/cli/serve.go`:
  - После открытия Bundle: если `cfg.Auth.Type == "token"`:
    - Создать `authSvc := core.NewAuthService(bundle.Users, bundle.Tokens, cfg.Auth.BCryptCost, core.SystemClock{})`.
    - Подключить `BearerTokenMiddleware(authSvc)` ВМЕСТО `TenantFromConfigMiddleware`.
  - Если `cfg.Auth.Type == "none"`: TenantFromConfigMiddleware (существующее F1-поведение).
- [ ] 3. **GREEN** В `internal/adapters/httpapi/middleware_test.go`:
  - Helper `setupAuthSvc(t)` — in-memory user/token (через `internal/adapters/sqlite` с tempdir), создаёт user + IssueToken, возвращает (svc, validToken, expiredToken).
  - Тесты:
    - TestBearerMiddleware_NoHeader_401
    - TestBearerMiddleware_BadFormat_401 (`Basic xyz`, `Bearer`)
    - TestBearerMiddleware_InvalidToken_401
    - TestBearerMiddleware_ExpiredToken_401
    - TestBearerMiddleware_ValidToken_200_CtxPopulated (handler проверяет ctx.User != nil, ctx.Role == owner).
- [ ] 4. **VERIFY** `task test ./internal/adapters/httpapi/...` GREEN.

NOTE: TenantFromConfigMiddleware остаётся в коде, не удаляется. Просто wiring выбирает.

---

## T-6 — CLI commands (`user`, `token`)

***_Complexity: standard_***
***_Requirements: REQ-7.1, REQ-7.2, REQ-7.3, REQ-7.4, REQ-7.5, REQ-7.6, REQ-7.7, REQ-7.8, REQ-7.9_***
***_Preservation: CP-13, CP-15_***

GOAL: добавить CLI-команды `user` и `token`.

Subtasks:
- [ ] 1. **CODE** Создать `internal/cli/user.go`:
  - `userCmd` с subcommands `create`, `list`, `delete`.
  - `userCreateCmd`: флаги `--email`, `--password`, `--role` (default "author"), `--first-owner` (bool).
    - Открыть Bundle. Если `Bundle.Users == nil` → error "user management requires sqlite/postgres".
    - Если `--first-owner`:
      - count, _ := Bundle.Users.Count(ctx, tenantID)
      - count > 0 → exit 1 "first owner already exists"
      - role = "owner" (override)
    - Иначе count == 0 → exit 1 "no users yet, use --first-owner first"
    - svc.CreateUser(...) → выводим ID, email, role.
  - `userListCmd`: tabwriter с колонками id, email, role, created_at. БЕЗ password_hash.
  - `userDeleteCmd`: arg `<user-id>`. Если user.Role == owner → проверить `CountOwnersByTenant() > 1` иначе exit 1 "cannot delete last owner". Затем Users.Delete (cascade удалит tokens).
- [ ] 2. **CODE** Создать `internal/cli/token.go`:
  - `tokenCmd` с subcommands `create`, `list`, `revoke`.
  - `tokenCreateCmd`: флаги `--user-id`, `--name`, `--expires-in <duration>` (опц.).
    - Открыть Bundle. Если nil — error.
    - svc.IssueToken(ctx, userID, name, expiresIn).
    - Вывод: `Token: jtpat_...\n⚠️  Save this token, it will not be shown again.`
  - `tokenListCmd --user-id <id>`: tabwriter колонки id, name, prefix, created_at, expires_at, last_used_at. БЕЗ secret_hash.
  - `tokenRevokeCmd <token-id>`: tokens.Delete.
- [ ] 3. **CODE** В `internal/cli/root.go`:
  - Добавить `rootCmd.AddCommand(userCmd)` и `tokenCmd`.
- [ ] 4. **GREEN** Создать `internal/cli/user_test.go`:
  - Helper `tempBundle(t) (*storage.Bundle, *config.Config, func())` — sqlite в tempdir, populated config.
  - TestUserCmd_FirstOwner_EmptyDB
  - TestUserCmd_FirstOwner_NonEmptyDB → exit 1
  - TestUserCmd_Create_NoFirstOwner_NoUsers → exit 1
  - TestUserCmd_FS_Rejected (storage.type=fs)
  - TestUserCmd_DeleteLastOwner_Rejected
  - TestUserCmd_List
- [ ] 5. **GREEN** Создать `internal/cli/token_test.go`:
  - TestTokenCmd_Create_OutputContainsRaw
  - TestTokenCmd_List
  - TestTokenCmd_Revoke
  - TestTokenCmd_FS_Rejected
- [ ] 6. **VERIFY** `task test ./internal/cli/... && task build` GREEN.

NOTE: CLI tests запускают cmd через `cmd.SetArgs([]string{...})` и `cmd.Execute()` — pattern из существующих тестов.

---

## T-7 — Финал: тесты, race detector, lint, smoke

***_Complexity: standard_***
***_Requirements: REQ-8.1, REQ-8.2, REQ-8.3, REQ-8.4, REQ-8.5_***
***_Preservation: ALL_***

GOAL: финализация — full sweep + smoke + CHANGELOG/docs.

Subtasks:
- [ ] 1. **VERIFY** `task fmt && task vet && task test && task test:race && task test:integration` GREEN.
- [ ] 2. **VERIFY** `task generate && git diff --exit-code -- internal/adapters/{sqlite/sqlitedb,postgres/pgdb}` — clean.
- [ ] 3. **VERIFY** `task lint` — нет new findings (pre-existing minor могут оставаться).
- [ ] 4. **CODE** Обновить `CHANGELOG.md` секцией F4: новые типы, AuthService, BearerMiddleware, CLI user/token, миграция 0002, breaking change в `auth.type=token` mode.
- [ ] 5. **CODE** Обновить `.jtpost.example.yaml`:
  - Раскомментировать `auth.type` с пояснением `none|token`.
  - Добавить `auth.bcrypt_cost: 10`.
- [ ] 6. **VERIFY** Smoke test:
  - tempdir → `jtpost init --force` → отредактировать `.jtpost.yaml` (`storage.type=sqlite`, `auth.type=token`).
  - `jtpost user create --first-owner --email me@x --password password123 --role owner` → user создан.
  - `jtpost token create --user-id <id> --name cli` → получен PAT.
  - `jtpost user list` → видно user.
  - Запустить `jtpost serve &` → curl `http://localhost:8080/api/posts` без auth → 401; с `Authorization: Bearer <pat>` → 200 (пустой массив).
- [ ] 7. **CODE** Обновить `CHANGELOG.md` финальным описанием smoke-результатов если что-то сюрпризнуло.

---

## T-8 — GATE

***_Complexity: mechanical_***
***_Requirements: ALL_***

CRITICAL: последняя задача.

Instructions:
1. Все T-1..T-7 marked complete через `pipeline.sh task T-N`.
2. Final `task test`, `task test:race`, `task test:integration`, `task build`, `task lint`, `task generate && git diff --exit-code` — все pass.
3. Coverage matrix verified: каждый REQ-X.Y → ≥1 GREEN тест.
4. Если что-то fail — вернуться к ответственной задаче.
