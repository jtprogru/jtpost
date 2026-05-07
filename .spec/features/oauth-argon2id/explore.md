# Exploration: OAuth GitHub + Argon2id (F4c)

## Intent

F4c — финальная часть B.2 из DEVELOPMENT_PLAN. F4a дал local password+PAT+RBAC, F4b — cookie-sessions+CSRF. F4c закрывает оставшиеся пункты:

1. **OAuth2 GitHub provider** — `Login with GitHub`. Foundation для future Google/Yandex (одинаковый pattern через `oauth2` или `coreos/go-oidc`). После callback — связывание `oauth_accounts(provider, external_id) → user` и выдача session-cookie из F4b.
2. **Argon2id** — миграция bcrypt → Argon2id для password hashing. Hasher-абстракция (`core.PasswordHasher` interface) с двумя impls: legacy bcrypt detector (читает existing F4a-hashes) и Argon2id (новые passwords). При login со старым bcrypt-hash — re-hash в Argon2id silently.

**Чего F4c НЕ делает:**
- Не реализует Google/Yandex providers — отдельные follow-up (но архитектура их поддержит).
- Не делает email verification flow (требует email infra).
- Не делает password reset (откладывается).
- Не реализует `audit_log` — отдельная B-этап фича.
- Не делает 2FA / TOTP.
- Не делает OAuth для CLI / device flow (`jtpost auth login --github`).
- Не делает per-channel RBAC.

**Триггер:** после F4b есть session-механика; OAuth-callback нуждается в session для state-protection и для финальной выдачи cookie. Argon2id — security-best-practice (bcrypt уже считается suboptimal в 2025+).

---

## Investigation

### Что уже есть после F4a + F4b

**Auth chain в `serve.go`:** Bearer → Session → CSRF → RequireAuth. F4c добавит OAuth-handlers (login-redirect + callback) с `RequireAuthMiddleware` skip-list (как login).

**`AuthService` (F4a + F4b):** `CreateUser` использует `bcrypt.GenerateFromPassword(... cost)`. `VerifyPassword` использует `bcrypt.CompareHashAndPassword`. F4c заменит на `Hasher`-интерфейс.

**Конфиг `OAuthConfig`** уже определён в `config.go` (F1):
```go
type OAuthConfig struct {
    Provider     string
    ClientID     string
    ClientSecret string
    RedirectURL  string
}
```
F4c расширит до multiple providers (map) ИЛИ оставит single-provider в F4c (GitHub) + extension позже.

**Storage Bundle** имеет Posts/Users/Tokens/Sessions. F4c добавит `OAuthAccounts core.OAuthAccountRepository` для linking.

### Зависимости

- **`golang.org/x/oauth2`** — стандарт Go OAuth2-client. Поддерживает GitHub natively (`oauth2/github`).
- **`golang.org/x/crypto/argon2`** — Argon2id implementation in Go stdlib-extras. **НЕТ** в текущем go.mod (есть только `bcrypt`).
- Альтернативы Argon2: `github.com/matthewhartstonge/argon2` (3rd-party, parametrized API). Stdlib `argon2.IDKey` достаточно — самостоятельный wrapping для format string `$argon2id$v=19$m=...,t=...,p=...$<salt>$<hash>`.
- **`coreos/go-oidc`** — для OIDC (OpenID Connect, e.g. Google). Не нужен для GitHub (он не OIDC). Откладывается до Google-provider.

### Тестовый контекст

- OAuth flow тестируется через mock GitHub OAuth-сервер (httptest.NewServer) — стандартный pattern.
- Argon2id тесты — round-trip hash/verify с разными параметрами; backward-compat detection по prefix.

### Архитектурные точки

- `internal/core/password_hasher.go` — `PasswordHasher` interface, `Argon2idHasher`, `LegacyBcryptHasher`, `MultiHasher` (detection by prefix).
- `internal/core/auth_service.go` — заменить `bcrypt.GenerateFromPassword`/`CompareHashAndPassword` на `s.passwordHasher.Hash`/`Verify`.
- `internal/core/oauth_account.go` — `OAuthAccount` type.
- `internal/core/oauth_repository.go` — `OAuthAccountRepository` interface.
- `internal/adapters/{sqlite,postgres}` — миграция `0004_oauth_accounts.sql`, queries, адаптеры.
- `internal/adapters/storage` — Bundle.OAuthAccounts.
- `internal/core/oauth_service.go` — `OAuthService` (отдельно от AuthService): `BuildAuthorizeURL(provider) (url, state)`, `HandleCallback(provider, code, state) (*User, *LoginResult)`.
- `internal/adapters/httpapi/oauth_handlers.go` — `GET /api/auth/oauth/{provider}` (initiate), `GET /api/auth/oauth/{provider}/callback`.
- `internal/adapters/config/config.go` — `auth.password_hasher` ("argon2id"|"bcrypt"|"auto"); `auth.oauth.providers map[string]OAuthProviderConfig` (расширение).
- `internal/cli/serve.go` — wiring OAuthService в serve handlers.

### Что F4c НЕ затрагивает

- F4a/F4b PAT и cookie-session-flow остаются. OAuth выдаёт session-cookie через тот же mechanism.
- Existing users с bcrypt-passwords продолжают логиниться (LegacyBcryptHasher).
- При первой успешной login со старым hash → пересохраняем в Argon2id.

---

## Build Tooling

- **Test/Build/Lint:** без изменений.
- **Generate:** sqlc — новые queries для oauth_accounts.
- **CI:** без изменений.

---

## Options Considered

### Option A: Single GitHub provider в F4c, multi-provider — отдельно

OAuth только для GitHub. Config — single `OAuthConfig` (как сейчас). Аргументы: уже сделано в F1 schema; foundation готова; Google/Yandex — extension.

- **Pros:** меньший scope; быстрее MVP.
- **Cons:** требует follow-up рефакторинга для multi-provider (Map[provider]Config + dispatch).
- **Сложность:** Low.

### Option B: Multi-provider scaffold с GitHub как первой реализацией

`auth.oauth.providers map[string]OAuthProviderConfig`; GitHub зарегистрирован, Google/Yandex — placeholder. Service routing по provider name.

- **Pros:** future-proof, добавление Google = только config + 5 строк mapping.
- **Cons:** немного больше scope.
- **Сложность:** Medium.

### Option C: Argon2id-only, OAuth — отдельная фича

Разделить F4c на две: F4c-a (Argon2id), F4c-b (OAuth GitHub).

- **Pros:** меньшие фичи, проще ревью.
- **Cons:** обе небольшие отдельно; bundling экономит pipeline-overhead.
- **Сложность:** Low+Low.

### Option D: только OAuth, Argon2id — отложить

Bcrypt cost=10 — приемлемо (по NIST 2024 minimum достаточно); Argon2id рекомендован, но не критичен.

- **Pros:** минимальный scope.
- **Cons:** оставляет security-debt; миграция в будущем сложнее (две раздельные фичи рядом).
- **Сложность:** Low.

---

## Constraints & Risks

### Backward compatibility

- F4a/F4b deployments продолжают работать. Existing bcrypt-passwords проходят `LegacyBcryptHasher.Verify`.
- При login → `Hasher.NeedsRehash(hash)` → если bcrypt → silent re-hash в Argon2id и `UpdateUser`.
- `auth.oauth.*` уже в config schema (F1) — расширение к `providers map` совместимо если default не меняется.

### Security

- **Argon2id parameters** (OWASP 2024 baseline): `time=1, memory=64MB, threads=4, keyLen=32`. Заметим что эти параметры дороже чем bcrypt cost=10 (~1s vs ~100ms). Конфигурируемо.
- **OAuth state**: random 32-byte token, хранится в session-cookie (F4b infrastructure) или в short-lived `oauth_states` таблице. Простой подход: cookie с `oauth_state` value, проверяется в callback.
- **Redirect URL whitelist**: GitHub redirects ОБРАТНО на наш callback. URL должен match `cfg.Auth.OAuth.RedirectURL` exactly (registered с GitHub OAuth App).
- **Code exchange**: HTTPS-only POST к `https://github.com/login/oauth/access_token`. OAuth library это handles.
- **Email verification от GitHub**: GitHub API возвращает primary verified email. Используем его. Если у пользователя несколько emails, первый verified primary.
- **Existing user account linking**: если email из GitHub совпадает с existing local user → link (создать `oauth_accounts` запись). НЕ создавать second account.
- **Brute-force на state**: 32-byte random + 10-min TTL — достаточно.

### Performance

- Argon2id default ≈ 1s на password verify. Это медленнее bcrypt cost=10 (≈100ms). Login latency растёт. PAT-валидация не затронута (использует bcrypt cost=6 сепаратно).
- **Mitigation**: рассматриваем только при login (редкая операция); OAuth bypass'ит password hash полностью.

### Edge cases

- **Двойная привязка**: GitHub user уже привязан к user A, новый login через тот же GitHub → возврат user A (lookup by oauth_accounts.external_id).
- **Email collision**: GitHub email = email уже зарегистрированного local user → spike: автоматически link или создать новый user? Решение: автоматическая link с warning в audit (audit-log в F11). Без email verification — security risk; должны verify через GitHub API (primary+verified).
- **GitHub user без public email**: используем `email:read` scope для получения primary. Если не возвращает — error "email required".
- **Скрытый GitHub email** (privacy noreply): получаем `users/<login>/emails` и используем primary verified.
- **Rate limiting GitHub API**: для разработки достаточно. В prod — headers `X-RateLimit-*` мониторим.
- **Argon2id-rehash race**: два concurrent login → две попытки UpdateUser с argon2 hash. Idempotent (одинаковый hash или один проиграет UPDATE WHERE updated_at) — race не критичен.

---

## Recommended Direction

**Option B — multi-provider scaffold + GitHub-only impl + Argon2id bundled.**

1. **PasswordHasher abstraction** — interface + Argon2id default + LegacyBcryptHasher detector. AuthService параметризуется hasher'ом.
2. **OAuthAccount** entity + `0004_oauth_accounts.sql` (id, user_id, provider, external_id, email, created_at). UNIQUE(provider, external_id).
3. **OAuthService** — domain service: `BuildAuthorizeURL`, `HandleCallback`. Расположение: `internal/core/oauth_service.go`.
4. **GitHub provider** реализация: `internal/core/oauth_providers/github.go` (или nested). Использует `golang.org/x/oauth2/github` для endpoint, ручной `https://api.github.com/user` + `users/emails` для info.
5. **HTTP handlers** `internal/adapters/httpapi/oauth_handlers.go`: `GET /api/auth/oauth/{provider}` → 302 redirect; `GET /api/auth/oauth/{provider}/callback?code=&state=` → handle + Set-Cookie + redirect to /.
6. **Config** — `auth.oauth.providers map[string]ProviderConfig`. Existing `OAuthConfig` поле deprecated. F1-default сохранён, при загрузке мапится.
7. **Bundle.OAuthAccounts** + sqlc + sqlite/postgres адаптеры.
8. **CHANGELOG + docs**: пример GitHub-OAuth setup.

---

## Scope Boundaries

### Must-have (F4c)

- `core.PasswordHasher` interface + `Argon2idHasher` + `LegacyBcryptHasher` + `MultiHasher` (detection by prefix `$2a$`/`$2b$`/`$2y$` → bcrypt; `$argon2id$` → argon2).
- AuthService использует Hasher; на successful Login со старым bcrypt-hash → re-hash to Argon2id + UpdateUser.
- `OAuthAccount` type, `OAuthAccountRepository` interface, миграция, sqlc, sqlite/postgres адаптеры.
- `OAuthService` domain logic + `GitHubProvider` implementation.
- HTTP endpoints `/api/auth/oauth/github`, `/api/auth/oauth/github/callback` через RequireAuthMiddleware skip-list.
- State-token хранится в short-lived cookie (10-min TTL, HttpOnly+Secure+SameSite=Lax) или в `oauth_states` table — выбрать в Design.
- Email-based account linking: если у GitHub-email уже есть local user → link.
- Config: `auth.oauth.providers.github.{client_id, client_secret, redirect_url}` + `auth.password_hasher` ("argon2id"|"bcrypt"|"auto" default "argon2id").
- Тесты: hasher round-trip, legacy detection, mock GitHub server для OAuth flow.

### Deferred

- **Google/Yandex providers** — добавить в follow-up через тот же pattern.
- **`jtpost auth login --github`** CLI device flow.
- **Audit log** на oauth events.
- **Password reset email flow**.
- **Multi-account linking UI** (один user, несколько GitHub accounts).
- **OIDC через coreos/go-oidc** (для Google/Yandex; GitHub не OIDC).

### Needs spike

- **State storage**: cookie vs DB-table. Cookie проще; DB-table надёжнее (даёт revocation). Решение в Design.
- **Argon2id parameters config**: сделать конфигурируемые memory/time/threads или hardcoded baseline? Решение в Design.

---

## Assumptions & Open Questions

### Assumptions

- `[ASSUMPTION: один GitHub-provider в F4c; Google/Yandex — follow-up]`
- `[ASSUMPTION: Argon2id baseline OWASP 2024: time=1, memory=64MB, threads=4, keyLen=32]`
- `[ASSUMPTION: state-token хранится в short-lived HttpOnly cookie (10 мин TTL)]`
- `[ASSUMPTION: existing local user с тем же email → автоматически link OAuth account]` — security risk без email verification, но GitHub primary email verified
- `[ASSUMPTION: GitHub email scope = "user:email"]`
- `[ASSUMPTION: legacy bcrypt re-hash происходит в Login (не в VerifyPassword) — UpdateUser требует TenantID]`
- `[ASSUMPTION: при re-hash failure (БД error) — login всё равно success; следующий login повторит]`
- `[ASSUMPTION: при auth.type=token + GitHub OAuth disabled (no client_id) — endpoints возвращают 404]`

### Open Questions

1. State storage: cookie vs DB? Cookie — простее. DB — устойчивее к XSS на client. Cookie + signed-token — компромисс. Предложение: cookie с random state + проверка в callback.
2. Argon2id parameters: hardcoded или из config? Предложение: hardcoded baseline; config-override — в follow-up.
3. Linking автоматический vs explicit confirm? Предложение: автоматический если email совпадает (GitHub primary verified).
4. После callback — redirect куда? Предложение: HTTP 302 на `/` (Web UI). Параметр `?next=...` — отложен.
5. CSRF на oauth-callback: защищаем через state-token (стандарт OAuth) — но нужен ли дополнительный CSRFMiddleware? Нет — callback это GET с state-cookie.
6. `oauth_accounts.email` — хранить или вычислять через user.email? Хранить — для history/audit.
7. Конфиг multi-provider: `auth.oauth.providers.github.client_id` (nested) vs flat `auth.oauth.github_client_id`? Предложение: nested map.

---

## Done When

- [x] Codebase прочитан.
- [x] 4 опции.
- [x] Trade-offs.
- [x] Scope.
- [x] Assumptions.
- [x] Open Questions (7).
- [x] Build tooling.
- [ ] Артефакт зарегистрирован.
