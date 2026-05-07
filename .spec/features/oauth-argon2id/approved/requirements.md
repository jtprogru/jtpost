# OAuth GitHub + Argon2id (F4c) — Requirements

**Status:** Draft
**Author:** Claude (Opus 4.7) + Mikhail Savin
**Date:** 2026-05-07
**Feature:** oauth-argon2id (F4c)
**Branch:** `feature/oauth-argon2id`

## Overview

F4c закрывает B.2 из DEVELOPMENT_PLAN: GitHub OAuth provider + миграция bcrypt → Argon2id. Hasher-абстракция (`core.PasswordHasher`) поддерживает оба формата с детекцией по prefix; legacy bcrypt-passwords пересохраняются в Argon2id silent при successful login. OAuth flow: `GET /api/auth/oauth/github` → 302 на GitHub → callback `/callback?code=&state=` → exchange + GitHub user info + automatic email-based account linking → выдача session-cookie из F4b. State-token хранится в short-lived (10 мин) HttpOnly cookie. Multi-provider scaffold (`auth.oauth.providers map`) подготовлен; в F4c реализован только GitHub. Google/Yandex/audit-log отложены.

## Glossary

| Term | Definition | Code Artifact |
|------|------------|---------------|
| `PasswordHasher` | Interface: Hash, Verify, NeedsRehash | `internal/core/password_hasher.go` |
| `Argon2idHasher` | Default impl с baseline OWASP 2024 params (time=1, memory=64MB, threads=4, keyLen=32) | `internal/core/password_hasher.go` |
| `LegacyBcryptHasher` | Read-only verifier для existing F4a-hashes | `internal/core/password_hasher.go` |
| `MultiHasher` | Detector by prefix: `$2a$/$2b$/$2y$` → bcrypt; `$argon2id$` → argon2 | `internal/core/password_hasher.go` |
| `OAuthAccount` | Linking record: id, user_id, provider, external_id, email, created_at | `internal/core/oauth_account.go` |
| `OAuthAccountRepository` | Storage interface (GetByExternalID, Create, ListByUser) | `internal/core/oauth_account.go` |
| `OAuthService` | Domain service: BuildAuthorizeURL, HandleCallback | `internal/core/oauth_service.go` |
| `GitHubProvider` | OAuth2 client implementation | `internal/core/oauth_providers/github.go` |
| `oauthStateCookie` | Cookie `jtpost_oauth_state` с random 32-byte state, TTL 10m | `internal/adapters/httpapi/oauth_handlers.go` |
| `argon2idPrefix` | Константа `"$argon2id$"` для detection | `password_hasher.go` |

## User Stories

- Как **владелец канала**, я хочу нажать "Login with GitHub" → быть редиректнутым на GitHub OAuth → после approve получить session-cookie без ввода password.
- Как **владелец**, я хочу что бы существующий local user с email matching GitHub email автоматически получил OAuth-link (без двойного account).
- Как **разработчик-сопровождающий**, я хочу безболезненной миграции password-hash формата: existing bcrypt-passwords продолжают работать, новые login пишут Argon2id, при re-login старого user — автоматический re-hash.
- Как **DevOps**, я хочу через config зарегистрировать GitHub OAuth App: `auth.oauth.providers.github.{client_id, client_secret, redirect_url}`.

## Requirements

### Group 1 — PasswordHasher

**REQ-1.1** WHEN модуль `core` определяет интерфейс `PasswordHasher`, the system SHALL содержать методы: `Hash(password string) (string, error)`, `Verify(hash, password string) error`, `NeedsRehash(hash string) bool`.

**REQ-1.2** WHEN экспортируется тип `Argon2idHasher`, the system SHALL использовать параметры по OWASP 2024 baseline: `time=1`, `memory=64*1024 KB (64MB)`, `threads=4`, `keyLen=32`, `saltLen=16`.

**REQ-1.3** WHEN `Argon2idHasher.Hash(password)` вызывается, the system SHALL вернуть строку формата `$argon2id$v=19$m=65536,t=1,p=4$<base64-salt>$<base64-hash>`.

**REQ-1.4** WHEN `Argon2idHasher.Verify(hash, password)` вызывается, the system SHALL парсить строку, перехешировать password с теми же params, сравнивать через `subtle.ConstantTimeCompare`. Mismatch → `core.ErrUnauthorized`.

**REQ-1.5** WHEN экспортируется `LegacyBcryptHasher`, the system SHALL поддерживать только `Verify` (read-only): `bcrypt.CompareHashAndPassword`. `Hash` возвращает ошибку (deprecated).

**REQ-1.6** WHEN `MultiHasher.Verify(hash, password)` вызывается, the system SHALL детектировать тип hash по prefix:
- `$argon2id$` → delegate в `Argon2idHasher.Verify`.
- `$2a$|$2b$|$2y$` → delegate в `LegacyBcryptHasher.Verify`.
- иначе → `ErrUnauthorized` (unknown format).

**REQ-1.7** WHEN `MultiHasher.NeedsRehash(hash)` вызывается, the system SHALL вернуть `true` для bcrypt-prefix hashes (legacy → нужен upgrade), `false` для `$argon2id$` (current).

**REQ-1.8** WHEN `MultiHasher.Hash(password)` вызывается, the system SHALL делегировать в `Argon2idHasher.Hash` (новые passwords всегда Argon2id).

### Group 2 — AuthService integration

**REQ-2.1** WHEN `core.NewAuthService` вызывается, the system SHALL принимать дополнительный параметр `hasher PasswordHasher` в начале (после repos). Старая `bcryptCost int` параметр заменяется hasher'ом.

**REQ-2.2** WHEN `AuthService.CreateUser(in)` вызывается, the system SHALL хешировать password через `s.hasher.Hash(password)`.

**REQ-2.3** WHEN `AuthService.VerifyPassword(ctx, tenantID, email, password)` валидирует, the system SHALL использовать `s.hasher.Verify(user.PasswordHash, password)`.

**REQ-2.4** WHEN `AuthService.Login(ctx, in, ttl)` после успешного `VerifyPassword` обнаруживает `s.hasher.NeedsRehash(user.PasswordHash) == true`, the system SHALL re-hash password в новом формате и вызвать `users.Update(ctx, user)` (асинхронно через goroutine с context.Background()). При ошибке UpdateUser — log warning, не блокировать login.

### Group 3 — OAuthAccount domain

**REQ-3.1** WHEN модуль `core` определяет тип `OAuthAccount`, the system SHALL содержать поля: `ID uuid.UUID`, `UserID uuid.UUID`, `Provider string`, `ExternalID string`, `Email string`, `CreatedAt time.Time`.

**REQ-3.2** WHEN модуль `core` экспортирует `OAuthAccountRepository`, the system SHALL определять методы: `GetByExternalID(ctx, provider, externalID) (*OAuthAccount, error)`, `Create(ctx, *OAuthAccount) error`, `ListByUser(ctx, userID) ([]*OAuthAccount, error)`, `Delete(ctx, id) error`.

**REQ-3.3** WHEN goose применяет миграцию `0004_oauth_accounts.sql`, the system SHALL создать таблицу `oauth_accounts(id PK, user_id FK→users.id ON DELETE CASCADE, provider, external_id, email, created_at)` с `UNIQUE(provider, external_id)`, INDEX(user_id).

### Group 4 — OAuthService

**REQ-4.1** WHEN `OAuthService.BuildAuthorizeURL(provider string) (url, state string, error)` вызывается, the system SHALL найти провайдера по имени, сгенерировать random 32-byte state (base64), вернуть OAuth-authorize URL и state.

**REQ-4.2** WHEN `OAuthService.HandleCallback(ctx, provider, code string)` вызывается, the system SHALL:
- exchange code → access token через provider.
- получить user info (external_id, email) через provider API.
- найти `oauth_accounts.GetByExternalID(provider, external_id)` → если найдено → возврат соответствующего user.
- иначе попытаться `users.GetByEmail(tenantID, email)` → если найдено → создать `OAuthAccount` link → возврат user (REQ-7.4 linking).
- иначе создать нового user (с пустым password_hash — означает OAuth-only) и `OAuthAccount`.
- вернуть `*User`.

**REQ-4.3** WHEN `OAuthService.HandleCallback` сталкивается с провайдером отвечающим без verified email, the system SHALL вернуть ошибку `core.ErrValidation` с сообщением `oauth provider returned no verified email`.

**REQ-4.4** WHEN провайдер с указанным именем не зарегистрирован, the system SHALL вернуть `core.ErrConfigInvalid: oauth provider not configured`.

### Group 5 — GitHubProvider

**REQ-5.1** WHEN регистрируется `GitHubProvider`, the system SHALL использовать `golang.org/x/oauth2/github` для endpoint URLs.

**REQ-5.2** WHEN `GitHubProvider.AuthorizeURL(state)` вызывается, the system SHALL вернуть `https://github.com/login/oauth/authorize?client_id=...&redirect_uri=...&state=...&scope=user:email`.

**REQ-5.3** WHEN `GitHubProvider.Exchange(ctx, code)` вызывается, the system SHALL POST на `https://github.com/login/oauth/access_token` с client credentials, вернуть `*oauth2.Token`.

**REQ-5.4** WHEN `GitHubProvider.FetchUserInfo(ctx, token)` вызывается, the system SHALL:
- GET `https://api.github.com/user` → struct `{id int, login, email *}`.
- GET `https://api.github.com/user/emails` → list of `{email, primary, verified}`.
- Выбрать primary && verified email; если такого нет → ошибка.
- Вернуть `OAuthUserInfo{ExternalID: strconv.Itoa(id), Email: <primary verified>, DisplayName: login}`.

### Group 6 — HTTP handlers

**REQ-6.1** WHEN HTTP `GET /api/auth/oauth/{provider}` принимается, the system SHALL вызвать `OAuthService.BuildAuthorizeURL(provider)`, установить cookie `jtpost_oauth_state=<state>; Max-Age=600; HttpOnly; Secure; SameSite=Lax; Path=/api/auth/oauth/`, вернуть HTTP 302 с `Location: <authorize_url>`.

**REQ-6.2** WHEN HTTP `GET /api/auth/oauth/{provider}/callback?code=&state=` принимается, the system SHALL:
- прочитать `jtpost_oauth_state` cookie; если отсутствует → 400.
- Сравнить cookie value с `state` query param через `subtle.ConstantTimeCompare`; mismatch → 400.
- Очистить state-cookie (`Max-Age=-1`).
- Вызвать `OAuthService.HandleCallback(ctx, provider, code)` → user.
- Вызвать `AuthService.IssueSessionForUser(ctx, user, ttl)` (новый метод; см. REQ-2.5) → LoginResult.
- Установить session-cookie + редирект 302 на `/`.

**REQ-6.3** WHEN HTTP `GET /api/auth/oauth/{provider}` запускается с unknown provider, the system SHALL вернуть HTTP 404.

**REQ-6.4** WHEN HTTP /api/auth/oauth/* endpoints включены в RequireAuthMiddleware skip-list, the system SHALL пропускать их без auth-проверки.

**REQ-6.5** WHEN HTTP `GET /api/auth/oauth/{provider}/callback` сталкивается с ошибкой OAuth (HandleCallback fail), the system SHALL вернуть HTTP 400 с body `{"error":"oauth_failed", "details": "..."}`.

### Group 2 (продолжение) — IssueSessionForUser

**REQ-2.5** WHEN `AuthService.IssueSessionForUser(ctx, user *User, ttl time.Duration) (*LoginResult, error)` вызывается, the system SHALL создать новую session (как `Login` минус password verify) и вернуть `LoginResult`. Используется OAuth flow.

### Group 7 — Account linking

**REQ-7.1** WHEN `OAuthService.HandleCallback` находит существующий `oauth_accounts` запись по (provider, external_id), the system SHALL вернуть user из этой записи (re-login).

**REQ-7.2** WHEN `OAuthService.HandleCallback` НЕ находит `oauth_accounts` запись и `users.GetByEmail(tenantID, oauthEmail)` находит существующего user, the system SHALL создать `OAuthAccount{UserID: existingUser.ID, ...}` (link) и вернуть existing user.

**REQ-7.3** WHEN `OAuthService.HandleCallback` НЕ находит ни oauth_account ни user-by-email, the system SHALL создать нового User c пустым `PasswordHash=""` (OAuth-only), `Role=cfg.Auth.OAuthDefaultRole` (или `RoleAuthor` если не задан), и одновременно `OAuthAccount`.

**REQ-7.4** WHEN AuthService.VerifyPassword вызывается для user с `PasswordHash == ""`, the system SHALL вернуть `ErrUnauthorized` (OAuth-only users не могут логиниться через password).

### Group 8 — Storage Bundle + Config

**REQ-8.1** WHEN `storage.Bundle` определяется, the system SHALL содержать поле `OAuthAccounts core.OAuthAccountRepository` (nil для fs).

**REQ-8.2** WHEN `Config.Auth` расширяется, the system SHALL содержать поле `OAuth.Providers map[string]OAuthProviderConfig`. Каждый ProviderConfig: `{ClientID, ClientSecret, RedirectURL string}`. Существующее `OAuthConfig` поле сохраняется как deprecated; при миграции заполняется как `Providers["github"]`.

**REQ-8.3** WHEN `Config.Auth` расширяется, the system SHALL содержать поле `PasswordHasher string` со значениями `"argon2id" | "bcrypt" | "auto"` (default `"auto"` = MultiHasher detection).

**REQ-8.4** WHEN `Config.Validate()` запускается с `Auth.PasswordHasher` ∉ `{"", "auto", "argon2id", "bcrypt"}`, the system SHALL вернуть `ErrConfigInvalid`.

**REQ-8.5** WHEN при инициализации serve.go регистрируется OAuthService, the system SHALL включать только тех провайдеров из `Auth.OAuth.Providers` у которых заданы все поля (ClientID, ClientSecret, RedirectURL).

### Group 9 — Тесты

**REQ-9.1** WHEN `task test` запускается, the system SHALL пройти `TestArgon2idHasher_RoundTrip`, `TestArgon2idHasher_Verify_Wrong`, `TestLegacyBcryptHasher_*`, `TestMultiHasher_DetectByPrefix`, `TestMultiHasher_NeedsRehash` (table-driven).

**REQ-9.2** WHEN `task test` запускается, the system SHALL пройти `TestAuthService_Login_RehashLegacy` (login old bcrypt-user → password updated to argon2id), `TestAuthService_VerifyPassword_OAuthOnlyUser_Rejected` (PasswordHash="" → ErrUnauthorized).

**REQ-9.3** WHEN `task test` запускается, the system SHALL пройти SQLite/Postgres тесты для `OAuthAccountRepository` (CRUD + cascade delete).

**REQ-9.4** WHEN `task test` запускается, the system SHALL пройти `TestOAuthService_BuildAuthorizeURL`, `TestOAuthService_HandleCallback_NewUser` (mock GitHubProvider), `_ExistingOAuthAccount`, `_LinkByEmail`.

**REQ-9.5** WHEN `task test` запускается, the system SHALL пройти HTTP-handler тесты для `/api/auth/oauth/github` (state cookie set, redirect to GitHub) и `/api/auth/oauth/github/callback` (state validation, success → session cookie + redirect /).

## Topological Order

```
Group 1 (Hasher) → Group 2 (AuthService integration) → Group 8 (Config) →
Group 3 (OAuthAccount + repo + migration) → Group 5 (GitHubProvider) →
Group 4 (OAuthService) → Group 7 (Linking logic) → Group 6 (HTTP handlers) →
Group 9 (Tests финал)
```

## Conflict Priority

**Конфликт 1.** REQ-2.4 (re-hash после success login) vs REQ-2.5 (IssueSessionForUser в OAuth bypassит password). 

**Resolution:** OAuth-flow не вызывает VerifyPassword → не делает re-hash. Re-hash только для password-login через `Login`.

**Конфликт 2.** REQ-7.2 (auto-link by email) vs security (email injection через GitHub).

**Resolution:** GitHub primary email обязан быть verified (REQ-5.4). Это снижает риск. В follow-up — explicit confirm UI.

## Open Design Questions

| Question | Why It Matters | Impacted Requirements |
|----------|---------------|----------------------|
| Argon2id parameters config-override? | Adjustable for low-power servers. Сейчас hardcoded baseline. | REQ-1.2 |
| OAuth user без email из GitHub — fail или fallback `<login>@users.noreply.github.com`? | UX vs security. | REQ-5.4, REQ-4.3 |
| После callback redirect на `/`, или из query `?next=/some/path`? | Web UI integration. | REQ-6.2 |
| `oauth_accounts` уникальность: per-tenant (provider, external_id, tenant_id) или global (provider, external_id)? | Multi-tenant impact. | REQ-3.3 |

## Verification Commands

| Action | Command | Source |
|--------|---------|--------|
| Test (unit) | `task test` | Taskfile.yml |
| Test (race) | `task test:race` | Taskfile.yml |
| Test (integration) | `task test:integration` | Taskfile.yml |
| Build | `task build` | Taskfile.yml |
| Lint | `task lint` | Taskfile.yml |
| Generate | `task generate` | Taskfile.yml |
