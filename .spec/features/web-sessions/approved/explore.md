# Exploration: Web Sessions + CSRF (F4b)

## Intent

F4b — следующая часть B.2 из DEVELOPMENT_PLAN после F4a (local users + PAT + RBAC scaffold). Текущая F4a-инфраструктура поддерживает только **stateless** API-доступ через `Authorization: Bearer jtpat_...`. Для Web UI (F8) нужна **stateful** сессионная аутентификация: cookie + CSRF + login endpoint.

**Scope F4b:**

1. Доменная модель `Session` (id, user_id, token_hash, csrf_token, expires_at, created_at, last_used_at) и интерфейс `SessionRepository`.
2. SQLite + Postgres адаптеры реализуют SessionRepository через миграцию `0003_sessions.sql`.
3. `AuthService` расширяется методами `Login(email, password)` (создаёт session), `Logout(sessionID)` (revokes), `ValidateSession(token)` (returns User+Role).
4. **Cookie-based session middleware** (`SessionMiddleware`) — альтернатива BearerTokenMiddleware. Извлекает session-cookie, валидирует, кладёт User/Role в context. **Composable**: BearerTokenMiddleware OR SessionMiddleware (любой из двух source).
5. **CSRF middleware** — для не-GET/HEAD/OPTIONS методов проверяет `X-CSRF-Token` заголовок против session.csrf_token.
6. HTTP endpoints: `POST /api/auth/login` (email+password → session cookie set + CSRF token в response body), `POST /api/auth/logout` (invalidate). `POST /api/auth/csrf` (refresh CSRF token).
7. `Config.Auth.SessionTTL` (default 24h), `Config.Server.CookieSecure` (default true), `Config.Server.CookieDomain` (опц.).

**Чего F4b НЕ делает:**

- Не реализует OAuth2 — отдельная фича `F4c-oauth-github` поверх F4b.
- Не реализует Argon2id — отложен в F4c (один миграционный шаг bcrypt → argon2id).
- Не делает Web UI — F8.
- Не делает password reset / email verification — нужен email infra.
- Не делает 2FA / TOTP — отдельная фича позже.
- Не делает session sharing across devices / device tracking — отложено.
- Не делает sliding session expiration (auto-extend on use) — отложено; в F4b — fixed TTL.

**Триггер:** F4a даёт PAT для CLI/API; для будущего Web UI (F8) нужны cookies. F4c OAuth callback тоже потребует session (для CSRF state). F4b — обязательная foundation.

---

## Investigation

### Что уже есть после F4a

**`internal/core/auth_service.go`** имеет 6 методов: CreateUser, VerifyPassword, IssueToken, ValidateToken, RevokeToken, AuthorizeOperation. F4b добавит Login/Logout/ValidateSession.

**`internal/core/scope.go`** имеет `WithUser/WithRole`. F4b использует те же helpers.

**`internal/adapters/httpapi/middleware.go`** имеет `BearerTokenMiddleware(svc)` (F4a) и legacy `TenantFromConfigMiddleware`. F4b добавит `SessionMiddleware` и `CSRFMiddleware`.

**`internal/cli/serve.go`** при `auth.type=token` подключает Bearer. F4b: при `auth.type=token` — chain Bearer + Session (Bearer первым, Session как fallback). При других значениях — F1-stub.

**Storage Bundle** уже extensible: `Bundle{Posts, Users, Tokens, Closer}`. F4b добавит `Sessions core.SessionRepository`.

### Зависимости

- **`crypto/rand`** + **`crypto/subtle`** — random token generation + constant-time comparison.
- **Cookie-handling**: стандартный `net/http` (Set-Cookie, http.Cookie struct).
- **CSRF**: double-submit-cookie pattern (CSRF-token хранится в session + возвращается клиенту в response body; клиент включает в `X-CSRF-Token` header). Альтернатива — encrypted-token, но для F4b — простой подход.
- НЕТ новых внешних библиотек (gorilla/csrf, gorilla/sessions — не нужны при right-sized scope).

### Тестовый контекст

- httpapi-тесты с `httptest.NewRecorder` + Set-Cookie inspection.
- CSRF-тесты через `req.Header.Set("X-CSRF-Token", ...)`.
- Session expiry — через fakeClock (как в F4a auth_service_test).

### Архитектурные точки

- `internal/core/session.go` — `Session` type, `SessionRepository` interface.
- `internal/core/auth_service.go` — расширить Login/Logout/ValidateSession.
- `internal/adapters/{sqlite,postgres}/sessions.go` + queries + миграция 0003.
- `internal/adapters/storage/factory.go` — `Bundle.Sessions` поле.
- `internal/adapters/httpapi/middleware.go` — `SessionMiddleware`, `CSRFMiddleware`.
- `internal/adapters/httpapi/auth_handlers.go` — `POST /api/auth/login`, `logout`, `csrf`.
- `internal/cli/serve.go` — wiring middleware chain.
- `internal/adapters/config/config.go` — новые поля.

### Что F4b НЕ затрагивает

- F4a `BearerTokenMiddleware` остаётся работать (composable chain).
- CLI команды `user`/`token` без изменений.
- `gitrepo` decorator без изменений.
- Existing API endpoints остаются — `SessionMiddleware` просто populates ctx так же как Bearer.

---

## Build Tooling

- **Orchestrator:** Task (без изменений).
- **Test/Build/Lint/Generate:** `task test`, `task build`, `task lint`, `task generate`.
- **Source:** `Taskfile.yml`, `sqlc.yaml`.

CI: без изменений.

---

## Options Considered

### Option A: Cookie-only sessions (без CSRF в F4b)

Cookies + login/logout endpoints. CSRF откладывается в F4c.

- **Pros:** меньше scope, быстрее MVP.
- **Cons:** небезопасно для prod (CSRF атаки тривиальны без mitigation). F4c может слить с OAuth — это отложит Web UI.
- **Сложность:** Low. **Безопасность: Low.**

### Option B: Cookie + CSRF (double-submit pattern) — recommended

Cookies + CSRF token в session + double-submit pattern (X-CSRF-Token header).

- **Pros:** complete foundation for Web UI без security debt; standard pattern.
- **Cons:** немного больше кода. CSRF возможно избыточен в pure JSON API (Bearer-токен в API immune to CSRF), но Web UI с cookies нуждается.
- **Сложность:** Medium.

### Option C: Encrypted/signed JWT cookies

JWT token в cookie вместо server-side session.

- **Pros:** stateless — нет SessionRepository.
- **Cons:** revocation требует blacklist; complicated key rotation; для F4b более сложно чем server-side. Не соответствует F4a-pattern (server-side PAT lookup).
- **Сложность:** Medium-High.

### Option D: Sessions external (Redis)

Хранить sessions в Redis вместо БД.

- **Pros:** быстрее (in-memory).
- **Cons:** дополнительная зависимость (Redis); не подходит для single-binary deploy CLI; усложняет setup.
- **Сложность:** High (infra).

---

## Constraints & Risks

### Backward compatibility

- F4a deployments с `auth.type=token` не ломаются: BearerTokenMiddleware остаётся, SessionMiddleware добавляется в chain параллельно. Если запрос имеет Bearer — Bearer wins; иначе пробуется Session-cookie.
- Deployments с `auth.type=none` не ломаются (F1-stub middleware).

### Security

- **Cookie attributes**: `HttpOnly` (всегда), `Secure` (default true; конфигурируемо для local dev), `SameSite=Lax` (хорошее baseline для Web UI), `Domain` (из конфига; default — current request host).
- **CSRF**: double-submit. Server присылает CSRF-token в login response body; client сохраняет (localStorage / state); включает в `X-CSRF-Token` для каждого state-changing request. Server сравнивает с session.csrf_token (constant-time).
- **Session token entropy**: 32 bytes из crypto/rand → base64 (44 chars). Хешируется через bcrypt (cost=4) для server-side хранения — да, низкий cost достаточен (32-byte secret имеет ~256-bit entropy, bcrypt здесь только защищает от leak БД).
- **Session fixation**: при login генерируется новый session-token (старый невалиден). После logout — hard delete.
- **Session expiry**: fixed TTL (default 24h). После expiry — middleware → 401.
- **Concurrent logins**: разрешены (multiple sessions per user). Logout удаляет только current session.
- **Token in URL/logs**: cookies НЕ в URL. Header `X-CSRF-Token` НЕ должен логироваться. LoggingMiddleware маскирует.

### Performance

- SessionMiddleware на каждый request: SELECT по session-token-hash. Аналогично F4a Bearer (lookup-by-prefix). Cost=4 bcrypt = ~5ms — приемлемо.
- Caching отложено (как и в F4a).

### Edge cases

- **Logged-in + Bearer header**: Bearer wins (priority). Session ignored.
- **Cookie без CSRF token в state-changing request**: 403 Forbidden.
- **Session без user (cascade race)**: ValidateSession → 401, очистить cookie.
- **Multiple tabs / concurrent CSRF**: один CSRF-token per session — все tabs используют тот же. ОК.
- **Login без storage.type=sqlite|postgres**: 500 / config error (как F4a `auth.type=token` requires SQL).
- **CSRF token rotation**: F4b не делает (один token per session, до logout). В F4c можно ввести rotation после login.

---

## Recommended Direction

**Option B (Cookie + CSRF, double-submit)**:

1. **Domain** — `internal/core/session.go`, `SessionRepository` interface, `LoginInput`/`LoginResult` DTOs.
2. **Schema migration** — `0003_sessions.sql` для sqlite + postgres (`sessions(id, user_id, token_hash, csrf_token, created_at, expires_at, last_used_at)`, FK CASCADE).
3. **sqlc queries** — sessions (CreateSession, GetSessionByTokenHash, DeleteSession, DeleteSessionsByUser, UpdateSessionLastUsedAt).
4. **Adapters** — sqlite/postgres реализуют SessionRepository (через `*PostRepository.Sessions()` фасад).
5. **AuthService extension** — `Login(ctx, tenantID, email, password) (*LoginResult, error)`, `Logout(ctx, sessionID)`, `ValidateSession(ctx, rawToken) (*User, Role, *Session, error)`.
6. **Middleware** — `SessionMiddleware(authService)` извлекает cookie `jtpost_session=<token>` → ValidateSession → ctx populated. **Compose**: helper `AuthChain(...)` → tries Bearer first, then Session.
7. **CSRFMiddleware** — для методов != GET/HEAD/OPTIONS: проверяет `X-CSRF-Token` против `session.csrf_token` (constant-time через `subtle.ConstantTimeCompare`). Действует ТОЛЬКО когда session активна (Bearer-only requests CSRF-immune).
8. **HTTP handlers** — `POST /api/auth/login` (body: `{"email": "...", "password": "..."}` → 200 Set-Cookie + body `{"csrf_token": "..."}`), `POST /api/auth/logout`, `POST /api/auth/csrf` (refresh — generate new csrf in same session).
9. **Storage Bundle** — добавить `Sessions core.SessionRepository`.
10. **Config** — `auth.session_ttl` (default 24h), `server.cookie_secure` (default true), `server.cookie_domain` (опц).
11. **CLI serve.go** — wiring chain при `auth.type=token`: Bearer → Session → CSRF.

---

## Scope Boundaries

### Must-have (F4b)

- Session domain + repository + миграция.
- `AuthService.Login/Logout/ValidateSession`.
- `SessionMiddleware` + composable с Bearer.
- `CSRFMiddleware` (double-submit).
- HTTP handlers: login/logout/csrf-refresh.
- Storage Bundle расширен.
- Config `auth.session_ttl`, `server.cookie_secure`, `server.cookie_domain`.
- Tests: AuthService Login/Logout/Validate, SessionMiddleware accept/reject/expired/missing, CSRFMiddleware accept/reject, login/logout endpoints e2e.

### Deferred

- **F4c**: OAuth GitHub provider, Argon2id password hashing.
- **F8**: Web UI с login form.
- **F11**: audit log на login events, sliding session expiration, device tracking, 2FA.
- Per-channel RBAC (нужны channels).
- Email verification / password reset.

### Needs spike

- **CSRF в API-only сценарии**: Bearer-only клиенты не должны CSRF-проверяться. F4b: CSRFMiddleware skip если в ctx нет session-source-flag. Решение в Design: маркер `core.WithAuthSource(ctx, "session"|"bearer")`.

---

## Assumptions & Open Questions

### Assumptions

- `[ASSUMPTION: cookie session token = 32 bytes from crypto/rand → base64]` — стандарт.
- `[ASSUMPTION: server-side хранится bcrypt(cookie_token, cost=4)]` — secret уже high-entropy.
- `[ASSUMPTION: CSRF token = 32 bytes random, stored plaintext в session, double-submit pattern]` — простой и достаточный.
- `[ASSUMPTION: Один CSRF token per session, без rotation в F4b]`.
- `[ASSUMPTION: sliding expiration отложен — в F4b fixed TTL]`.
- `[ASSUMPTION: cookie attributes: HttpOnly+Secure+SameSite=Lax, Domain из конфига]`.
- `[ASSUMPTION: Bearer wins над Session при наличии обоих в request]`.
- `[ASSUMPTION: CSRF проверяется ТОЛЬКО когда auth-source = session]`.
- `[ASSUMPTION: login требует storage.type=sqlite|postgres (как F4a)]`.

### Open Questions (для Requirements)

1. **Cookie name**: `jtpost_session` или `_jtpost_session`? Предложение: `jtpost_session`.
2. **CSRF response location**: в response body как JSON `{csrf_token}` или в response header `X-CSRF-Token`? Предложение: оба (body для login response, header для refresh).
3. **Logout идемпотентен?** Если cookie невалиден — возвращать 200 (no-op) или 401? Предложение: 200 always (UX-priority — clear cookie).
4. **`/api/auth/csrf` без существующей session**: 401 или генерация anonymous CSRF? Предложение: 401 (CSRF не имеет смысла без session).
5. **session_ttl минимум/максимум**: 5 минут .. 30 дней? Предложение: range (5m, 720h) при Validate.
6. **При login если уже есть session в cookie**: revoke старую и выдать новую? Предложение: ДА (security best practice).
7. **CSRF проверка для login endpoint**: нужна ли? Login сам устанавливает session — нет необходимости. Предложение: skip CSRF для login (и /api/auth/csrf — он генерирует CSRF).
8. **Path для cookie**: `/` или `/api`? Предложение: `/` (Web UI и API).

---

## Done When

- [x] Codebase прочитан (F4a auth_service, middleware, F4a storage Bundle).
- [x] 4 опции (A/B/C/D).
- [x] Trade-offs.
- [x] Scope boundaries.
- [x] Assumptions tagged.
- [x] Open Questions (8).
- [x] Build tooling (без изменений).
- [ ] Артефакт зарегистрирован.
