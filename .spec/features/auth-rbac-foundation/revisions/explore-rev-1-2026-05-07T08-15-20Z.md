# Exploration: Auth/RBAC Foundation (F4)

## Intent

F4 (полная) из DEVELOPMENT_PLAN B.2 включает 5 крупных направлений: local provider (email+password+Argon2id), OAuth2 (GitHub/Google/Yandex через `coreos/go-oidc`), RBAC (`owner|editor|author|viewer` per-channel), сессии через secure HTTP-only cookies + CSRF, и API-токены (PAT). Это объёмно для одной spec-фичи.

**Scope F4 (этой фичи) — Auth Foundation (F4a):**

1. Доменная модель `User` (id, email, password_hash, roles, created_at, updated_at, tenant_id) и `APIToken` (id, user_id, token_hash, name, expires_at, last_used_at).
2. Локальный auth-provider: email+password с **bcrypt** (не Argon2id — упрощение для F4a; Argon2id — в follow-up F4b).
3. Personal Access Tokens (PAT) для API-доступа (CLI и сторонние клиенты): генерация, отзыв, валидация по prefix+hash.
4. RBAC scaffold: типы `Role` и `Permission`, маппинг ролей → permissions, проверка `service.AuthorizeOperation(ctx, op)`. Per-channel-permissions откладываются на момент когда появятся channels (B.1.3).
5. **Bearer-token middleware** заменяющий F1-заглушку `TenantFromConfigMiddleware`. Извлекает PAT из `Authorization: Bearer <token>`, валидирует, кладёт user/tenant/roles в context. При отсутствии токена — 401.
6. CLI команды: `jtpost user create/list/delete`, `jtpost token create/list/revoke`. Команда `jtpost auth login` (для будущего sessions) — отложена.
7. Расширение `Config.Auth`: добавить `auth.bcrypt_cost` (default 10), оставить existing `tenant_default`/`author_default`/`secret` как backward-compat fallback (если auth.type=none — используется F1-заглушка).

**Чего F4 не делает (отложено):**

- OAuth2 — отдельная фича `F4b-oauth-providers`.
- Cookie-sessions + CSRF — отдельная фича `F4c-web-sessions` (нужна для F8 Web UI).
- Argon2id (вместо bcrypt) — в `F4b` (одна миграция вместо двух).
- Per-channel RBAC — отложен до появления channel-сущности (часть B.1).
- `audit_log` для security events — отдельная фича `F11-audit`.
- Reset-password flow — нужна email infrastructure, отложено.
- 2FA / TOTP — нет в DEVELOPMENT_PLAN, не делаем.
- `jtpost auth login` команда (interactive PAT-creation через temporary password) — отложена в `F4b`.

**Триггер:** F1-foundation создал `auth.tenant_default`/`author_default` как заглушку. F2 ввёл storage parity. F3 git decorator. F4 закрывает «hardcoded single-user» режим — теперь несколько пользователей могут работать через API с собственными PAT, а HTTP-сервер enforces auth.

---

## Investigation

### Что уже есть

**F1-заглушка middleware** (`internal/adapters/httpapi/middleware.go:13-26`):
- `TenantFromConfigMiddleware(cfg)` — кладёт `cfg.Auth.TenantDefault`/`AuthorDefault` в context. Реальная аутентификация отсутствует.
- F4 заменит его на `BearerTokenMiddleware(authService)`.

**Config Auth schema** (F1, `internal/adapters/config/config.go:83-90`):
```go
type AuthConfig struct {
    Type          string        // "none" | "basic" | "oauth" | "token"
    Secret        string        // JWT signing secret (опц.)
    TenantDefault uuid.UUID
    AuthorDefault uuid.UUID
    OAuth         OAuthConfig
    TokenTTL      time.Duration
}
```
- `auth.type` уже валидируется но не имеет behaviour.
- F4: `type=none` → F1-заглушка остаётся; `type=token` → BearerTokenMiddleware включается.

**Storage Layer** (F2):
- `storage.Open(cfg)` возвращает `core.PostRepository` — пользователи и токены **не** часть post-репо.
- F4 нужен **отдельный** `core.UserRepository` / `core.TokenRepository`. Или — расширение storage factory под несколько репозиториев. Choice-point: см. Options.

**Storage backend support:**
- FS-репо для users/tokens — не имеет смысла (бинарные значения, частые reads). Только sqlite/postgres.
- При `storage.type=fs` → users-функционал недоступен → если `auth.type=token` это противоречие → fail at Validate.

**Текущие CLI**:
- `jtpost init` создаёт `tenant_default`/`author_default` в конфиге. F4 после миграции — генерирует первого `owner`-юзера для тенанта.
- `jtpost serve` запускает HTTP-сервер с F1-middleware. F4 включает BearerToken.

### Зависимости

- **`golang.org/x/crypto/bcrypt`** — для password hashing (стандарт Go-сообщества; `cost=10..14`).
- **PAT design**: prefix-based с lookup-able prefix + hash полного токена.
  - Format: `jtpat_<8-char-prefix>_<24-char-secret>` (e.g., `jtpat_a1b2c3d4_xxxxxxxxxxxxxxxxxxxxxxxx`).
  - В БД хранится `prefix` (8 chars, indexable, lookup) + `secret_hash` (bcrypt(`secret`)).
  - При проверке: split → SELECT WHERE prefix=? → bcrypt.CompareHashAndPassword(secret_hash, secret).
  - Это стандартный pattern (GitHub, GitLab, etc.).
- **JWT не нужен** в F4a — у нас server-side lookup PAT. JWT появится с sessions (F4c).

### Тестовый контекст

- Стиль: native testing + table-driven (как в F1/F2/F3).
- **Bcrypt в тестах**: использовать `bcrypt.MinCost (4)` — иначе test suite будет тормозить (default cost 10 = ~100ms на hash).
- Integration-тесты PAT через httpapi-handlers + in-memory user-repo.

### Архитектурные точки

- `internal/core/user.go` — domain types (User, Role, Permission, APIToken).
- `internal/core/auth_service.go` — `AuthService.Login`, `AuthorizeOperation`, `IssueToken`, `ValidateToken`.
- `internal/core/user_repository.go` — interface `UserRepository`, `TokenRepository`.
- `internal/adapters/sqlite/users.go` (или новые миграции `0002_users.sql`) — sqlc-queries для users, tokens.
- `internal/adapters/postgres/users.go` — аналог.
- `internal/adapters/storage/factory.go` — расширить `Open` чтобы возвращал `(PostRepository, UserRepository, TokenRepository, io.Closer, error)` или ввести `Bundle` struct.
- `internal/adapters/httpapi/middleware.go` — заменить `TenantFromConfigMiddleware` на `BearerTokenMiddleware(authService)` или добавить новый и оставить старый как fallback при `auth.type=none`.
- `internal/cli/user.go`, `internal/cli/token.go` — новые команды.
- `internal/adapters/config/config.go` — `auth.bcrypt_cost int` поле.

### Что F4 НЕ затрагивает

- Postgres миграции в F4 нужны (новые таблицы `users`, `tokens`) — это часть scope.
- `gitrepo` decorator — без изменений.
- `httpapi/server.go` хендлеры — потребляют `core.PostService`, не нуждаются в auth-знании (auth через middleware → ctx).

---

## Build Tooling

- **Orchestrator:** Task (`Taskfile.yml`).
- **Test:** `task test` (unit), `task test:integration` (Postgres).
- **Build:** `task build`.
- **Lint:** `task lint`.
- **Generate:** `task generate` (sqlc — для новых users/tokens queries).
- **Source:** `Taskfile.yml`, `sqlc.yaml`.

CI: без изменений (integration-тесты через testcontainers поддерживают новые таблицы).

---

## Options Considered

### Option A: Один storage Bundle с UserRepository + TokenRepository

`storage.Open(cfg)` возвращает `*Bundle{Posts, Users, Tokens, Closer}`. Все CLI и HTTP получают bundle.

- **Pros:** единый lifecycle, один Close, простая миграция при добавлении новых entities.
- **Cons:** breaking change — все вызовы `storage.Open` обновлены; больший diff.
- **Сложность:** Medium.

### Option B: Отдельные factory-функции

`storage.OpenPosts(cfg)`, `storage.OpenUsers(cfg)`, `storage.OpenTokens(cfg)`. Каждый CLI вызывает только нужное.

- **Pros:** zero breaking — F2 `Open(cfg)` остаётся; новые функции — additive.
- **Cons:** дублирование connection-init (для sqlite/postgres каждый Open создаёт новый pool); потенциально 3× connection вместо 1×.
- **Сложность:** Low (по структуре), но wasteful.

### Option C: Расширить core.PostRepository интерфейс ещё методами

Добавить `GetUser`, `IssueToken` напрямую в `PostRepository`.

- **Pros:** минимально для caller.
- **Cons:** SRP violated; адаптер должен знать про auth; не работает для FS (FS не имеет users).
- **Сложность:** High по проблемам.

### Option D: Auth как отдельный сервис, обращающийся к storage через свой connection

`auth.NewService(cfg)` сама открывает sqlite/postgres pool отдельно, не через storage factory.

- **Pros:** изоляция, auth-service может использовать другой storage type если хочется.
- **Cons:** двойной pool, сложно для пользователя (должен синхронизировать DSN в двух местах).
- **Сложность:** Medium.

---

## Constraints & Risks

### Backward compatibility

- F4 при `auth.type=none` оставляет F1-поведение: middleware читает `tenant_default`/`author_default` из конфига. Существующие deployments не ломаются.
- `auth.type=token` включает BearerToken. Если БД не содержит users — 401 на любой запрос (до создания первого юзера через `jtpost user create`).
- Новые таблицы `users`, `tokens` создаются миграцией `0002_*.sql` для sqlite + postgres. F1-M1 фиксирует «prod-данных нет», поэтому миграция простая `CREATE TABLE`.

### Storage type compatibility

- `auth.type=token && storage.type=fs` → НЕ поддерживается (валидация). Сообщение: «token-auth requires sqlite or postgres».
- При `auth.type=none` любой storage type — fine.

### Security

- **Password hashing**: bcrypt cost=10 default; в тестах — MinCost=4.
- **PAT storage**: в БД — только bcrypt-hash, не cleartext. После создания токен показывается пользователю один раз; дальше — не recoverable.
- **Timing attacks**: bcrypt.CompareHashAndPassword — constant-time. Lookup token by prefix — потенциально timing-attackable, но prefix 8 chars random => 2^32 entropy, что достаточно для сужения lookup.
- **Token revocation**: hard delete из БД. Нет blacklist'а — сразу.
- **Rate limiting**: НЕ в F4 (отложено в F11/maintenance). Brute-force защита — через bcrypt cost.
- **Session fixation**: not applicable (no sessions in F4a).
- **CSRF**: not applicable (no cookies in F4a; PAT в Authorization header).
- **Secrets в logs**: bearer-token, password — НЕ должны логироваться. Маска `Bearer ***`.

### Concurrency

- User creation: SQL `UNIQUE(email, tenant_id)` → race-condition при concurrent create → второй получит ErrAlreadyExists. ОК.
- Token creation: prefix collision вероятность 1/2^32 — игнорируем, но `UNIQUE(prefix)` constraint спасёт от данных.

### Performance

- Bearer-middleware на КАЖДЫЙ HTTP-запрос делает SQL SELECT + bcrypt. Bcrypt cost=10 — ~100ms. Это медленно для read-heavy API.
  - **Mitigation**: in-memory cache для validated tokens (TTL 60s). НЕ в F4 (отложено).
  - **Альтернатива**: уменьшить cost до 6 для PAT (PAT-secret уже 24-char random — собственная entropy достаточна, bcrypt здесь только защищает от БД-leak). cost=6 = ~10ms.
  - Решение: cost для PAT-secret = 6 (фиксированно, не из конфига). cost для password = из конфига.

### Edge cases

- **Bootstrap**: первый user должен быть создан без auth (БД пуста). Решение: `jtpost user create --first-owner` команда, которая работает ТОЛЬКО если в users-table 0 записей. Дальше — требует existing PAT.
- **Removed user with active tokens**: cascade delete (SQL FK ON DELETE CASCADE).
- **Expired tokens**: проверка `expires_at < now()` в middleware → 401. БД retains expired tokens до cleanup-команды (отложен).
- **Multi-tenant in users**: каждый user принадлежит одному `tenant_id`. Cross-tenant — отдельная фича.
- **Email format validation**: regex / `mail.ParseAddress` — простая.

---

## Recommended Direction

**Option A (Bundle)** — наиболее чисто:

1. **Domain** — `internal/core/user.go`, `auth_service.go`, `user_repository.go`.
2. **Schema migrations** — sqlite + postgres `0002_users.sql` (таблицы `users`, `tokens`, FK `users.tenant_id` → no FK to tenants table в F4 потому что её нет).
3. **sqlc queries** — `users.sql`, `tokens.sql` для обоих диалектов.
4. **Adapters** — обновить `internal/adapters/{sqlite,postgres}` чтобы реализовывали `core.UserRepository` и `core.TokenRepository`.
5. **Storage Bundle** — `storage.Open(cfg) (*Bundle, error)` где `Bundle{Posts, Users, Tokens, Closer}`. F2-сигнатура `Open(cfg) (PostRepository, io.Closer, error)` — оставить как `OpenPosts(cfg)` shim для CLI-команд которые работают только с posts (новые команды auth используют Bundle).
6. **AuthService** — `core.AuthService` с методами `CreateUser/VerifyPassword/IssueToken/RevokeToken/ValidateToken/AuthorizeOperation`.
7. **Middleware** — `httpapi.BearerTokenMiddleware(authService)`. При `auth.type=token` подключается к chain. При `auth.type=none` — fallback на F1-заглушку.
8. **CLI** — новые команды `jtpost user {create,list,delete}`, `jtpost token {create,list,revoke}`. Каждая берёт Bundle через factory.
9. **`Config.Validate()` extension** — `auth.type=token && storage.type=fs` → ErrConfigInvalid.

Implementation order: domain → schema → sqlc → adapters → AuthService → Bundle factory → middleware → CLI → migrations from F1-stub.

---

## Scope Boundaries

### Must-have (v1, эта фича)

- `core.User`, `core.Role`, `core.Permission`, `core.APIToken` types.
- `core.AuthService` (CreateUser, VerifyPassword, IssueToken, RevokeToken, ValidateToken, AuthorizeOperation).
- `core.UserRepository` + `core.TokenRepository` interfaces.
- SQLite + Postgres адаптеры реализуют обе.
- goose-миграция `0002_users.sql` для обоих диалектов.
- sqlc-queries для users, tokens (обе схемы).
- `storage.Bundle{Posts, Users, Tokens, Closer}` + `Open(cfg) (*Bundle, error)`.
- `httpapi.BearerTokenMiddleware(authService)` + интеграция в `serve.go`.
- `jtpost user create/list/delete` (с `--first-owner` bootstrap для нулевого пользователя).
- `jtpost token create/list/revoke`.
- `Config.Auth.BCryptCost` поле + `Validate()` для type=token vs storage type.
- RBAC roles (`owner`, `editor`, `author`, `viewer`) с map permissions.
- Тесты: unit для AuthService, Repository contract; integration HTTP для middleware.
- `repotest.RunContract` для FS не запускает users-тесты (FS не реализует UserRepository).

### Deferred (v2 / последующие фичи)

- **F4b OAuth2** — GitHub/Google/Yandex через `coreos/go-oidc`.
- **F4b Argon2id** — миграция bcrypt → Argon2id для password hashes (одна миграция вместо двух).
- **F4c sessions/cookies** — для Web UI (F8). Включает CSRF.
- **F4c `jtpost auth login` interactive command** — login через email+password → выдаёт session cookie.
- **F11 audit_log** — security events (failed logins, token issuance/revocation).
- **Per-channel RBAC** — после появления channels-сущности.
- **Token caching** — in-memory кеш validated tokens (60s TTL) для performance.
- **Token cleanup CLI** — `jtpost token prune --expired-before 30d`.
- **Email-based password reset** — нужна email infrastructure.
- **2FA / TOTP** — out of scope.
- **Rate limiting** — F11.

### Needs spike

- **Permission granularity per-channel**: после появления channels — нужно spike, как `User.Roles` маппятся на `(channel, role)`-pairs. F4 же делает global roles (без channels).
- **Token lookup performance**: prefix=8 chars даёт 2^32 entropy; SQL UNIQUE(prefix) обеспечивает O(1) lookup. Достаточно? Verifиkировать на нагрузке (но F4 — pre-prod, не критично).

---

## Assumptions & Open Questions

### Assumptions

- `[ASSUMPTION: bcrypt вместо Argon2id в F4a]` — упрощение; миграция в F4b будет single-step DROP+ADD column.
- `[ASSUMPTION: PAT-secret bcrypt cost = 6, password bcrypt cost = из конфига (default 10)]` — компромисс между security и middleware-performance.
- `[ASSUMPTION: Один user — один tenant в F4]` — multi-tenant per-user (например, owner и editor разных tenants) отложено.
- `[ASSUMPTION: Bootstrap: jtpost user create --first-owner работает только при пустой БД]` — после первого user всё через PAT.
- `[ASSUMPTION: Storage Bundle struct — основная точка входа; F2 Open(cfg) сохранён как Bundle.Posts shim для backward-compat CLI команд]`.
- `[ASSUMPTION: При auth.type=none сохраняется F1-заглушка; при auth.type=token включается BearerTokenMiddleware]`.
- `[ASSUMPTION: Token format jtpat_<8prefix>_<24secret>]` — стандартный pattern, читаемый.
- `[ASSUMPTION: При auth.type=token и storage.type=fs — Validate fail]` — fs не подходит для users.

### Open Questions (для Requirements)

1. **Bootstrap-команда**: `jtpost user create --first-owner` или `jtpost auth bootstrap`? Предложение: `--first-owner` flag к `jtpost user create`.
2. **PAT expiration default**: бессрочный (NULL) или 90 дней? Предложение: NULL по умолчанию, но `--expires-in 90d` как опция.
3. **Username vs email**: только email или username тоже? Предложение: только email (стандарт).
4. **Password complexity rules**: enforce min length, complexity? Предложение: min 8 chars, без других правил (UX).
5. **`jtpost token create` вывод**: показать токен plain-text один раз и ничего не сохранять локально, или сохранить в `~/.jtpost/tokens` (для CLI authentication)? Предложение: показать один раз + опц. `--save-to-config` пишет в `.jtpost.yaml` (deprecated path).
6. **Список ролей фиксированный или конфигурируемый**? Предложение: фиксированный hardcoded `owner|editor|author|viewer` в F4. Custom — после.
7. **Permissions грануляции в F4**: per-operation (create_post, edit_post, delete_post, publish_post, manage_users)? Предложение: 5-7 базовых permissions.
8. **CLI `jtpost serve` + `auth.type=token`**: middleware включается автоматически? Предложение: ДА, по `cfg.Auth.Type`.

---

## Done When (для approve)

- [x] Codebase прочитан (F1 middleware shim cited; F2 storage factory; Auth schema из config.go).
- [x] Сравнены 4 опции (A/B/C/D).
- [x] Trade-off'ы явные.
- [x] Scope boundaries (Must / Deferred / Needs spike) — категоризированы.
- [x] Assumptions помечены.
- [x] Open Questions есть (8 пунктов).
- [x] Build Tooling зафиксирован (без новых требований).
- [ ] Артефакт зарегистрирован — следующий шаг.
