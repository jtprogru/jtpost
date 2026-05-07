# Implementation Report: Foundation — Domain Model & Configuration (F1)

## Summary

Полностью реализовано расширение домена и конфигурации согласно task-plan.md (T-1 … T-8). Все 8 top-level задач (~36 подзадач) выполнены. 51 файл изменён, +3343 / −1785 строк.

Ключевые результаты:
- Доменная модель `Post` расширена 5 обязательными полями (`TenantID`, `AuthorID`, `CreatedAt`, `UpdatedAt`, `Revision`) и 5 опциональными (`Excerpt`, `CoverImage`, `Attachments`, `PublishHistory`, `RevisionSHA`).
- 7 статусов с явной таблицей переходов (`allowedTransitions`), 10 разрешённых переходов с поддержкой откатов.
- Конфиг `Config` расширен 4 новыми секциями (`Storage`, `Auth`, `Worker`, `Server`) с поддержкой env-override через viper и кастомным DecodeHook для `time.Duration` и `uuid.UUID`.
- FS-репозиторий теперь хранит посты в `<posts_dir>/<tenant_short_id>/<slug>.md`. Tenant scope enforcement через `core.WithTenant(ctx, ...)`.
- HTTP API получил middleware `TenantFromConfigMiddleware` (заглушка под F4), `jsonPost` отражает все новые поля, enforcement tenant immutability на PATCH.
- CLI: интерактивный `jtpost init` с `--force`, генерация UUIDv7 для `tenant_default`/`author_default` с проверкой коллизии префикса, `jtpost new --tenant/--author`, `jtpost list --format json` (закрытие TODO), удалена legacy `getService` из `root.go`.
- Все 65 REQ из requirements.md покрыты тестами; 49 unit + 15 PBT-substitute (тегированы `Property/N`).

## Commands Used

- **Test:** `task test` (`go test -v -coverprofile=cover.out ./...`)
- **Race:** `task test:race` (`go test -race -v ./...`)
- **Build:** `task build` (`CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go`)
- **Lint:** `task lint` (`golangci-lint run -v`)
- **Format:** `task fmt` (`gofmt -s -w .`)
- **Vet:** `task vet` (`go vet ./...`)

## Task Execution

- [x] **T-1** Расширить доменные типы `Post`, `PostFilter`, добавить `Attachment`/`PublishAttempt`/новые статусы — GREEN. 6 подзадач выполнены: добавлен пакет `internal/core/scope.go` с context-keys (T-4.3a), переписан `core_test.go` под полный декартов перебор статусов (CP-4), создан `post_test.go` с round-trip тестами (CP-6, CP-8). Удалена `StatusOrder`, добавлены `IsTransitionAllowed`, `AllStatuses`. Новые ошибки: `ErrInvalidTransition`, `ErrTenantMismatch`, `ErrPublishRetryExhausted`.
- [x] **T-2** Расширить `PostService` под новые контракты — GREEN. CreatePost требует non-zero TenantID/AuthorID, UpdatePost инкрементирует Revision и проверяет tenant immutability, добавлены `Archive`, `MarkFailed`, `AppendPublishAttempt`. `mockRepository` в тестах теперь возвращает копии для предотвращения side-effects. Все service-тесты зелёные.
- [x] **T-3** Расширить `Config` под новые секции — GREEN (делегировано subagent'у). Добавлены `StorageConfig`, `GitStorageConfig`, `PostgresConfig`, `AuthConfig`, `OAuthConfig`, `WorkerConfig`, `ServerConfig`. Реализован `uuidDecodeHook` через `github.com/go-viper/mapstructure/v2`. 30+ env-bind вызовов. Validate отвергает zero-tenant/author и неизвестный storage.type. Создан `config_test.go` с 9 тестами включая table-driven env-override.
- [x] **T-4** Обновить fsrepo — GREEN (делегировано subagent'у). FS-репозиторий теперь работает с подкаталогами по tenant_short_id, использует `core.TenantFromContext` для scope, реализует sort/limit/offset, truncation `PublishHistory` до 10 (CP-5). `frontmatter_parser` валидирует 9 обязательных полей (CP-7). Добавлен метод `Attachment.AbsolutePath` с защитой от path traversal.
- [x] **T-5** Обновить httpapi — GREEN (делегировано subagent'у). Добавлен `TenantFromConfigMiddleware`. `jsonPost` расширен всеми новыми полями. Tenant immutability enforcement: 403 на mismatch, 400 на попытку смены tenant_id через PATCH (CP-2, CP-14). Backward-compat `NewServer(service, publisher)` сохранён.
- [x] **T-6** Обновить CLI — GREEN (делегировано subagent'у вместе с T-7). Удалён `getService` (REQ-9.3). `init` стал интерактивным с `--force`, swappable `uuidGenerator` для тестируемости (CP-12, CP-13). `new` принимает `--tenant`/`--author`. `list --format json` (CP-15). Все `next`/`stats`/`plan`/`show`/`edit`/`delete`/`status`/`publish`/`import`/`migrate`/`migrate_ids` адаптированы под новые сигнатуры сервиса с `core.WithTenant(ctx, ...)`.
- [x] **T-7** testdata + .jtpost.example.yaml + sqlite — GREEN. 5 файлов в `testdata/posts/` получили обязательные поля frontmatter. `.jtpost.example.yaml` переписан со всеми новыми секциями. SQLite схема расширена колонками `tenant_id`, `author_id`, `revision`, `idx_posts_tenant_id` index. Полная имплементация SQLite — F2.
- [x] **T-8** GATE — финальная верификация — GREEN.
  - `task fmt`: clean
  - `task vet`: clean
  - `task lint`: 0 issues
  - `task test`: 9 пакетов ok
  - `task test:race`: 9 пакетов ok
  - `task build`: успех (`./dist/jtpost`)
  - `task test:coverage`: общее покрытие — см. ниже
  - CHANGELOG.md обновлён с разделом «F1: Foundation» (breaking changes, новый функционал, migration path).

## Final Verification

### Tests

```
ok  	github.com/jtprogru/jtpost/internal/adapters/config	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/fsrepo	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/httpapi	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/sqlite	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegram	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegramconv	(cached)
ok  	github.com/jtprogru/jtpost/internal/cli	(cached)
ok  	github.com/jtprogru/jtpost/internal/core	(cached)
ok  	github.com/jtprogru/jtpost/internal/logger	(cached)
```

### Race tests

```
ok  	github.com/jtprogru/jtpost/internal/adapters/config	1.846s
ok  	github.com/jtprogru/jtpost/internal/adapters/fsrepo	1.497s
ok  	github.com/jtprogru/jtpost/internal/adapters/httpapi	2.087s
ok  	github.com/jtprogru/jtpost/internal/adapters/sqlite	2.518s
ok  	github.com/jtprogru/jtpost/internal/adapters/telegram	2.813s
ok  	github.com/jtprogru/jtpost/internal/adapters/telegramconv	(cached)
ok  	github.com/jtprogru/jtpost/internal/cli	3.409s
ok  	github.com/jtprogru/jtpost/internal/core	2.853s
ok  	github.com/jtprogru/jtpost/internal/logger	(cached)
```

### Build

```
task: [tidy] go mod tidy
task: [build] go mod download
task: [build] CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go
```

### Lint

```
0 issues.
```

### Coverage

```
ok  	github.com/jtprogru/jtpost/internal/adapters/config	coverage: 90.9% of statements
ok  	github.com/jtprogru/jtpost/internal/adapters/fsrepo	coverage: 57.5% of statements
ok  	github.com/jtprogru/jtpost/internal/adapters/httpapi	coverage: 69.1% of statements
ok  	github.com/jtprogru/jtpost/internal/adapters/sqlite	coverage: 73.2% of statements
ok  	github.com/jtprogru/jtpost/internal/adapters/telegram	coverage: 14.8% of statements
ok  	github.com/jtprogru/jtpost/internal/adapters/telegramconv	coverage: 100.0% of statements
ok  	github.com/jtprogru/jtpost/internal/cli	coverage: 38.2% of statements
ok  	github.com/jtprogru/jtpost/internal/core	coverage: 57.3% of statements
ok  	github.com/jtprogru/jtpost/internal/logger	coverage: 94.6% of statements
```

Замечание: общее покрытие пока ниже целевых 85% (см. F10 в roadmap), но F1 не вводит coverage gate (REQ Q8=a, отложено в F10). Целевое покрытие будет достигнуто в фазе F10 «Quality: tests & coverage» через интеграционные тесты HTTP API, mock-сервер Telegram, e2e CLI-флоу и fuzzing frontmatter-парсера.

## Files Changed

51 файл, +3343 / −1785 строк. Полный список (по областям):

**Domain core (8 файлов):**
- `internal/core/post.go` (+ типы `Attachment`, `PublishAttempt`, метод `TenantShortID`, расширение `Post`/`PostFilter`)
- `internal/core/core.go` (новые статусы, `allowedTransitions`, `IsTransitionAllowed`, `AllStatuses`, удалена `StatusOrder`)
- `internal/core/errors.go` (новые ошибки, `IsStatusTransitionValid` deprecated)
- `internal/core/service.go` (расширен контракт, новые методы `Archive`, `MarkFailed`, `AppendPublishAttempt`)
- `internal/core/scope.go` (NEW — context-keys)
- `internal/core/post_test.go` (NEW — round-trip, attachment validation, sort key whitelist)
- `internal/core/core_test.go` (полный декартов перебор статусов)
- `internal/core/service_test.go` (полная переписка под новый контракт)

**Configuration (2 файла):**
- `internal/adapters/config/config.go` (4 новые подсекции, env-bindings, validate)
- `internal/adapters/config/config_test.go` (NEW — defaults, env-override, validation)

**Storage (5 файлов):**
- `internal/adapters/fsrepo/repository.go` (tenant subdirs, sort/limit/offset, scope enforcement)
- `internal/adapters/fsrepo/frontmatter_parser.go` (новые поля, truncation, валидация)
- `internal/adapters/fsrepo/repository_test.go`, `frontmatter_parser_test.go`, `test_helpers_test.go` (полная переписка)
- `internal/adapters/sqlite/repository.go` (схема: `tenant_id`, `author_id`, `revision`, индекс)

**HTTP API (4 файла):**
- `internal/adapters/httpapi/middleware.go` (`TenantFromConfigMiddleware`)
- `internal/adapters/httpapi/server.go` (`jsonPost` расширен, tenant enforcement)
- `internal/adapters/httpapi/server_test.go`, `middleware_test.go` (новые тесты)

**CLI (16 файлов):**
- `internal/cli/root.go` (удалён `getService`)
- `internal/cli/init.go` (`--force`, swappable uuidGenerator, prompt)
- `internal/cli/new.go` (`--tenant`, `--author`)
- `internal/cli/list.go` (JSON формат)
- `internal/cli/{next,stats,plan,show,edit,delete,status,publish,import,migrate,migrate_ids}.go` (адаптация под scope-контекст)
- `internal/cli/{init,new,list,delete,plan,next,stats,migrate_ids}_test.go`, `test_helpers_test.go` (новые/обновлённые тесты)

**Other:**
- `internal/adapters/telegram/publisher.go` (минимальная адаптация для компиляции)
- `internal/logger/logger_test.go` (мелкие правки)
- `testdata/posts/*.md` × 5 (новые обязательные поля)
- `.jtpost.example.yaml` (полная переписка)
- `CHANGELOG.md` (раздел F1)
- `go.mod` (`mapstructure/v2` повышен до прямой зависимости)

## Notes

**Pre-existing issues найдены при работе:**
1. Ни один не обнаружен.

**Deviations from plan:**
1. T-3 — `config_test.go` уже существовал; subagent дополнил его, а не пересоздал.
2. T-3 — `time.Duration` и `uuid.UUID` парсятся через единый `mapstructure.ComposeDecodeHookFunc`. `mapstructure/v2` стал прямой зависимостью.
3. T-4 — `Attachment.AbsolutePath` добавлен в `internal/core/post.go` (а не в fsrepo) — для переиспользования из других слоёв.
4. T-4 — Legacy frontmatter-helpers (`ParseFrontmatter`, `BuildFrontmatter`, `NormalizeFrontmatter`) сохранены для совместимости.
5. T-5 — `NewServer(service, publisher)` оставлен backward-compatible (cfg может быть nil), что позволило не менять `cmd/jtpost/main.go` и `cli/serve.go`.
6. T-5 — `createPost` дополнительно поддерживает поля `content`/`deadline` через follow-up `UpdatePost` для round-trip совместимости с тестами.
7. T-7 — Минимальные изменения SQLite-схемы (только новые колонки + индекс). Полная функциональность с sqlc/goose — F2.
8. T-7 — `internal/cli/import.go` использует package-level `importCfg` для инъекции tenant/author в импортированные посты (минимальное изменение, без concurrency-проблем).
9. После прохождения тестов прогон `task lint` обнаружил 31 стилистический issue (intrange, gochecknoglobals на test-vars, funcorder, dupword, и т.д.) — все исправлены отдельным subagent-проходом, итог: `0 issues`.

**Известные ограничения:**
1. `Revision` инкрементируется в сервисе — concurrent updates могут race'ить. Optimistic locking — F2 (Postgres `UPDATE ... WHERE revision = ?`).
2. Coverage в `internal/core` (57.3%) и `internal/adapters/fsrepo` (57.5%) ниже целевых 85% — F10 закроет.
3. SQLite-репозиторий полностью функционирует, но не использует новые колонки `tenant_id`/`author_id` в queries — F2 переведёт на sqlc и реальные multi-tenant запросы.
