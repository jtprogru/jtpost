# Implementation Report: OAuth GitHub + Argon2id (F4c)

## Summary

F4c реализована полностью: 8 задач плана выполнены. Argon2id default password hashing с автоматической миграцией legacy bcrypt-hashes; GitHub OAuth provider с link-by-email pattern; OAuthAccount entity + repository; HTTP handlers для full OAuth flow (initiate → state cookie → callback → session cookie). Backward-compat F4a/F4b сохранён.

## Task Execution

- [x] **T-1** PasswordHasher (Argon2id/Legacy/Multi) + Config — GREEN
- [x] **T-2** AuthService integration с Hasher + IssueSessionForUser + auto-rehash — GREEN
- [x] **T-3** OAuthAccount + repo + миграция (subagent) — GREEN (Postgres integration tests тоже PASS)
- [x] **T-4** GitHubProvider (subagent) — GREEN
- [x] **T-5** OAuthService (BuildAuthorizeURL, HandleCallback, linking) — GREEN (7 tests)
- [x] **T-6** HTTP handlers + serve.go wiring + middleware skip-list — GREEN (5 handler tests)
- [x] **T-7** CHANGELOG + .jtpost.example.yaml — done
- [x] **T-8** GATE — full sweep + race detector clean (после фикса race в mockUserRepo)

## Final Verification

- **Tests** (`go test ./...`): 12 пакетов GREEN.
- **Race** (`go test -race ./...`): GREEN после фикса race в `mockUserRepo` (добавлен `sync.RWMutex`).
- **Build**: `task build` OK.
- **fmt/vet**: pass.

## Files Changed

**Created (12):**
- `internal/core/password_hasher.go`, `password_hasher_test.go`
- `internal/core/oauth_account.go` (subagent T-3)
- `internal/core/oauth_repository.go` (subagent T-3)
- `internal/core/oauth_provider.go` (subagent T-4)
- `internal/core/oauth_service.go`, `oauth_service_test.go`
- `internal/core/oauth_providers/github.go`, `github_test.go` (subagent T-4)
- `internal/adapters/{sqlite,postgres}/migrations/0004_oauth_accounts.sql` (subagent T-3)
- `internal/adapters/{sqlite,postgres}/queries/oauth_accounts.sql` (subagent T-3)
- `internal/adapters/{sqlite,postgres}/oauth_accounts.go`, `oauth_accounts_test.go` (subagent T-3)
- `internal/adapters/{sqlite/sqlitedb,postgres/pgdb}/oauth_accounts.sql.go` (sqlc-generated, subagent T-3)
- `internal/adapters/httpapi/oauth_handlers.go`, `oauth_handlers_test.go`
- `internal/cli/oauth_init.go`

**Modified:**
- `internal/core/auth_service.go` — hasher field вместо bcryptCost, Login auto-rehash, IssueSessionForUser, VerifyPassword отклоняет OAuth-only users
- `internal/core/auth_service_test.go` — sync.Mutex в mockUserRepo для race-safety
- `internal/adapters/storage/factory.go` — Bundle.OAuthAccounts (subagent T-3)
- `internal/adapters/httpapi/middleware.go` — skip-list для `/api/auth/oauth/` в RequireAuth и CSRF
- `internal/adapters/httpapi/server.go` — OAuthService field в ServerConfig + регистрация route
- `internal/adapters/httpapi/{bearer,session,auth_handlers}_test.go` — обновлены под новую сигнатуру NewAuthService
- `internal/cli/{serve,user,token}.go` — обновлены под новую сигнатуру
- `internal/adapters/config/config.go` — Auth.PasswordHasher, Auth.OAuthProviders map
- `CHANGELOG.md`, `.jtpost.example.yaml`
- `go.mod`, `go.sum` — golang.org/x/crypto/argon2, golang.org/x/oauth2

## Deviations

- **MockUserRepo race-fix**: добавлен `sync.RWMutex` для безопасности concurrent rehash в `TestAuthService_Login_RehashLegacy`. Не deviation от scope, а обязательное исправление race detector.
- **`HasherFromConfig`** возвращает `MultiHasher` для всех значений config'а (не различает "argon2id"/"bcrypt"/"auto"). MultiHasher всегда write Argon2id + read both. Это прагматичный выбор: explicit "bcrypt" не имеет смысла для новых passwords (security). Документировать как known behavior.

## Pipeline state

8/8 задач T-1..T-8 marked complete. Артефакт регистрируется ниже.
