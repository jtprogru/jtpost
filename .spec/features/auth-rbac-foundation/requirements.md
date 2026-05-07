# Auth/RBAC Foundation (F4) — Requirements

**Status:** Draft
**Author:** Claude (Opus 4.7) + Mikhail Savin
**Date:** 2026-05-07
**Feature:** auth-rbac-foundation (F4a)
**Branch:** `feature/auth-rbac-foundation`

## Overview

F4a даёт jtpost первое работающее auth-решение: локальные пользователи (email + bcrypt password), Personal Access Tokens (PAT) с server-side lookup (`jtpat_<8prefix>_<24secret>`), и RBAC scaffold с 4 hardcoded ролями (`owner`, `editor`, `author`, `viewer`) и 6 permissions. Storage layer расширяется до `storage.Bundle{Posts, Users, Tokens, Closer}` — single open per process, cohesive lifecycle. HTTP-сервер при `auth.type=token` использует `BearerTokenMiddleware`, который заменяет F1-заглушку `TenantFromConfigMiddleware`. CLI получает `jtpost user create/list/delete` (с `--first-owner` bootstrap) и `jtpost token create/list/revoke`. `auth.type=token` требует `storage.type ∈ {sqlite, postgres}` (FS не подходит для users-таблиц). OAuth2, Argon2id, sessions/cookies, audit_log — отложены в F4b/F4c.

## Glossary

| Term | Definition | Code Artifact |
|------|------------|---------------|
| `User` | Сущность учётной записи: ID, email, password_hash, role, tenant_id, timestamps | `internal/core/user.go` |
| `APIToken` | Personal Access Token: ID, user_id, prefix, secret_hash, name, expires_at, last_used_at | `internal/core/user.go` |
| `Role` | Роль (`owner` \| `editor` \| `author` \| `viewer`) с маппингом на permissions | `internal/core/auth.go` |
| `Permission` | Разрешение: `posts:create`, `posts:edit`, `posts:delete`, `posts:publish`, `users:manage`, `tokens:manage` | `internal/core/auth.go` |
| `AuthService` | Доменный сервис: CreateUser, VerifyPassword, IssueToken, RevokeToken, ValidateToken, AuthorizeOperation | `internal/core/auth_service.go` |
| `UserRepository` | Интерфейс адаптера: GetByID, GetByEmail, Create, Update, Delete, List, Count | `internal/core/user_repository.go` |
| `TokenRepository` | Интерфейс: GetByPrefix, Create, Delete, ListByUser, UpdateLastUsedAt | `internal/core/user_repository.go` |
| `Bundle` | Единый объект, объединяющий PostRepository + UserRepository + TokenRepository + Closer | `internal/adapters/storage/factory.go` |
| `BearerTokenMiddleware` | HTTP middleware: парсит Authorization header, валидирует PAT, кладёт user/tenant/role в context | `internal/adapters/httpapi/middleware.go` |
| `UserContextKey` | Ключ для извлечения текущего User из context | `internal/core/scope.go` |
| `RoleContextKey` | Ключ для извлечения Role | `internal/core/scope.go` |
| `tokenFormatPrefix` | Константа `"jtpat_"` (jtpost personal access token) | `internal/core/auth.go` |
| `firstOwnerFlag` | CLI-флаг `--first-owner` для bootstrap первого пользователя при пустой users-таблице | `internal/cli/user.go` |

## User Stories

- Как **владелец канала-в-команде**, я хочу создать первого пользователя через `jtpost user create --first-owner --email me@x.com`, чтобы получить admin-доступ без существующего PAT.
- Как **командный editor**, я хочу залогиниться в API через PAT в Authorization header, чтобы CLI и интеграции могли работать от моего имени.
- Как **владелец**, я хочу создавать новые PAT с именами для каждой интеграции и отзывать их по необходимости, чтобы контролировать доступ.
- Как **разработчик-сопровождающий**, я хочу, чтобы middleware валидировал PAT и клал User/Role в context, чтобы хендлеры могли проверять permissions.
- Как **API-консьюмер**, я хочу 401 Unauthorized при missing/invalid токене, и 403 Forbidden при недостатке permissions, чтобы понимать тип ошибки.
- Как **владелец**, я хочу что бы expired-токены автоматически отклонялись, чтобы implementing access-rotation было безопасно.

## Requirements

### Group 1 — Domain types

**REQ-1.1** WHEN модуль `core` определяет тип `User`, the system SHALL содержать поля `ID uuid.UUID`, `TenantID uuid.UUID`, `Email string`, `PasswordHash string`, `Role Role`, `CreatedAt time.Time`, `UpdatedAt time.Time`.

**REQ-1.2** WHEN модуль `core` определяет тип `APIToken`, the system SHALL содержать поля `ID uuid.UUID`, `UserID uuid.UUID`, `Prefix string` (8 символов), `SecretHash string`, `Name string`, `CreatedAt time.Time`, `ExpiresAt *time.Time`, `LastUsedAt *time.Time`.

**REQ-1.3** WHEN модуль `core` определяет тип `Role`, the system SHALL экспортировать константы `RoleOwner = "owner"`, `RoleEditor = "editor"`, `RoleAuthor = "author"`, `RoleViewer = "viewer"`.

**REQ-1.4** WHEN модуль `core` определяет тип `Permission`, the system SHALL экспортировать строковые константы для 6 permissions: `posts:create`, `posts:edit`, `posts:delete`, `posts:publish`, `users:manage`, `tokens:manage`.

**REQ-1.5** WHEN модуль `core` экспортирует функцию `RolePermissions(role Role) []Permission`, the system SHALL возвращать:
- `owner` → все 6 permissions
- `editor` → `posts:create`, `posts:edit`, `posts:delete`, `posts:publish`
- `author` → `posts:create`, `posts:edit`
- `viewer` → ∅ (read-only)

### Group 2 — AuthService API

**REQ-2.1** WHEN `AuthService.CreateUser(ctx, input CreateUserInput)` вызывается, the system SHALL хешировать пароль через `bcrypt.GenerateFromPassword(password, cfg.Auth.BCryptCost)` и записать `User` в repository.

**REQ-2.2** WHEN `AuthService.CreateUser` вызывается с пустым email или паролем короче 8 символов, the system SHALL вернуть `core.ErrValidation`.

**REQ-2.3** WHEN `AuthService.CreateUser` вызывается с email, который уже существует в том же tenant, the system SHALL вернуть `core.ErrAlreadyExists`.

**REQ-2.4** WHEN `AuthService.VerifyPassword(ctx, email, password)` вызывается, the system SHALL найти user по email + tenant из ctx, выполнить `bcrypt.CompareHashAndPassword`, и вернуть `*User` при success или `core.ErrUnauthorized` при failure.

**REQ-2.5** WHEN `AuthService.IssueToken(ctx, userID uuid.UUID, name string, expiresIn *time.Duration)` вызывается, the system SHALL:
- сгенерировать random 8-char prefix и 24-char secret;
- bcrypt-хешировать secret с `cost=6` (фиксированный);
- сохранить `APIToken` в repository;
- вернуть структуру `IssuedToken{Token: "jtpat_<prefix>_<secret>", APIToken: stored}`.

**REQ-2.6** WHEN `AuthService.ValidateToken(ctx, raw string)` вызывается с строкой формата `jtpat_<prefix>_<secret>`, the system SHALL:
- распарсить prefix и secret;
- найти `APIToken` по prefix через repository;
- проверить bcrypt-hash secret;
- проверить `ExpiresAt > time.Now()` (если не nil);
- найти соответствующего User;
- вернуть `(User, Role)`;
- иначе — `core.ErrUnauthorized`.

**REQ-2.7** WHEN `AuthService.ValidateToken` успешно валидирует, the system SHALL обновить `APIToken.LastUsedAt = time.Now()` асинхронно (через goroutine), не блокируя возврат.

**REQ-2.8** WHEN `AuthService.RevokeToken(ctx, tokenID uuid.UUID)` вызывается, the system SHALL удалить запись из repository (hard delete).

**REQ-2.9** WHEN `AuthService.AuthorizeOperation(ctx, perm Permission)` вызывается, the system SHALL извлечь Role из ctx, проверить `RolePermissions(role)` на наличие `perm`, вернуть `nil` или `core.ErrForbidden`.

**REQ-2.10** WHEN формат raw-токена не соответствует `jtpat_<8>_<24>`, the system SHALL вернуть `core.ErrUnauthorized` без обращения к БД.

### Group 3 — Repository интерфейсы и адаптеры

**REQ-3.1** WHEN модуль `core` определяет интерфейс `UserRepository`, the system SHALL содержать методы: `GetByID(ctx, id) (*User, error)`, `GetByEmail(ctx, tenantID uuid.UUID, email string) (*User, error)`, `Create(ctx, user)`, `Update(ctx, user)`, `Delete(ctx, id)`, `List(ctx, tenantID) ([]*User, error)`, `Count(ctx, tenantID) (int64, error)`.

**REQ-3.2** WHEN модуль `core` определяет интерфейс `TokenRepository`, the system SHALL содержать методы: `GetByPrefix(ctx, prefix string) (*APIToken, error)`, `Create(ctx, token)`, `Delete(ctx, id)`, `ListByUser(ctx, userID) ([]*APIToken, error)`, `UpdateLastUsedAt(ctx, id, t time.Time)`.

**REQ-3.3** WHEN SQLite-адаптер вызывает методы UserRepository/TokenRepository, the system SHALL использовать sqlc-сгенерированные queries для всех операций (кроме UpdateLastUsedAt — допустим ручной exec для скорости).

**REQ-3.4** WHEN Postgres-адаптер вызывает методы UserRepository/TokenRepository, the system SHALL соблюдать тот же поведенческий контракт что SQLite.

**REQ-3.5** WHEN repository вызывает `GetByEmail` или `GetByPrefix` и записи нет, the system SHALL вернуть `core.ErrNotFound`.

**REQ-3.6** WHEN goose-миграция применяется, the system SHALL создать таблицы `users(id, tenant_id, email, password_hash, role, created_at, updated_at)` с `UNIQUE(tenant_id, email)` и `tokens(id, user_id, prefix, secret_hash, name, created_at, expires_at, last_used_at)` с `UNIQUE(prefix)` и `FK ON DELETE CASCADE` в обоих диалектах.

### Group 4 — Storage Bundle

**REQ-4.1** WHEN пакет `internal/adapters/storage` экспортирует тип `Bundle`, the system SHALL содержать поля `Posts core.PostRepository`, `Users core.UserRepository`, `Tokens core.TokenRepository`, `Closer io.Closer`.

**REQ-4.2** WHEN `storage.Open(cfg) (*Bundle, error)` вызывается, the system SHALL открыть один storage backend (по `cfg.Storage.Type`) и заполнить все три репозитория из него (для `fs` — `Users`/`Tokens` равны `nil`, см. REQ-4.5).

**REQ-4.3** WHEN существующая сигнатура `storage.OpenAs(cfg, type)` сохраняется, the system SHALL продолжать работать как F2-shim (возвращает `(PostRepository, io.Closer, error)` — wrapper над `Bundle`), чтобы не ломать вызовы из `cli/migrate.go`.

**REQ-4.4** WHEN `Bundle.Closer.Close()` вызывается, the system SHALL закрыть все ресурсы единократно.

**REQ-4.5** WHEN `cfg.Storage.Type == "fs"`, the system SHALL установить `Bundle.Users = nil` и `Bundle.Tokens = nil` (FS не поддерживает users); вызовы CLI-команд auth при этом возвращают понятную ошибку.

### Group 5 — Config validation и Bootstrap

**REQ-5.1** WHEN `Config.Auth` определяется, the system SHALL содержать новое поле `BCryptCost int` со значением default `10` (через `NewDefaultConfig`).

**REQ-5.2** WHEN `Config.Validate()` вызывается с `Auth.Type == "token"` и `Storage.Type == "fs"`, the system SHALL вернуть `errors.Join(core.ErrConfigInvalid, errors.New("auth.type=token requires storage.type=sqlite or postgres"))`.

**REQ-5.3** WHEN `Config.Validate()` вызывается с `Auth.BCryptCost < 4` или `> 14`, the system SHALL вернуть `core.ErrConfigInvalid`.

**REQ-5.4** WHEN `Config.Auth.Type` имеет значение, отличное от `"none" | "token"`, the system SHALL вернуть `core.ErrConfigInvalid` (OAuth/basic — не реализованы в F4a).

### Group 6 — HTTP middleware

**REQ-6.1** WHEN HTTP-сервер запускается с `cfg.Auth.Type == "token"`, the system SHALL подключить `BearerTokenMiddleware(authService)` ВМЕСТО `TenantFromConfigMiddleware`.

**REQ-6.2** WHEN HTTP-сервер запускается с `cfg.Auth.Type == "none"`, the system SHALL подключить `TenantFromConfigMiddleware` (F1-поведение, backward-compat).

**REQ-6.3** WHEN `BearerTokenMiddleware` обрабатывает запрос без заголовка `Authorization` или с невалидным форматом, the system SHALL вернуть HTTP 401 с телом `{"error":"unauthorized"}` и НЕ вызывать handler.

**REQ-6.4** WHEN `BearerTokenMiddleware` валидирует токен через `AuthService.ValidateToken`, the system SHALL положить в `context.Context` значения: `User` (через новый `core.WithUser`), `TenantID` (через `core.WithTenant`), `Role` (через новый `core.WithRole`).

**REQ-6.5** WHEN `BearerTokenMiddleware` встречает expired или invalid token, the system SHALL вернуть HTTP 401 (не 403).

**REQ-6.6** WHEN `BearerTokenMiddleware` логирует обработку запросов, the system SHALL маскировать токен в логах: `Authorization: Bearer ***`.

### Group 7 — CLI commands

**REQ-7.1** WHEN команда `jtpost user create --email <e> --password <p> --role <r>` запускается и в БД 0 пользователей, the system SHALL отклонить запрос с подсказкой использовать `--first-owner` (REQ-7.2).

**REQ-7.2** WHEN команда `jtpost user create --first-owner --email <e> --password <p>` запускается и `Bundle.Users.Count(ctx, tenantID) == 0`, the system SHALL создать первого `owner`-пользователя БЕЗ требования аутентификации.

**REQ-7.3** WHEN команда `jtpost user create --first-owner` запускается и пользователи уже существуют, the system SHALL вернуть ошибку «first owner already exists».

**REQ-7.4** WHEN команда `jtpost user list` запускается, the system SHALL вывести таблицу всех users в текущем tenant (id, email, role, created_at).

**REQ-7.5** WHEN команда `jtpost user delete <user-id>` запускается, the system SHALL удалить пользователя и cascade-удалить его tokens (через FK).

**REQ-7.6** WHEN команда `jtpost token create --user-id <id> --name <name> [--expires-in <duration>]` запускается, the system SHALL вызвать `AuthService.IssueToken` и вывести `Token: jtpat_...` в stdout с предупреждением «save this token, it will not be shown again».

**REQ-7.7** WHEN команда `jtpost token list --user-id <id>` запускается, the system SHALL вывести список tokens пользователя (id, name, prefix, created_at, expires_at, last_used_at) — БЕЗ secret_hash.

**REQ-7.8** WHEN команда `jtpost token revoke <token-id>` запускается, the system SHALL удалить токен из БД.

**REQ-7.9** WHEN CLI-команды auth (`user`/`token`) запускаются с `cfg.Storage.Type == "fs"`, the system SHALL вернуть ошибку «user management requires storage.type=sqlite or postgres».

### Group 8 — Тесты

**REQ-8.1** WHEN запускается `task test`, the system SHALL пройти unit-тесты `core/auth_service_test.go` (CreateUser/Verify/Issue/Validate/Revoke/Authorize) с использованием mock-репозиториев.

**REQ-8.2** WHEN запускается `task test`, the system SHALL пройти SQLite-адаптер тесты для UserRepository и TokenRepository (CRUD, GetByEmail, GetByPrefix, cascade delete).

**REQ-8.3** WHEN запускается `task test:integration`, the system SHALL пройти Postgres-адаптер тесты те же, что для SQLite (через testcontainers).

**REQ-8.4** WHEN запускается `task test`, the system SHALL пройти интеграционный HTTP-тест `BearerTokenMiddleware`: валидный PAT → 200 + ctx populated, invalid → 401, expired → 401, missing header → 401.

**REQ-8.5** WHEN запускается `task test`, the system SHALL включать тест что bcrypt cost для PAT-secret = 6 (через `bcrypt.Cost([]byte(hash))`).

## Topological Order

```
Group 1 (Domain types)         — фундамент
       ↓
Group 5 (Config + Bootstrap)   — расширение конфига и валидация
       ↓
Group 3 (Repository) + миграции — interface + sql implementation
       ↓
Group 2 (AuthService)          — использует repositories
       ↓
Group 4 (Storage Bundle)       — wiring repository через factory
       ↓
Group 6 (HTTP middleware)      — использует AuthService + Bundle
       ↓
Group 7 (CLI commands)         — потребитель Bundle + AuthService
       ↓
Group 8 (Тесты)                — финал, покрывает все группы
```

## Conflict Priority

**Конфликт 1.** REQ-7.2 (`--first-owner` создаёт user без auth) vs REQ-6.3 (middleware требует токен).

**Resolution:** CLI команды НЕ проходят через HTTP-middleware. Они работают напрямую с `Bundle` и `AuthService` локально. `--first-owner` — CLI-only feature. HTTP-эндпоинт для первого owner НЕ предоставляется в F4a (deferred).

**Конфликт 2.** REQ-2.7 (LastUsedAt async update) vs ctx-cancellation (если запрос отменён до UpdateLastUsedAt).

**Resolution:** Goroutine использует `context.Background()` — не привязана к request-ctx. Это сознательно: пометить «использовали токен» важно даже если запрос провалился.

## Open Design Questions

| Question | Why It Matters | Impacted Requirements |
|----------|---------------|----------------------|
| `User.Role` хранится как одиночное поле или массив (multi-role)? | Влияет на schema migrations и API. | REQ-1.1, REQ-3.6 |
| `tenant_id` для User: тот же что `cfg.Auth.TenantDefault` или отдельная сущность? | F4 не вводит tenants-таблицу, но user.tenant_id должен быть consistent. | REQ-1.1, REQ-7.2 |
| `BCryptCost` валидация в `Validate()` — диапазон 4..14 или 10..14 (security-baseline)? | Влияет на performance тестов. | REQ-5.3 |
| Token-prefix collision: retry на UNIQUE-violation или fail? | Очень редкое событие (1/2^32), но детерминированное поведение нужно. | REQ-2.5 |
| `jtpost user delete` для последнего owner: разрешить или запретить? | Иначе можно потерять admin-доступ. | REQ-7.5 |
| `jtpost user list` показывать `password_hash`? | НЕТ. Но нужно явно зафиксировать. | REQ-7.4 |

## Verification Commands

| Action               | Command                          | Source         |
|----------------------|----------------------------------|----------------|
| Test (unit)          | `task test`                      | `Taskfile.yml` |
| Test (race)          | `task test:race`                 | `Taskfile.yml` |
| Test (integration)   | `task test:integration`          | `Taskfile.yml` |
| Build                | `task build`                     | `Taskfile.yml` |
| Lint                 | `task lint`                      | `Taskfile.yml` |
| Generate (sqlc)      | `task generate`                  | `Taskfile.yml` |
| Format / Vet         | `task fmt`, `task vet`           | `Taskfile.yml` |
