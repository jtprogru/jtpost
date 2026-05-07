# Web Sessions + CSRF (F4b) — Requirements

**Status:** Draft
**Author:** Claude (Opus 4.7) + Mikhail Savin
**Date:** 2026-05-07
**Feature:** web-sessions (F4b)
**Branch:** `feature/web-sessions`

## Overview

F4b расширяет F4a auth-foundation **stateful** сессионной аутентификацией для Web UI: cookie-based sessions + CSRF (double-submit pattern) + login/logout endpoints. PAT-доступ через Bearer-header (F4a) сохраняется и работает параллельно — middleware-chain пробует сначала Bearer, потом session-cookie. CSRF-middleware активируется ТОЛЬКО когда auth-source = session (Bearer-клиенты CSRF-immune). Cookie attributes: `HttpOnly`, `Secure` (configurable), `SameSite=Lax`. Server-side session token хранится как bcrypt-hash (cost=4 — secret уже 256-bit entropy). Новый Bundle.Sessions repo + миграция `0003_sessions.sql`.

## Glossary

| Term | Definition | Code Artifact |
|------|------------|---------------|
| `Session` | id, user_id, token_hash, csrf_token, created_at, expires_at, last_used_at | `internal/core/session.go` |
| `SessionRepository` | Storage interface для sessions | `internal/core/session_repository.go` |
| `LoginInput` | DTO для AuthService.Login (TenantID, Email, Password) | `internal/core/auth_service.go` |
| `LoginResult` | DTO с .RawToken (cookie value), .CSRFToken, .Session, .User | `internal/core/auth_service.go` |
| `SessionMiddleware` | Cookie-based middleware-альтернатива BearerTokenMiddleware | `internal/adapters/httpapi/middleware.go` |
| `CSRFMiddleware` | Double-submit CSRF guard для state-changing methods | `internal/adapters/httpapi/middleware.go` |
| `authSourceKey` | ctxKey для маркера source ("bearer" \| "session") | `internal/core/scope.go` |
| `cookieName` | Константа `"jtpost_session"` | `internal/adapters/httpapi/middleware.go` |
| `csrfHeader` | Константа `"X-CSRF-Token"` | `internal/adapters/httpapi/middleware.go` |
| `sessionCookieCost` | bcrypt cost = 4 (hardcoded; secret entropy уже 256-bit) | `internal/core/auth_service.go` |

## User Stories

- Как **владелец канала**, я хочу залогиниться через Web UI с email+password и получить session-cookie, чтобы не передавать PAT в браузере.
- Как **разработчик-фронтенд**, я хочу получить CSRF-токен в response login-endpoint, чтобы посылать его в `X-CSRF-Token` для state-changing requests.
- Как **API-консьюмер**, я хочу продолжать использовать Bearer-PAT — он не должен ломаться введением sessions.
- Как **владелец**, я хочу logout endpoint, чтобы прервать session и принудительно выдать клиенту истёкшую cookie.
- Как **разработчик-сопровождающий**, я хочу что бы CSRF проверялся ТОЛЬКО для session-аутентификации; Bearer-only клиенты не должны слать CSRF-token.

## Requirements

### Group 1 — Domain types

**REQ-1.1** WHEN модуль `core` определяет тип `Session`, the system SHALL содержать поля `ID uuid.UUID`, `UserID uuid.UUID`, `TokenHash string`, `CSRFToken string`, `CreatedAt time.Time`, `ExpiresAt time.Time`, `LastUsedAt *time.Time`.

**REQ-1.2** WHEN модуль `core` определяет тип `LoginInput`, the system SHALL содержать поля `TenantID uuid.UUID`, `Email string`, `Password string`.

**REQ-1.3** WHEN модуль `core` определяет тип `LoginResult`, the system SHALL содержать `RawToken string` (cookie value, plaintext, показывается клиенту в Set-Cookie), `CSRFToken string`, `Session *Session`, `User *User`.

**REQ-1.4** WHEN модуль `core` экспортирует `SessionRepository`, the system SHALL определять методы: `GetByTokenHash(ctx, hash) (*Session, error)`, `Create(ctx, *Session) error`, `Delete(ctx, id) error`, `DeleteByUser(ctx, userID) error`, `UpdateLastUsedAt(ctx, id, t time.Time) error`, `UpdateCSRFToken(ctx, id, csrf string) error`.

### Group 2 — AuthService extension

**REQ-2.1** WHEN `AuthService.Login(ctx, in LoginInput) (*LoginResult, error)` вызывается, the system SHALL валидировать password через `VerifyPassword`, сгенерировать random session-token (32 bytes base64), bcrypt-hash его (cost=4), сгенерировать CSRF-token (32 bytes base64, plaintext), создать Session с `ExpiresAt = now + cfg.SessionTTL`, сохранить через repository.

**REQ-2.2** WHEN `AuthService.Login` сталкивается с invalid email/password, the system SHALL вернуть `core.ErrUnauthorized` (без leakage).

**REQ-2.3** WHEN `AuthService.Logout(ctx, sessionID uuid.UUID) error` вызывается, the system SHALL hard-delete session через repository. Если session не существует — возвращает `nil` (idempotent).

**REQ-2.4** WHEN `AuthService.ValidateSession(ctx, rawToken string) (*User, Role, *Session, error)` вызывается, the system SHALL bcrypt-hash rawToken, найти session через `GetByTokenHash`, проверить `ExpiresAt > now`, найти соответствующего User, обновить `LastUsedAt` асинхронно. Возвращает `(user, user.Role, session, nil)` или `core.ErrUnauthorized`.

**REQ-2.5** WHEN `AuthService.ValidateSession` валидирует session, the system SHALL обновить `LastUsedAt` через goroutine с `context.Background()` (как REQ-2.7 в F4a).

**REQ-2.6** WHEN `AuthService.RefreshCSRF(ctx, sessionID uuid.UUID) (string, error)` вызывается, the system SHALL сгенерировать новый CSRF-token, сохранить через `UpdateCSRFToken`, вернуть new token.

### Group 3 — Repository + миграция

**REQ-3.1** WHEN goose применяет миграцию `0003_sessions.sql` (sqlite + postgres), the system SHALL создать таблицу `sessions(id PK, user_id NOT NULL, token_hash NOT NULL UNIQUE, csrf_token NOT NULL, created_at NOT NULL, expires_at NOT NULL, last_used_at)` с FK ON DELETE CASCADE на `users(id)`, INDEX(user_id), INDEX(expires_at).

**REQ-3.2** WHEN sqlite/postgres адаптер реализует `core.SessionRepository`, the system SHALL предоставлять `*PostRepository.Sessions()` фасад (как Users()/Tokens() в F4a).

**REQ-3.3** WHEN repository вызывает `GetByTokenHash` и записи нет, the system SHALL вернуть `core.ErrNotFound`.

### Group 4 — SessionMiddleware

**REQ-4.1** WHEN HTTP-сервер инициализируется при `cfg.Auth.Type=="token"`, the system SHALL подключить middleware-chain `Bearer || Session → CSRF`. SessionMiddleware пробуется ПОСЛЕ BearerTokenMiddleware если Bearer не нашёл валидный header.

**REQ-4.2** WHEN `SessionMiddleware` обрабатывает запрос с cookie `jtpost_session=<token>`, the system SHALL вызвать `AuthService.ValidateSession(token)`. При success — положить `WithUser`/`WithTenant`/`WithAuthor`/`WithRole`/`WithSession` в context, продолжить handler. При failure — НЕ возвращать 401 (даём шанс Bearer next; если Bearer тоже не сработал — final middleware возвращает 401).

**REQ-4.3** WHEN запрос содержит и `Authorization: Bearer ...`, и `jtpost_session` cookie, the system SHALL отдавать приоритет Bearer-токену (Session игнорируется).

**REQ-4.4** WHEN `SessionMiddleware` валидирует session, the system SHALL положить `core.WithAuthSource(ctx, "session")` в context (для CSRF-маршрутизации).

**REQ-4.5** WHEN `BearerTokenMiddleware` валидирует PAT, the system SHALL также положить `core.WithAuthSource(ctx, "bearer")`.

**REQ-4.6** WHEN ни Bearer, ни Session не валиден, the system SHALL вернуть HTTP 401 (как сейчас в F4a Bearer middleware).

### Group 5 — CSRFMiddleware

**REQ-5.1** WHEN HTTP-метод запроса ∈ {GET, HEAD, OPTIONS}, the system SHALL пропустить запрос без CSRF-проверки.

**REQ-5.2** WHEN HTTP-метод запроса ∉ {GET, HEAD, OPTIONS} И `core.AuthSourceFromContext(ctx) != "session"`, the system SHALL пропустить запрос без CSRF-проверки (Bearer-only requests CSRF-immune).

**REQ-5.3** WHEN HTTP-метод запроса ∉ {GET, HEAD, OPTIONS} И auth-source = session, the system SHALL извлечь session из ctx, прочитать `X-CSRF-Token` header, сравнить с `session.CSRFToken` через `subtle.ConstantTimeCompare`. Mismatch ИЛИ missing header → HTTP 403 с `{"error":"csrf_invalid"}`.

**REQ-5.4** WHEN CSRF-проверка проходит, the system SHALL передать запрос handler без модификации response.

**REQ-5.5** WHEN HTTP path = `/api/auth/login` ИЛИ `/api/auth/csrf`, the system SHALL пропустить CSRF-проверку (REQ-5.2 не применяется — login устанавливает session, csrf-endpoint его обновляет).

### Group 6 — HTTP endpoints

**REQ-6.1** WHEN HTTP `POST /api/auth/login` принимает body `{"email": "...", "password": "..."}`, the system SHALL вызвать `AuthService.Login(ctx, LoginInput{TenantID: cfg.Auth.TenantDefault, Email, Password})`. При success — установить cookie `jtpost_session=<RawToken>` (HttpOnly, Secure, SameSite=Lax, Path="/", `Expires=session.ExpiresAt`) и вернуть body `{"csrf_token": "...", "user_id": "...", "role": "..."}` со статусом 200.

**REQ-6.2** WHEN HTTP `POST /api/auth/login` сталкивается с invalid email/password, the system SHALL вернуть HTTP 401 с body `{"error":"unauthorized"}`.

**REQ-6.3** WHEN HTTP `POST /api/auth/login` запускается и в request есть валидная session-cookie, the system SHALL revoke старую session (delete) ДО создания новой.

**REQ-6.4** WHEN HTTP `POST /api/auth/logout` принимается с валидной session-cookie, the system SHALL вызвать `AuthService.Logout(ctx, sessionID)` и вернуть HTTP 200 + `Set-Cookie: jtpost_session=; Max-Age=0; HttpOnly; Path=/`.

**REQ-6.5** WHEN HTTP `POST /api/auth/logout` без валидной cookie, the system SHALL вернуть HTTP 200 (idempotent, очистить cookie на клиенте).

**REQ-6.6** WHEN HTTP `POST /api/auth/csrf` принимается с валидной session-cookie, the system SHALL вызвать `AuthService.RefreshCSRF(ctx, sessionID)` и вернуть body `{"csrf_token": "..."}` + header `X-CSRF-Token: <new-token>`.

**REQ-6.7** WHEN HTTP `POST /api/auth/csrf` без session-cookie, the system SHALL вернуть HTTP 401.

### Group 7 — Storage Bundle + Config

**REQ-7.1** WHEN `storage.Bundle` определяется, the system SHALL содержать поле `Sessions core.SessionRepository` (nil для fs).

**REQ-7.2** WHEN `storage.OpenBundle` вызывается с `Storage.Type ∈ {sqlite, postgres}`, the system SHALL заполнить `Sessions = repo.Sessions()`.

**REQ-7.3** WHEN `Config.Auth` определяется, the system SHALL содержать поле `SessionTTL time.Duration` со значением default `24h`.

**REQ-7.4** WHEN `Config.Server` определяется, the system SHALL содержать поля `CookieSecure bool` (default `true`) и `CookieDomain string` (опционально пустой — используется `r.Host`).

**REQ-7.5** WHEN `Config.Validate()` вызывается с `auth.session_ttl < 5*time.Minute || > 720*time.Hour`, the system SHALL вернуть `core.ErrConfigInvalid`.

### Group 8 — Тесты

**REQ-8.1** WHEN `task test` запускается, the system SHALL пройти `TestAuthService_Login_*`, `Logout_*`, `ValidateSession_*`, `RefreshCSRF_*` (через mock-repos).

**REQ-8.2** WHEN `task test` запускается, the system SHALL пройти SQLite SessionRepository CRUD-тесты (Create, GetByTokenHash, Delete, DeleteByUser, UpdateLastUsedAt, UpdateCSRFToken, cascade delete on user delete).

**REQ-8.3** WHEN `task test:integration` запускается, the system SHALL пройти Postgres SessionRepository тесты (зеркало sqlite через testcontainers).

**REQ-8.4** WHEN `task test` запускается, the system SHALL пройти SessionMiddleware тесты: valid cookie → ctx populated; expired cookie → next middleware (Bearer or 401); missing cookie → next; invalid token → next.

**REQ-8.5** WHEN `task test` запускается, the system SHALL пройти CSRFMiddleware тесты: GET → pass; POST с auth=bearer → pass; POST с auth=session + valid CSRF header → pass; POST с auth=session + missing header → 403; POST с auth=session + wrong header → 403; POST к /api/auth/login или /csrf — pass без CSRF.

**REQ-8.6** WHEN `task test` запускается, the system SHALL пройти HTTP integration test login → cookie set → CSRF-protected POST → success.

## Topological Order

```
Group 1 (Domain types) → Group 7 (Config) → Group 3 (Repo+migration) →
Group 2 (AuthService extension) → Group 4 (SessionMiddleware) →
Group 5 (CSRFMiddleware) → Group 6 (HTTP handlers) → Group 8 (тесты финал)
```

## Conflict Priority

**Конфликт 1.** REQ-4.2 (SessionMiddleware при failure не возвращает 401) vs REQ-4.6 (если ни Bearer ни Session — 401).

**Resolution:** middleware chain. Каждый middleware (Bearer, Session) populates ctx если success; если ни один не populated — финальный mark-as-required middleware возвращает 401. Реализация: `RequireAuthMiddleware` после Session — проверяет `core.UserFromContext(ctx)`; если nil → 401. Bearer и Session — soft-pass (next-middleware вызывается всегда).

**Конфликт 2.** REQ-5.5 (login/csrf endpoints CSRF-skip) vs REQ-5.3 (session-source запросы требуют CSRF).

**Resolution:** CSRFMiddleware проверяет path в самом начале — `/api/auth/login` и `/api/auth/csrf` skip независимо от auth-source.

## Open Design Questions

| Question | Why It Matters | Impacted Requirements |
|----------|---------------|----------------------|
| Cookie path: `/` или `/api`? | Affects scope. | REQ-6.1 |
| CSRF-token rotation после успешного state-change? | Security vs UX. | REQ-2.6 |
| Sliding session expiration (extend at use)? | UX vs simplicity. | REQ-2.4 |
| Session list endpoint `/api/auth/sessions`? | UX feature, deferred. | — |
| Cookie name пробел/подчерк | naming convention. | — |

## Verification Commands

| Action | Command | Source |
|--------|---------|--------|
| Test (unit) | `task test` | Taskfile.yml |
| Test (race) | `task test:race` | Taskfile.yml |
| Test (integration) | `task test:integration` | Taskfile.yml |
| Build | `task build` | Taskfile.yml |
| Lint | `task lint` | Taskfile.yml |
| Generate (sqlc) | `task generate` | Taskfile.yml |
