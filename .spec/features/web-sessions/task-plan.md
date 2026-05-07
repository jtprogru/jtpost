# Web Sessions + CSRF (F4b) — Task Plan

**Test Style Source:** Tier 2 — `internal/core/auth_service_test.go` (mock repos), `internal/adapters/httpapi/bearer_test.go` (httptest.NewRecorder + Set-Cookie inspection), `internal/adapters/sqlite/users_test.go` (SQL repo CRUD).

**Commands:**

| Action | Command | Source |
|--------|---------|--------|
| Test (unit) | `task test` | Taskfile.yml |
| Test (race) | `task test:race` | Taskfile.yml |
| Test (integration) | `task test:integration` | Taskfile.yml |
| Build | `task build` | Taskfile.yml |
| Lint | `task lint` | Taskfile.yml |
| Generate | `task generate` | Taskfile.yml |

---

## Coverage Matrix

| Requirement | Task(s) | CP |
|-------------|---------|-----|
| REQ-1.x | T-1 | CP-1 |
| REQ-2.x | T-3 | CP-1, CP-2, CP-3, CP-4, CP-10 |
| REQ-3.x | T-2 | CP-14 |
| REQ-4.x | T-5 | CP-5, CP-16 |
| REQ-5.x | T-5 | CP-6, CP-7, CP-8, CP-9 |
| REQ-6.x | T-6 | CP-10, CP-11, CP-12, CP-13 |
| REQ-7.x | T-1 (config), T-4 (Bundle) | CP-15 |
| REQ-8.x | T-7 | All |

---

## Work Type: Pure feature

---

## T-1 — Domain types + scope helpers + Config validation

***_Complexity: mechanical_***
***_Requirements: REQ-1.x, REQ-7.3, REQ-7.4, REQ-7.5_***
***_Preservation: CP-15_***

Subtasks:
- [ ] 1. **CODE** Создать `internal/core/session.go`: `Session`, `LoginInput`, `LoginResult` types.
- [ ] 2. **CODE** Создать `internal/core/session_repository.go`: интерфейс `SessionRepository` с 6 методами.
- [ ] 3. **CODE** В `internal/core/scope.go`:
  - Добавить `sessionKey`, `authSourceKey` в const блок.
  - `WithSession(ctx, *Session)`, `SessionFromContext(ctx) (*Session, bool)`.
  - `WithAuthSource(ctx, source string)`, `AuthSourceFromContext(ctx) (string, bool)`.
- [ ] 4. **CODE** В `internal/adapters/config/config.go`:
  - `AuthConfig.SessionTTL time.Duration` (default 24h в `NewDefaultConfig`).
  - `ServerConfig.CookieSecure bool` (default true), `CookieDomain string` (опц).
  - `Validate()`: при `Auth.Type=="token"` проверять `SessionTTL ∈ [5m, 720h]` (если `SessionTTL > 0`; default уже норма).
  - В `loadFromFile` добавить `SetDefault`/`BindEnv` для `auth.session_ttl`, `server.cookie_secure`, `server.cookie_domain`.
- [ ] 5. **GREEN** В `internal/adapters/config/config_test.go`: `TestConfig_Validate_SessionTTL` (table-driven: 1m, 5m, 24h, 720h, 1000h × token-type).
- [ ] 6. **VERIFY** `go test ./internal/core/... ./internal/adapters/config/...` GREEN.

---

## T-2 — Repository: миграция + sqlc + sqlite/postgres адаптеры

***_Complexity: complex_***
***_Requirements: REQ-3.x_***
***_Preservation: CP-14_***

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/sqlite/migrations/0003_sessions.sql` (DDL из design §2.5).
- [ ] 2. **CODE** Создать `internal/adapters/postgres/migrations/0003_sessions.sql` (Postgres-вариант).
- [ ] 3. **CODE** Создать `internal/adapters/sqlite/queries/sessions.sql`:
  - `CreateSession :exec`, `GetSessionByTokenHash :one`, `DeleteSession :exec`, `DeleteSessionsByUser :exec`, `UpdateSessionLastUsedAt :exec`, `UpdateSessionCSRFToken :exec`.
- [ ] 4. **CODE** Создать `internal/adapters/postgres/queries/sessions.sql` (те же queries, $N плейсхолдеры).
- [ ] 5. **CODE** Запустить `task generate`. Сгенерируется `sessions.sql.go` в обоих pkg.
- [ ] 6. **CODE** Создать `internal/adapters/sqlite/sessions.go`: тип `SessionRepository` + `*PostRepository.Sessions()` фасад.
- [ ] 7. **CODE** Создать `internal/adapters/postgres/sessions.go` аналогично (с pgx-types).
- [ ] 8. **GREEN** Создать `internal/adapters/sqlite/sessions_test.go`: `TestSQLiteSessionRepo_CRUD`, `_GetByTokenHash`, `_CascadeDelete` (DeleteUser → sessions удалены).
- [ ] 9. **GREEN** Создать `internal/adapters/postgres/sessions_test.go` под `//go:build integration`.
- [ ] 10. **VERIFY** `task generate && task test ./internal/adapters/sqlite/...` GREEN.

---

## T-3 — AuthService: Login/Logout/ValidateSession/RefreshCSRF

***_Complexity: standard_***
***_Requirements: REQ-2.x_***
***_Preservation: CP-1, CP-2, CP-3, CP-4, CP-10_***

Subtasks:
- [ ] 1. **CODE** В `internal/core/auth_service.go` добавить поле `sessions SessionRepository` в struct и параметр в `NewAuthService(...)`.
- [ ] 2. **CODE** Реализовать `Login(ctx, in LoginInput, ttl time.Duration)`:
  - `VerifyPassword(ctx, in.TenantID, in.Email, in.Password)` → user.
  - Сгенерировать random session-token (32 bytes из crypto/rand → base64.RawURLEncoding).
  - bcrypt-hash session-token (cost=4) → token_hash.
  - Сгенерировать csrf-token (32 bytes random base64).
  - Создать Session (UUIDv7), `ExpiresAt = clock.Now()+ttl`.
  - `sessions.Create`. На UNIQUE collision retry ≤ 3 раза.
  - Вернуть `LoginResult{RawToken, CSRFToken, Session, User}`.
- [ ] 3. **CODE** Реализовать `Logout(ctx, sessionID)`: `sessions.Delete(ctx, sessionID)` — игнорировать ErrNotFound (idempotent).
- [ ] 4. **CODE** Реализовать `ValidateSession(ctx, raw)`:
  - bcrypt-hash raw — НЕТ, сначала надо найти. Альтернатива: хранить в БД hash из bcrypt-with-fixed-salt? Не работает с cost-bcrypt.
  - **Правильно:** сделать token-format вида `<8-prefix>_<rest>` где prefix indexed; хранить `hash(rest)`. Это копия PAT-pattern из F4a.
  - Перепроектировать: token = `<8 prefix>_<rest32>`; в БД prefix indexed UNIQUE; secret_hash = bcrypt(rest, 4).
  - GetByPrefix → bcrypt.Compare(hash, rest) → expiresAt check → users.GetByID.
  - Async UpdateLastUsedAt.
- [ ] 5. **CODE** Реализовать `RefreshCSRF(ctx, sessionID)`: gen 32-byte random → `sessions.UpdateCSRFToken` → return.
- [ ] 6. **CODE** Обновить `Session` type и schema 0003 — нужны `prefix` и `secret_hash` колонки вместо просто `token_hash`. (Это влияет на T-2; см. note ниже.)
- [ ] 7. **GREEN** В `internal/core/auth_service_test.go` добавить session-тесты:
  - mock-расширить с `SessionRepository` mock.
  - TestAuthService_Login_Success, _WrongPassword, _Roundtrip, _Expired, _BadFormat, Logout_Idempotent, RefreshCSRF.
- [ ] 8. **VERIFY** `go test ./internal/core/...` GREEN.

NOTE: При проектировании я понял что bcrypt-only-hash без prefix даёт O(N) lookup. **Правильный pattern — копия PAT**: token format `<8 prefix>_<32 secret>`, BD: `prefix UNIQUE INDEX` + `secret_hash`. Изменить design.md и T-2 миграцию: `token_hash` → `prefix` + `secret_hash`.

DEVIATION FROM DESIGN: будет применено в реализации.

---

## T-4 — Storage Bundle расширение

***_Complexity: mechanical_***
***_Requirements: REQ-7.1, REQ-7.2_***

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/storage/factory.go`:
  - `Bundle.Sessions core.SessionRepository`.
  - `OpenBundle`: для sqlite/postgres → `Sessions = repo.Sessions()`; для fs → nil.
- [ ] 2. **GREEN** В `factory_test.go`: проверить `Bundle.Sessions != nil` для sqlite, `== nil` для fs.
- [ ] 3. **VERIFY** GREEN.

---

## T-5 — Middleware: Session, CSRF, RequireAuth + chain wiring

***_Complexity: standard_***
***_Requirements: REQ-4.x, REQ-5.x_***
***_Preservation: CP-5, CP-6, CP-7, CP-8, CP-9, CP-16_***

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/httpapi/middleware.go`:
  - Перепроектировать `BearerTokenMiddleware`: при `Authorization: Bearer ...` валидирует и populates ctx с `WithAuthSource(ctx, "bearer")`. На failure — soft-pass (НЕ возвращает 401 сразу).
  - `SessionMiddleware(svc)`: при наличии cookie `jtpost_session=<token>` — `svc.ValidateSession`. На success — populates ctx + `WithAuthSource(ctx, "session")` + `WithSession(ctx, session)`. На failure — soft-pass.
  - `CSRFMiddleware()`: для `r.Method ∈ {POST, PATCH, DELETE, PUT}` И auth-source="session" И path ∉ skipList:
    - извлечь session из ctx; если nil — 403.
    - `subtle.ConstantTimeCompare([]byte(r.Header.Get("X-CSRF-Token")), []byte(session.CSRFToken)) != 1` → 403 csrf_invalid.
  - skipList: `/api/auth/login`, `/api/auth/csrf`.
  - `RequireAuthMiddleware()`: финальный gate. Если `core.UserFromContext(ctx) == nil` → 401.
- [ ] 2. **CODE** В `internal/cli/serve.go` обновить chain при `auth.type=token`:
  - `handler = RecoveryMiddleware(LoggingMiddleware(...))` остаётся вне auth chain.
  - Внутри: `handler = SessionMiddleware(authSvc)(handler)`; `handler = BearerTokenMiddleware(authSvc)(handler)`; `handler = CSRFMiddleware()(handler)`; `handler = RequireAuthMiddleware()(handler)`.
  - **Порядок execution**: BearerTokenMW first (попытается populate ctx), потом SessionMW (если bearer не сработал), потом CSRFMW (проверит если auth=session), потом RequireAuthMW (финальный 401).
  - Wrap login/logout/csrf endpoints чтобы CSRF и RequireAuth для них пропускались (login/csrf/logout endpoints — отдельная sub-chain).
- [ ] 3. **GREEN** В `internal/adapters/httpapi/session_test.go` написать тесты middleware:
  - SessionMW: valid cookie → ctx populated; expired → soft pass + 401 from final; missing → soft pass + 401.
  - BearerWinsOverSession.
  - CSRFMW: GET pass; POST с auth=bearer pass; POST с auth=session valid CSRF pass; POST с auth=session bad CSRF 403; POST к login/csrf — pass.
  - RequireAuthMW: no user → 401.
- [ ] 4. **VERIFY** `task test ./internal/adapters/httpapi/...` GREEN.

---

## T-6 — HTTP handlers: login/logout/csrf

***_Complexity: standard_***
***_Requirements: REQ-6.x_***
***_Preservation: CP-10, CP-11, CP-12, CP-13_***

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/httpapi/auth_handlers.go`:
  - `LoginHandler(svc, cfg) http.HandlerFunc`:
    - parse JSON `{email, password}`.
    - revoke existing session (если cookie присутствует и валиден).
    - `svc.Login(ctx, LoginInput, cfg.Auth.SessionTTL)`.
    - Set-Cookie `jtpost_session=<RawToken>; HttpOnly; Secure=cfg.Server.CookieSecure; SameSite=Lax; Path=/; Expires=...`.
    - response body `{"csrf_token", "user_id", "role", "expires_at"}`.
  - `LogoutHandler(svc) http.HandlerFunc`:
    - извлечь session из ctx (если есть). svc.Logout.
    - Set-Cookie `jtpost_session=; Max-Age=0; HttpOnly; Path=/`.
    - 200.
  - `CSRFHandler(svc) http.HandlerFunc`:
    - session из ctx. nil → 401.
    - svc.RefreshCSRF(ctx, session.ID).
    - body `{"csrf_token": new}` + header `X-CSRF-Token: new`.
- [ ] 2. **CODE** В `internal/adapters/httpapi/server.go` зарегистрировать routes:
  - `mux.HandleFunc("POST /api/auth/login", LoginHandler(...))` и т.д.
  - Эти routes должны обходить RequireAuthMiddleware (login для НЕ-залогиненных).
  - Реализация: либо отдельный mux для /api/auth/* (без RequireAuth), либо логика в RequireAuthMW (skip path).
  - **Решение**: добавить skipList в RequireAuthMW: `/api/auth/login`, `/api/auth/logout`, `/api/auth/csrf` пропускаются (они работают с/без cookie).
- [ ] 3. **GREEN** Расширить `session_test.go` или создать `auth_handlers_test.go`:
  - TestLoginHandler_Success: POST {email, pwd} → 200 + Set-Cookie + body has csrf_token.
  - TestLoginHandler_WrongPassword → 401.
  - TestLoginHandler_RevokesPreviousSession.
  - TestLogoutHandler_NoCookie → 200 + Max-Age=0.
  - TestLogoutHandler_WithCookie → session deleted.
  - TestCSRFHandler_NoSession → 401.
  - TestCSRFHandler_WithSession → 200 + new csrf in body and header.
  - TestE2E_LoginThenProtectedPost: login → cookie+csrf → POST /api/posts с обоими → 200.
- [ ] 4. **VERIFY** `task test ./internal/adapters/httpapi/...` GREEN.

---

## T-7 — Финал + smoke + GATE

***_Complexity: mechanical_***
***_Requirements: ALL_***

Subtasks:
- [ ] 1. **VERIFY** `task fmt && task vet && task test && task test:race && task test:integration` GREEN.
- [ ] 2. **VERIFY** `task generate && git diff --exit-code -- internal/adapters/{sqlite/sqlitedb,postgres/pgdb}` clean.
- [ ] 3. **CODE** Обновить `CHANGELOG.md` секцией F4b.
- [ ] 4. **CODE** Обновить `.jtpost.example.yaml` — `auth.session_ttl`, `server.cookie_secure`, `server.cookie_domain` с пояснениями.
- [ ] 5. **VERIFY** Smoke test e2e:
  - tempdir → init → user create --first-owner.
  - jtpost serve & → curl POST /api/auth/login {email,password} → проверить Set-Cookie + csrf_token.
  - curl POST /api/posts с cookie + X-CSRF-Token → 200.
  - curl POST /api/posts с cookie но без CSRF → 403.
  - curl POST /api/auth/logout → 200 + Max-Age=0.
  - kill server.
- [ ] 6. **VERIFY** `task lint` — нет new findings выше severity minor.

---

## T-8 — GATE

***_Complexity: mechanical_***

CRITICAL: последняя задача. T-1..T-7 завершены, all checks GREEN. Коммит выполняется в follow-up.
