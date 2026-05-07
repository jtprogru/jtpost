# Implementation Report: Auth/RBAC Foundation (F4)

## Summary

F4 реализована полностью: 8 задач плана выполнены, 33 требования покрыты тестами, 16 Correctness Properties прослежены в коде. Bcrypt password hashing + PAT (`jtpat_<8>_<24>` server-side hashed lookup) + RBAC scaffold (4 роли × 6 permissions) + BearerTokenMiddleware + CLI commands. End-to-end smoke test (init → first-owner → token → curl): без auth → 401, с PAT → 200.

## Task Execution

- [x] **T-1** Foundation: domain types + errors + scope helpers + config validation — GREEN
- [x] **T-2** Repository: миграции 0002, sqlc, sqlite/postgres адаптеры (subagent) — GREEN. Deviation: композиция через `*PostRepository.Users()`/`.Tokens()` фасады вместо multi-interface (Go не позволяет одинаковые имена методов с разными сигнатурами).
- [x] **T-3** AuthService с mock-tests — GREEN (15 тестов: CreateUser, VerifyPassword, IssueToken format/cost, ValidateToken roundtrip/expired/badformat-no-SQL, RevokeToken, AuthorizeOperation matrix)
- [x] **T-4** Storage Bundle (`OpenBundle(cfg) (*Bundle, error)` с `Posts/Users/Tokens/Closer`) — GREEN. F2 `OpenAs` сохранён как shim.
- [x] **T-5** BearerTokenMiddleware + serve.go wiring — GREEN (4 middleware-теста)
- [x] **T-6** CLI: `jtpost user create/list/delete` (с `--first-owner` bootstrap, last-owner protection), `jtpost token create/list/revoke` — GREEN
- [x] **T-7** Финал — все checks GREEN
- [x] **T-8** GATE — fmt+vet+test+race+integration+generate+build все pass; smoke test e2e работает

## Final Verification

- **Tests** (`go test ./...`):
```
ok  	github.com/jtprogru/jtpost/internal/adapters/config	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/fsrepo	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/gitrepo	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/httpapi	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/sqlite	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/storage	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/cli	(cached)
ok  	github.com/jtprogru/jtpost/internal/core	(cached)
```

- **Race** (`go test -race ./...`): 11 пакетов GREEN, no data races.
- **Integration** (`go test -tags=integration ./internal/adapters/postgres/...`): SKIP без Docker, GREEN с Docker.
- **Build**: `task build` OK.
- **Generate**: sqlc actualized models.go (добавил User/Token types) — committed.

- **Smoke test e2e**:
```
$ jtpost init --force
$ # set storage.type=sqlite, auth.type=token in .jtpost.yaml
$ jtpost user create --first-owner --email me@example.com --password password123
✅ User created  ID: 019e0193-a715-...  Role: owner
$ jtpost token create --user-id <id> --name cli-smoke
🔑 Token: jtpat_L9RUkRcg_LsseGFNpeofY5s58euTkMk7H
$ jtpost serve --port 9911 &
$ curl http://localhost:9911/api/posts            → HTTP 401
$ curl -H "Authorization: Bearer jtpat_..." ...   → HTTP 200, []
```

## Files Changed

**Created:**
- `internal/core/user.go`, `auth_service.go`, `user_repository.go`, `auth_service_test.go`
- `internal/adapters/{sqlite,postgres}/migrations/0002_users_tokens.sql`
- `internal/adapters/{sqlite,postgres}/queries/users.sql`, `tokens.sql`
- `internal/adapters/{sqlite,postgres}/{users,tokens}.go`, `{users,tokens}_test.go`
- `internal/adapters/{sqlite/sqlitedb,postgres/pgdb}/{users,tokens}.sql.go` (sqlc-generated)
- `internal/adapters/httpapi/bearer_test.go`
- `internal/cli/user.go`, `token.go`

**Modified:**
- `internal/core/errors.go` (ErrUnauthorized, ErrForbidden), `scope.go` (WithUser/Role)
- `internal/adapters/config/config.go` (BCryptCost, Validate extension)
- `internal/adapters/storage/factory.go` (Bundle + OpenBundle)
- `internal/adapters/httpapi/middleware.go` (BearerTokenMiddleware)
- `internal/cli/serve.go` (Bundle wiring + middleware switch)
- `internal/cli/root.go` (userCmd, tokenCmd)
- `internal/adapters/{sqlite/sqlitedb,postgres/pgdb}/models.go` (sqlc-actualized)
- `CHANGELOG.md`, `.jtpost.example.yaml`
- `go.mod`, `go.sum` (golang.org/x/crypto)

## Deviations

- **T-2** Multi-interface на одном `*PostRepository` оказался невозможен в Go (одинаковые имена методов с разными сигнатурами). Реализация через композицию: отдельные структуры `*sqlite.UserRepository`/`*postgres.UserRepository` + фасадные методы `(*PostRepository).Users()` / `.Tokens()` возвращают их. ADR-2 цель (один pool на процесс) сохранена.
- **T-1** не написал `internal/core/user_test.go` отдельно — coverage RolePermissions перешёл в `auth_service_test.go` (TestRolePermissions table-driven).
- **T-7** не обновил `.jtpost.example.yaml` отдельно (сделано в T-8 GATE).

## Pipeline state

8/8 задач T-1..T-8 marked complete. Артефакт регистрируется ниже.
