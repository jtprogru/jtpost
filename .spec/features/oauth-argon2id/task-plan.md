# OAuth GitHub + Argon2id (F4c) — Task Plan

**Test Style Source:** Tier 2 (как F4a/F4b).

**Commands:** `task test`, `task test:race`, `task test:integration`, `task build`, `task lint`, `task generate`.

## Coverage Matrix

| Requirement | Tasks | CP |
|-------------|-------|-----|
| REQ-1.x (Hasher) | T-1 | CP-1, 2, 3, 4 |
| REQ-2.x (AuthService integration) | T-2 | CP-5, 6 |
| REQ-3.x (OAuthAccount + repo) | T-3 | CP-14, 15 |
| REQ-4.x (OAuthService) | T-5 | CP-7, 8, 9, 10 |
| REQ-5.x (GitHubProvider) | T-4 | CP-13 |
| REQ-6.x (HTTP handlers) | T-6 | CP-11, 12 |
| REQ-7.x (Linking) | T-5 | CP-7, 8, 9 |
| REQ-8.x (Bundle+Config) | T-1, T-3 | CP-14, 16 |
| REQ-9.x (Tests) | T-7 | All |

## Work Type: Pure feature

---

## T-1 — PasswordHasher + Config

***_Complexity: standard_***
***_Requirements: REQ-1.x, REQ-8.3, REQ-8.4_***
***_Preservation: CP-1, 2, 3, 4, 16_***

Subtasks:
- [ ] 1. **CODE** Создать `internal/core/password_hasher.go` с `PasswordHasher` interface, `Argon2idHasher` (params: time=1, mem=64*1024, threads=4, keyLen=32, saltLen=16; format `$argon2id$v=19$m=65536,t=1,p=4$<base64-salt>$<base64-hash>`), `LegacyBcryptHasher` (только Verify), `MultiHasher`.
- [ ] 2. **CODE** В `internal/adapters/config/config.go`:
  - `AuthConfig.PasswordHasher string` поле (default "auto").
  - SetDefault + BindEnv + Validate (range check `{"", "auto", "argon2id", "bcrypt"}`).
- [ ] 3. **CODE** Зависимость `golang.org/x/crypto/argon2` через `go get`.
- [ ] 4. **GREEN** `internal/core/password_hasher_test.go`: TestArgon2idHasher_RoundTrip, _WrongPassword, _FormatString, TestLegacyBcryptHasher_VerifyExisting, TestMultiHasher_DetectArgon2id/Bcrypt/UnknownFormat, TestMultiHasher_NeedsRehash (table-driven).
- [ ] 5. **VERIFY** `go test ./internal/core/... ./internal/adapters/config/...` GREEN.

NOTE: Argon2id hash для тестов — НЕЛЬЗЯ снижать parameters до "test-fast" (формат хранится в строке; разные params → разные hashes). Решение: один-два теста с full params (хоть и ~1s каждый); большая часть тестов проверяют detection / format / round-trip с fixed sample.

---

## T-2 — AuthService integration: hasher field, IssueSessionForUser, rehash

***_Complexity: standard_***
***_Requirements: REQ-2.x, REQ-7.4_***
***_Preservation: CP-5, 6_***

Subtasks:
- [ ] 1. **CODE** Изменить `core.NewAuthService(...)`:
  - Параметр `bcryptCost int` → `hasher PasswordHasher`.
  - Заменить все `bcrypt.GenerateFromPassword/CompareHashAndPassword` в CreateUser/VerifyPassword на `hasher.Hash`/`Verify`.
  - В `Login` после успешной VerifyPassword — `if hasher.NeedsRehash(user.PasswordHash)` → goroutine с context.Background() который вызывает `hasher.Hash(password)` → `users.Update`. Errors logged warn.
  - Добавить метод `IssueSessionForUser(ctx, user, ttl) (*LoginResult, error)` — идентичен `Login`-логике после VerifyPassword (генерация session-token + CSRF + Save).
  - В `VerifyPassword` если `user.PasswordHash == ""` → return `core.ErrUnauthorized` (OAuth-only user).
- [ ] 2. **CODE** Обновить все callsites `NewAuthService` в:
  - `internal/cli/serve.go`, `user.go`, `token.go` — заменить `cfg.Auth.BCryptCost` на построенный `hasher`.
  - `internal/adapters/httpapi/{bearer,session,auth_handlers}_test.go` — использовать `core.NewMultiHasher()` или helper.
- [ ] 3. **CODE** Helper `core.HasherFromConfig(cfg AuthConfig) PasswordHasher`:
  - "auto" / "" / "argon2id" → `MultiHasher{Argon2idHasher{...}, LegacyBcryptHasher{}}`.
  - "bcrypt" → `MultiHasher{Argon2idHasher{...}, LegacyBcryptHasher{}}` тоже (т.к. read-write всегда argon, read legacy bcrypt).
- [ ] 4. **GREEN** В `internal/core/auth_service_test.go` — обновить mocks (svc-builder использует MultiHasher), добавить:
  - TestAuthService_Login_RehashLegacy: создать user с manual-bcrypt-hash, login → eventually user.PasswordHash в Argon2id format.
  - TestAuthService_VerifyPassword_OAuthOnly_Rejected: PasswordHash="" → ErrUnauthorized.
  - TestAuthService_IssueSessionForUser_Roundtrip: returns valid LoginResult; ValidateSession success.
- [ ] 5. **VERIFY** `task test ./internal/core/... ./internal/cli/... ./internal/adapters/httpapi/...` GREEN.

NOTE: bcrypt cost для существующих manual hashes в тестах — `bcrypt.MinCost` (4); MultiHasher Verify их распознает.

---

## T-3 — OAuthAccount + repo + миграция + Bundle

***_Complexity: standard_***
***_Requirements: REQ-3.x, REQ-8.1, REQ-8.2, REQ-8.5_***
***_Preservation: CP-14, 15_***

Subtasks:
- [ ] 1. **CODE** Создать `internal/core/oauth_account.go` (тип + UserInfo) и `oauth_repository.go` (interface).
- [ ] 2. **CODE** Миграции `0004_oauth_accounts.sql` (sqlite + postgres).
- [ ] 3. **CODE** Queries `internal/adapters/{sqlite,postgres}/queries/oauth_accounts.sql`: GetByExternalID, Create, ListByUser, Delete.
- [ ] 4. **CODE** `task generate` — sqlc.
- [ ] 5. **CODE** Адаптеры `internal/adapters/{sqlite,postgres}/oauth_accounts.go` + facade `(*PostRepository).OAuthAccounts()`.
- [ ] 6. **CODE** `Bundle.OAuthAccounts core.OAuthAccountRepository` в factory.go.
- [ ] 7. **CODE** В `Config`:
  - `OAuthConfig` deprecated (оставить).
  - `AuthConfig.OAuthProviders map[string]OAuthProviderConfig` (новое поле). При load — если `OAuthConfig.Provider != ""` → mapping в `OAuthProviders[OAuthConfig.Provider]`.
  - SetDefault + BindEnv (для GitHub: `auth.oauth.providers.github.client_id`, `client_secret`, `redirect_url`).
- [ ] 8. **GREEN** `internal/adapters/sqlite/oauth_accounts_test.go`: CRUD + cascade.
- [ ] 9. **GREEN** Postgres-зеркало под integration tag.
- [ ] 10. **VERIFY** `task generate && task test ./internal/adapters/...` GREEN.

NOTE: viper не поддерживает `map[string]X` через env очень хорошо; для F4c достаточно загрузить provider config из YAML. Env-поддержка для `JTPOST_AUTH_OAUTH_PROVIDERS_GITHUB_CLIENT_ID` — отложить.

---

## T-4 — GitHubProvider

***_Complexity: standard_***
***_Requirements: REQ-5.x_***
***_Preservation: CP-13_***

Subtasks:
- [ ] 1. **CODE** `go get golang.org/x/oauth2`.
- [ ] 2. **CODE** Создать `internal/core/oauth_providers/github.go`:
  - `type GitHubProvider struct{ cfg oauth2.Config; httpClient *http.Client }`.
  - Constructor `NewGitHubProvider(clientID, clientSecret, redirectURL string)`.
  - `Name() string { return "github" }`.
  - `AuthorizeURL(state) string` — `cfg.AuthCodeURL(state)`.
  - `Exchange(ctx, code) (string, error)` — `cfg.Exchange(ctx, code)` → returns AccessToken.
  - `FetchUserInfo(ctx, accessToken) (*OAuthUserInfo, error)`:
    - GET `https://api.github.com/user` с `Authorization: Bearer <token>`.
    - GET `https://api.github.com/user/emails`.
    - Найти primary && verified email.
    - Вернуть `&OAuthUserInfo{ExternalID: strconv.Itoa(id), Email: <email>, DisplayName: login}`.
    - Если verified email отсутствует → `ErrValidation`.
- [ ] 3. **GREEN** `internal/core/oauth_providers/github_test.go`:
  - TestGitHubProvider_AuthorizeURL: проверка query params (client_id, redirect_uri, state, scope=user:email).
  - TestGitHubProvider_FetchUserInfo_Mock: httptest.NewServer возвращает /user и /user/emails JSON; verify provider.FetchUserInfo возвращает correct info.
  - TestGitHubProvider_FetchUserInfo_NoVerifiedEmail → ErrValidation.
- [ ] 4. **VERIFY** GREEN.

NOTE: Для подмены httptest URL в тесте — переопределить `oauth2.Endpoint` или вынести base-URL как параметр provider'а.

---

## T-5 — OAuthService + linking logic

***_Complexity: complex_***
***_Requirements: REQ-4.x, REQ-7.x_***
***_Preservation: CP-7, 8, 9, 10_***

Subtasks:
- [ ] 1. **CODE** `internal/core/oauth_service.go`:
  - `OAuthProvider` interface (Name, AuthorizeURL, Exchange, FetchUserInfo).
  - `OAuthService struct { providers map[string]OAuthProvider; users UserRepository; oauthAccounts OAuthAccountRepository; defaultTenantID uuid.UUID; defaultRole Role; clock Clock }`.
  - `NewOAuthService(...)`.
  - `BuildAuthorizeURL(provider) (url, state, error)`:
    - lookup provider по name → ErrConfigInvalid если miss.
    - generate 32-byte random state (base64 std encoding для cookie compat).
    - return provider.AuthorizeURL(state), state, nil.
  - `HandleCallback(ctx, provider, code) (*User, error)`:
    - lookup provider.
    - provider.Exchange → token.
    - provider.FetchUserInfo → info.
    - oauthAccounts.GetByExternalID(provider, info.ExternalID): если nil → returns user. Иначе:
      - users.GetByEmail(defaultTenantID, info.Email): если найден → создать OAuthAccount link, вернуть existing user.
      - иначе: создать User{ID: NewV7, TenantID: defaultTenantID, Email, PasswordHash: "", Role: defaultRole, CreatedAt/UpdatedAt: now} via users.Create. Затем OAuthAccount. Вернуть new user.
- [ ] 2. **GREEN** `internal/core/oauth_service_test.go`:
  - mock OAuthProvider.
  - TestOAuthService_BuildAuthorizeURL: returns url с state.
  - TestOAuthService_HandleCallback_ExistingOAuth: pre-existing oauth_account → returns user.
  - TestOAuthService_HandleCallback_LinkByEmail: existing user-by-email + new oauth_account → link + return existing.
  - TestOAuthService_HandleCallback_NewUser: ни того ни другого → создаёт User+OAuthAccount.
  - TestOAuthService_HandleCallback_UnknownProvider → ErrConfigInvalid.
  - TestOAuthService_HandleCallback_NoEmail (mock returns ErrValidation): пропагация.
- [ ] 3. **VERIFY** GREEN.

---

## T-6 — HTTP handlers + serve.go wiring

***_Complexity: standard_***
***_Requirements: REQ-6.x_***
***_Preservation: CP-11, 12_***

Subtasks:
- [ ] 1. **CODE** `internal/adapters/httpapi/oauth_handlers.go`:
  - `OAuthHandler struct { oauthSvc *core.OAuthService; authSvc *core.AuthService; cfg *config.Config }`.
  - Constructor `NewOAuthHandler(...)`.
  - `Initiate(provider string) http.HandlerFunc` (используется через path-router):
    - `BuildAuthorizeURL(provider)` → если ErrConfigInvalid → 404.
    - Set-Cookie `jtpost_oauth_state=<state>; Max-Age=600; HttpOnly; Secure=cfg.Server.CookieSecure; SameSite=Lax; Path=/api/auth/oauth/`.
    - 302 Location: <authorizeURL>.
  - `Callback(provider string) http.HandlerFunc`:
    - читать cookie `jtpost_oauth_state`. Отсутствует → 400 `state_missing`.
    - читать `state` query param. Не совпадает (constant-time) → 400 `state_mismatch`.
    - читать `code` query param. Empty → 400.
    - clear state-cookie (Max-Age=-1).
    - `oauthSvc.HandleCallback(ctx, provider, code)` → user. Errors → 400 `oauth_failed`.
    - `authSvc.IssueSessionForUser(ctx, user, ttl)` → loginResult.
    - setSessionCookie.
    - 302 Location: `/`.
- [ ] 2. **CODE** В `internal/adapters/httpapi/server.go`:
  - `ServerConfig.OAuthService *core.OAuthService` field.
  - В `registerRoutes`: `mux.HandleFunc("/api/auth/oauth/", ...)` — path-extracting router (parsing `/api/auth/oauth/{provider}` или `.../callback`).
- [ ] 3. **CODE** `internal/adapters/httpapi/middleware.go`: `requireAuthSkipPaths` дополнить `/api/auth/oauth/` (по prefix через дополнительный check `strings.HasPrefix(r.URL.Path, "/api/auth/oauth/")`).
- [ ] 4. **CODE** `internal/cli/serve.go`:
  - При `auth.type=token` создать `OAuthService` если в `cfg.Auth.OAuthProviders` зарегистрированы.
  - Передать в `httpapi.ServerConfig.OAuthService`.
- [ ] 5. **GREEN** `internal/adapters/httpapi/oauth_handlers_test.go`:
  - mock OAuthService через interface (sub'ить provider в OAuthService).
  - TestOAuthHandler_Initiate_RedirectAndCookie.
  - TestOAuthHandler_Initiate_UnknownProvider_404.
  - TestOAuthHandler_Callback_StateMissing_400.
  - TestOAuthHandler_Callback_StateMismatch_400.
  - TestOAuthHandler_Callback_Success_SessionCookie_Redirect.
- [ ] 6. **VERIFY** `task test` GREEN.

---

## T-7 — Финал + smoke + GATE

***_Complexity: mechanical_***
***_Requirements: REQ-9.x_***

Subtasks:
- [ ] 1. **VERIFY** `task fmt && task vet && task test && task test:race` GREEN.
- [ ] 2. **VERIFY** `task generate && git diff --exit-code -- internal/adapters/{sqlite/sqlitedb,postgres/pgdb}` clean.
- [ ] 3. **CODE** Обновить `CHANGELOG.md` секцией F4c.
- [ ] 4. **CODE** Обновить `.jtpost.example.yaml`: пример `auth.oauth.providers.github`, `auth.password_hasher: auto`.
- [ ] 5. **VERIFY** Smoke unit-test purely: hash + login + rehash работают (без real GitHub, т.к. нужны credentials).
- [ ] 6. **VERIFY** `task lint` — нет new findings выше severity minor.
- [ ] 7. **VERIFY** `task build` OK.

---

## T-8 — GATE

***_Complexity: mechanical_***

CRITICAL: финал. T-1..T-7 все complete; все checks GREEN.
