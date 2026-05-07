# Implementation Report: Web Sessions + CSRF (F4b)

## Summary

F4b реализована полностью: 8 задач плана выполнены. Cookie-based sessions (`jtpost_session=jts_<8>_<32>` с HttpOnly+Secure+SameSite=Lax+Path=/) + CSRF middleware (double-submit, `subtle.ConstantTimeCompare`) + composable middleware chain (Bearer || Session → CSRF → RequireAuth) + login/logout/csrf endpoints. Bearer-PAT (F4a) работает параллельно с cookie-sessions; Bearer wins при наличии. CSRF — только для session-auth state-changing requests; Bearer-only клиенты CSRF-immune.

## Task Execution

- [x] **T-1** Domain types + scope helpers + Config validation — GREEN
- [x] **T-2** Migration 0003_sessions.sql + sqlc + sqlite/postgres адаптеры (subagent) — GREEN
- [x] **T-3** AuthService.Login/Logout/ValidateSession/RefreshCSRF + 7 mock-tests — GREEN
- [x] **T-4** Bundle.Sessions wiring — GREEN
- [x] **T-5** Middleware: Session/CSRF/RequireAuth + soft-pass refactor BearerToken — GREEN (12 middleware-tests)
- [x] **T-6** Auth handlers (login/logout/csrf) + serve.go chain wiring — GREEN (5 handler-tests + e2e)
- [x] **T-7** CHANGELOG + .jtpost.example.yaml — done
- [x] **T-8** GATE — fmt+vet+test+race+build все pass; smoke e2e работает (login → 200 + cookie + csrf; GET с cookie → 200; POST без CSRF → 403; logout → 200)

## Final Verification

- **Tests** (`go test ./...`): 11 пакетов GREEN.
- **Race** (`go test -race ./...`): 11 пакетов GREEN.
- **Build**: `task build` OK.

- **Smoke e2e**:
```
$ jtpost user create --first-owner --email me@x.com --password password123 → owner
$ jtpost serve & 
$ curl -X POST /api/auth/login {email, password} → 200
   Set-Cookie: jtpost_session=jts_kdafmaEX_bLtuHbivFcdjTZHUyaXURB4CfQ8ZlpXA; HttpOnly; SameSite=Lax
   body: {"csrf_token":"3F7mLuYrvjTaIOCaMkQEpfpLFnFbcLXJ", ...}
$ curl -b cookie /api/posts → 200
$ curl -b cookie -X POST /api/posts (БЕЗ CSRF header)→ 403  ← correct CSRF block
$ curl -b cookie -X POST /api/auth/logout → 200
```

## Files Changed

**Created:**
- `internal/core/session.go`, `session_repository.go`
- `internal/adapters/{sqlite,postgres}/migrations/0003_sessions.sql`
- `internal/adapters/{sqlite,postgres}/queries/sessions.sql`
- `internal/adapters/{sqlite,postgres}/sessions.go`, `sessions_test.go`
- `internal/adapters/{sqlite/sqlitedb,postgres/pgdb}/sessions.sql.go` (sqlc-generated)
- `internal/adapters/httpapi/auth_handlers.go`, `session_test.go`, `auth_handlers_test.go`

**Modified:**
- `internal/core/auth_service.go` (Login/Logout/ValidateSession/RefreshCSRF + sessions field в struct + sessionFormatRegex)
- `internal/core/auth_service_test.go` (mockSessionRepo + 8 session-tests)
- `internal/core/scope.go` (WithSession, WithAuthSource + getters; AuthSource type)
- `internal/adapters/storage/factory.go` (Bundle.Sessions field)
- `internal/adapters/httpapi/middleware.go` (BearerTokenMiddleware soft-pass + SessionMiddleware + CSRFMiddleware + RequireAuthMiddleware)
- `internal/adapters/httpapi/server.go` (AuthService field, register /api/auth/* routes)
- `internal/adapters/httpapi/bearer_test.go` (authChain helper)
- `internal/adapters/config/config.go` (Auth.SessionTTL, Server.CookieSecure, Server.CookieDomain + Validate)
- `internal/cli/serve.go` (chain wiring при auth.type=token)
- `internal/cli/{user,token}.go` — `core.NewAuthService` обновлены под новую сигнатуру (передают `bundle.Sessions`)
- `CHANGELOG.md`, `.jtpost.example.yaml`

## Deviations

- **T-3 design deviation**: token_hash → prefix+secret_hash (зеркалит APIToken pattern, O(1) lookup). Документировано в task-plan.md NOTE и применено в migration.
- **T-2 deviation**: `ListSessionsByUser :many` query добавлен в обе схемы как future-proofing (не используется в F4b core, но полезен для будущей "Logout from all devices" UI).

## Pipeline state

8/8 задач T-1..T-8 marked complete. Артефакт регистрируется ниже.
