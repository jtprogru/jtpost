# Code Review: web-sessions (F4b)

## Verdict: PASS

Все 33 требования F4b реализованы и покрыты тестами; 16 Correctness Properties прослежены. End-to-end smoke (login → 200+cookie+csrf; GET с cookie → 200; POST без CSRF → 403; logout → 200) подтверждает работающую chain. Lint выдал ~22 finding'а — все minor (включая pre-existing F2/F3/F4a). Ни одного critical/major. Verdict = `PASS`.

## Change Set (summary)

24 файла created/modified. Ключевые:
- ✅ Domain: `core/session.go`, `session_repository.go`, scope helpers, errors.
- ✅ Repository: SQLite + Postgres миграция `0003_sessions.sql`, sqlc-queries, фасад `Sessions()`.
- ✅ Service: AuthService + `Login/Logout/ValidateSession/RefreshCSRF`.
- ✅ Storage Bundle: расширен `Sessions` поле.
- ✅ Middleware: BearerToken (refactored soft-pass), Session, CSRF, RequireAuth.
- ✅ Handlers: `/api/auth/{login,logout,csrf}`.
- ✅ CLI: `serve.go` chain wiring.
- ✅ Config: SessionTTL, CookieSecure, CookieDomain.
- ✅ Tests: 17 unit + 5 handler + e2e.
- ✅ CHANGELOG, .jtpost.example.yaml.

Ни одного unexpected/missing файла.

## Requirements Traceability

Все 33 REQ покрыты ≥1 тестом + кодом + CP. Сжатая матрица:

| Group | REQ | Tests | Code | CP |
|-------|-----|-------|------|-----|
| 1 (Domain) | 1.1..1.4 | TestRolePermissions, type-check | `core/session.go`, `session_repository.go` | CP-1 |
| 2 (AuthService) | 2.1..2.6 | `TestAuthService_Login_*/Logout_*/ValidateSession_*/RefreshCSRF` | `core/auth_service.go` | CP-1..CP-4, CP-10 |
| 3 (Repo) | 3.1..3.3 | `TestSQLiteSessionRepo_*` (5 tests) | `sqlite/sessions.go`, `postgres/sessions.go`, `0003_*.sql` | CP-14 |
| 4 (SessionMW) | 4.1..4.6 | `TestSession*`, `TestBearer*` (8 tests) | `middleware.go:SessionMiddleware/BearerToken/RequireAuth` | CP-5, CP-16 |
| 5 (CSRFMW) | 5.1..5.5 | `TestCSRFMiddleware_*` (6 tests) | `middleware.go:CSRFMiddleware` | CP-6, CP-7, CP-8, CP-9 |
| 6 (Handlers) | 6.1..6.7 | `TestLoginHandler_*/Logout/CSRF/E2E` (5 tests + e2e) | `auth_handlers.go` | CP-10..CP-13 |
| 7 (Bundle+Config) | 7.1..7.5 | `TestOpenBundle_*`, `TestConfig_Validate_*` | `factory.go`, `config.go` | CP-12, CP-15 |
| 8 (Tests) | 8.1..8.6 | сами тесты | — | All |

## Design Conformance

### 3.1 Architectural Boundaries ✅
- Новые типы в правильных пакетах: `core/`, `adapters/{sqlite,postgres,httpapi,storage,config}`, `cli/`.
- Зависимости направлены внутрь. AuthService не знает HTTP. Middleware импортирует только core.

### 3.2 Data Models ✅
- `Session` со полями (Prefix+SecretHash вместо TokenHash — deviation документирована).
- DDL соответствует §2.5: FK CASCADE, UNIQUE prefix, indexes.
- LoginInput/LoginResult — точно по design.

### 3.3 API Contracts ✅
- `AuthService.Login/Logout/ValidateSession/RefreshCSRF` — сигнатуры из design §2.3.
- Middleware-функции — точно по дизайну.
- HTTP endpoints совпадают: `POST /api/auth/{login,logout,csrf}`.

### 3.4 Error Handling ✅
- 13 сценариев из design §2.7 покрыты:
  - Login wrong password → 401.
  - Logout idempotent → 200 + clear cookie.
  - Session expired → soft-pass → final 401.
  - CSRF missing/wrong → 403 csrf_invalid (constant-time compare).
  - CSRF refresh без session → 401.
  - Bearer wins при наличии обоих.

### 3.5 Correctness Properties ✅
Все 16 CP связаны с тестами:
- CP-1 Login round-trip: `TestAuthService_Login_Success` + `ValidateSession_Roundtrip`.
- CP-2 Wrong password rejected: `TestAuthService_Login_WrongPassword`.
- CP-3 Logout invalidates: `TestAuthService_Logout`.
- CP-4 Expired rejected: `TestAuthService_ValidateSession_Expired`.
- CP-5 Bearer wins: `TestSessionMiddleware_BearerWinsOverSession`.
- CP-6 GET bypass CSRF: `TestCSRFMiddleware_GET_Bypass`.
- CP-7 Bearer-auth bypass CSRF: `TestCSRFMiddleware_BearerPOST_NoCSRF_Pass`.
- CP-8 Session POST требует CSRF: `TestCSRFMiddleware_SessionPOST_*` (Valid/Missing/Wrong).
- CP-9 Login skips CSRF: `TestCSRFMiddleware_LoginEndpoint_Bypass`.
- CP-10 Login revokes existing: e2e/handler.
- CP-11 Logout idempotent: `TestLogoutHandler_NoCookie_Idempotent`.
- CP-12 CSRF refresh requires session: `TestCSRFHandler_NoSession_401`.
- CP-13 Cookie attributes: `TestLoginHandler_Success` (HttpOnly + SameSite check).
- CP-14 Cascade delete: `TestSQLiteSessionRepo_CascadeDelete`.
- CP-15 SessionTTL validation: `TestConfig_Validate_SessionTTL` (pre-existing F4a coverage).
- CP-16 RequireAuth final gate: `TestSessionMiddleware_NoCookie_FullChain_401`.

### 3.6 Documentation Consistency ✅
Mermaid в design §2.2 отражает реальную структуру. Имена соответствуют коду.

## Code Quality

### 4.1 Naming & Clarity ✅
- `SessionCookieName` = "jtpost_session" const.
- `CSRFHeaderName` = "X-Csrf-Token" (canonical form для lint).
- `AuthSourceBearer` / `AuthSourceSession` — typed string constants.

### 4.2 Dead Code ✅
- F1 `TenantFromConfigMiddleware` сохранён для backward-compat (не удалён).
- BearerTokenMiddleware refactored из 401-on-fail в soft-pass — это intentional, документировано в комментарии.

### 4.3 Scope Creep ⚠ minor
- Deviation: token format `prefix+secret_hash` вместо `token_hash` (design §2.5). Документировано.
- T-2 added bonus query `ListSessionsByUser` for future-proofing.

### 4.4 Test Quality ✅
- Все тесты используют `t.TempDir()` для изоляции.
- Helpers (`fullChain`, `setupSession`, `setupHandler`) переиспользуются.
- E2E тест проверяет полный stack (login → cookie → protected handler).
- Constant-time compare проверен косвенно (CSRF reject при mismatch).

## Security

✅ Security-issues — none critical/major.

- **CSRF**: double-submit pattern с `subtle.ConstantTimeCompare`. Bearer-only requests CSRF-immune (правильное решение). Login/csrf endpoints в skip-list (intentional — login не имеет session ещё).
- **Cookie attributes**: HttpOnly (всегда), Secure (configurable), SameSite=Lax (баланс защиты/UX), Path=/, Expires set. Domain — опционален.
- **Session token entropy**: 32 random bytes из crypto/rand → ~190-bit base62. bcrypt cost=4 — secret entropy уже достаточная.
- **Session fixation**: Login revoke'ает существующую session перед созданием новой (intentional).
- **Logout idempotent**: 200 + clear cookie даже без auth — UX-priority.
- **No session leakage**: VerifyPassword (для login) → ErrUnauthorized универсально; не утечка существования email.
- **Cascade delete**: при DeleteUser, sessions автоматически удалены через FK ON DELETE CASCADE.
- **No sensitive logging**: Logging middleware не логирует Authorization/Cookie headers.
- **CSRF в API-only сценарии**: skip когда auth.source != session — Bearer-токен в API immune to CSRF.
- **Token format validation**: regex check ДО SQL-запроса (no DB hit on garbage tokens).

## Verification Evidence

- **Tests** (`go test ./...`):
```
ok  	github.com/jtprogru/jtpost/internal/adapters/config	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/fsrepo	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/gitrepo	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/httpapi	0.731s
ok  	github.com/jtprogru/jtpost/internal/adapters/sqlite	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/storage	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegram	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegramconv	(cached)
ok  	github.com/jtprogru/jtpost/internal/cli	0.815s
ok  	github.com/jtprogru/jtpost/internal/core	(cached)
ok  	github.com/jtprogru/jtpost/internal/logger	(cached)
```
- **Race**: 11 пакетов GREEN, no data races.
- **Build**: `task build` OK.
- **Lint**: ~22 finding'а, все minor (pre-existing F2/F3/F4a + новые косметические в F4b).
- **Smoke e2e**:
```
$ POST /api/auth/login {email, password} → 200
   Set-Cookie: jtpost_session=jts_kdafmaEX_bLtuHbivFcdjTZHUyaXURB4CfQ8ZlpXA; HttpOnly; SameSite=Lax
   {"csrf_token":"3F7mLuYrvjTaIOCaMkQEpfpLFnFbcLXJ", ...}
$ GET /api/posts (cookie) → 200
$ POST /api/posts (cookie, NO CSRF) → 403  ← правильно блокирован
$ POST /api/auth/logout → 200 + clear cookie
```

## Findings

| ID | Severity | File | Description | Requirement |
|----|----------|------|-------------|-------------|
| F-1 | minor | `internal/core/auth_service.go:343` | Async LastUsedAt update use `context.Background()` — gosec G118. Intentional (detached lifecycle). Уже имеет `//nolint:contextcheck`. | REQ-2.5 |
| F-2 | minor | `auth_handlers_test.go:150` | unnecessary conversion (unconvert). Косметическое. | — |
| F-3 | minor | `auth_handlers_test.go:22` | setupHandler returns *User unused (unparam). Можно убрать return value. | — |
| F-4 | minor | `sessions_test.go:15` | `newRepoWithSessions` returns *PostRepository unused. Косметическое. | — |
| F-5..F-22 | minor | прочие | Pre-existing lint от F2/F3/F4a (nilnil, funcorder, gochecknoglobals, и т.д.). | — |

## Recommendations

**Minor (follow-up):**
1. F-1: переместить `//nolint:gosec` директиву рядом с goroutine (вместо `//nolint:contextcheck`).
2. F-2..F-4: косметические правки в test helpers.

Все — кандидаты в follow-up PR; не блокируют PASS.

## Pipeline state

8/8 задач T-1..T-8 marked complete. Артефакт зарегистрирован.
