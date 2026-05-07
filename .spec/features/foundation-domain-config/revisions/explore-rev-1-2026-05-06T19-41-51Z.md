# Exploration: Foundation — Domain Model & Configuration (F1)

## Intent

Это первая (фундаментальная) фича большой программы доведения jtpost до финала. Без неё не сделать ни одну из последующих фич (F2–F12).

**Цели F1:**

1. Расширить доменную модель `Post` под нужды будущих фич: multi-tenant, авто-публикация (worker), git-история, медиа в Telegram, история публикаций с retry.
2. Расширить жизненный цикл статусов: добавить `archived` и `failed`, разрешить контролируемые откаты.
3. Расширить схему конфигурации под новые секции (`storage.*`, `auth.*`, `worker.*`, `server.*`) с сохранением совместимости с текущей загрузкой через viper + env.
4. Заложить «default tenant» — UUIDv7, генерируется однократно при `jtpost init`, пишется в `auth.tenant_default`.
5. Закрыть мелкие технические долги, тесно связанные с моделью: TODO в `internal/cli/list.go:66` (JSON-вывод) и legacy `getService()` в `internal/cli/root.go:74`.

**Чего F1 не делает (выносится в последующие фичи):**

- Не реализует Postgres-адаптер (F2), git-хранилище (F3), auth-middleware и реальный OAuth (F4), worker-процесс (F6), Telegram-медиа (F7), Web UI (F8).
- Только готовит **доменный фундамент и схему** для них: новые поля в `Post`, новые секции конфига, новые статусы, новые ошибки.

**Триггер:** запрос пользователя «довести приложение до финала», проведено двухблочное интервью (см. историю чата). Greenfield-расширение существующего кода (brownfield) — старых данных в продакшене нет, поэтому новые поля делаем **обязательными**, миграция данных не требуется (M1).

---

## Investigation

Проведён полный аудит проекта (см. отчёт в чате выше). Ключевые точки, релевантные F1:

### Текущая доменная модель (`internal/core/`)

**`internal/core/post.go`:**
- `Post` имеет 10 полей: `ID`, `Title`, `Slug`, `Status`, `Tags`, `Deadline`, `ScheduledAt`, `PublishedAt`, `Content`, `External{TelegramURL}`.
- `PostID = uuid.UUID`, генерация через `uuid.NewV7()` (`GeneratePostID`).
- `PostFilter`: `Statuses []PostStatus`, `Tags []string`, `Search string`. Без `TenantID`, `AuthorID`, без сортировки и пагинации.
- `ExternalLinks` содержит только `TelegramURL`.

**`internal/core/core.go`:**
- 5 статусов в `StatusOrder` массиве: `idea, draft, ready, scheduled, published`.

**`internal/core/errors.go`:**
- `IsStatusTransitionValid(from, to)` разрешает только переходы вперёд по индексу в `StatusOrder` (`toIdx > fromIdx`). Откаты невозможны. Конечный статус `published` не имеет «выхода» (нельзя архивировать).
- 11 доменных ошибок без `ErrPublishRetryExhausted`, `ErrTenantMismatch`, `ErrInvalidTransition` (отдельной от `ErrInvalidStatus`).

**`internal/core/repository.go`:** интерфейсы `PostRepository`, `TransactionalRepository`, `MigratableRepository`. Контекстный аргумент `context.Context` присутствует — это позволит позже добавить `tenant_id` через context value (см. Options).

**`internal/core/publisher.go`, `clock.go`, `service.go`:** `Publisher.Publish(ctx, *Post) (*Post, error)`, `Clock.Now()`, `PostService` — координирующая логика поверх repository + publisher.

### Текущая схема конфигурации (`internal/adapters/config/config.go`)

```go
type Config struct {
    PostsDir, TemplatesDir string
    Telegram TelegramConfig    // bot_token, chat_id
    SQLite   SQLiteConfig      // dsn
    Defaults DefaultConfig     // status, platforms[], deadline
}
```

- Загрузка через `viper`, env-префикс `JTPOST_`, ключи через `_` (например `JTPOST_TELEGRAM_BOT_TOKEN`).
- `BindEnv` явно вызывается для каждого вложенного ключа (limitation viper'а).
- `Save()` пишет YAML с правами `0o644`.
- `Validate()` проверяет только `PostsDir != ""`.
- Поле `Defaults.Platforms []string` — артефакт удалённой Platform-сущности (Этап 7.3 ROADMAP), сейчас не используется по назначению, но всё ещё в конфиге.

### Места, где появятся изменения

- `internal/core/post.go` — расширение `Post`, `PostFilter`, `ExternalLinks` (или новый `PublishHistory`).
- `internal/core/core.go` — добавление статусов `archived`, `failed`; пересмотр `StatusOrder` или замена на explicit transitions table.
- `internal/core/errors.go` — `IsStatusTransitionValid` переписать под explicit table; новые ошибки.
- `internal/core/service.go` — `CreatePostInput` расширить (TenantID, AuthorID, Excerpt и т.д.); новые методы (`Archive`, `MarkFailed`, `Rollback`).
- `internal/adapters/config/config.go` — новые подструктуры (`StorageConfig`, `AuthConfig`, `WorkerConfig`, `ServerConfig`), новые `BindEnv`, новые `SetDefault`.
- `internal/adapters/fsrepo/repository.go` и `frontmatter_parser.go` — сериализация новых полей в frontmatter.
- `internal/adapters/sqlite/` — расширение схемы (но полная реализация вынесена в F2).
- `internal/adapters/httpapi/server.go` — `jsonPost` структура + хендлеры PATCH/POST/GET для новых полей.
- `internal/cli/*.go` — обновление `new`, `list --format json` (TODO), `show`, удаление legacy `getService()` в `root.go:74`.
- `cmd/jtpost/main.go` — без изменений.

### Тесты

Существующих тестов: 20 файлов. Релевантные для F1:
- `internal/core/post_test.go`, `core_test.go`, `slug_test.go`
- `internal/adapters/fsrepo/repository_test.go`, `frontmatter_parser_test.go`
- `internal/adapters/httpapi/server_test.go`
- `internal/adapters/config/...` — нет, тесты конфига отсутствуют (gap для F1).
- `internal/cli/list_test.go`, `new_test.go`, `show_test.go`

Стиль: table-driven tests, native `testing` пакет, `testdata/` для фикстур.

### Влияние на CHANGELOG / ROADMAP

ROADMAP.md помечен пользователем как неактуальный — будет полностью переписан на финальном этапе F11. В F1 — только пометить «v0.4.0 in progress / foundation refactor».

---

## Build Tooling

- **Orchestrator:** [Task](https://taskfile.dev) (`Taskfile.yml` в корне).
- **Test:** `task test` (= `go test -v -coverprofile=cover.out ./...`).
- **Race tests:** `task test:race`.
- **Coverage report:** `task test:coverage` (генерирует `cover.html`).
- **Build:** `task build` (= `CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go`).
- **Lint:** `task lint` (= `golangci-lint run -v`).
- **Generate:** не используется в проекте (нет proto/mocks/sqlc на текущем этапе; `sqlc` появится в F2).
- **Format:** `task fmt`, `task vet`.
- **Source:** `Taskfile.yml`.

CI: GitHub Actions (`.github/workflows/ci.yml`) — Go 1.25/1.26, Ubuntu/macOS, golangci-lint, gosec, codecov.

---

## Options Considered

### Option A: «Big bang» — изменить `Post` атомарно одним коммитом

Расширить `Post`, `PostFilter`, `Config`, `IsStatusTransitionValid` в одном PR. Все тесты обновляются разом, фикстуры в `testdata/` пересоздаются.

- **Pros:** простой ментальный baseline для F2–F12. Один общий «before/after».
- **Cons:** большой PR (~30+ файлов), сложнее ревью, риск зацепить смежные адаптеры (httpapi, fsrepo) до их собственного рефакторинга.
- **Сложность:** Medium-High.

### Option B: Инкрементально по слоям (core → adapters → cli)

Сначала добавить новые поля в `core.Post` (с дефолтами/обязательностью), пройти `go build` и тесты. Потом обновить fsrepo (сериализация frontmatter). Потом httpapi (jsonPost). Потом cli. Каждый слой — отдельный коммит внутри одной фичи.

- **Pros:** легче отлаживать (видно, на каком слое сломалось), мельче коммиты, удобнее ревью.
- **Cons:** между коммитами может быть «всё компилируется, но не работает по полному пути» — придётся доводить до конца F1, чтобы запустить интеграционные тесты.
- **Сложность:** Medium.

### Option C: Tenant как context value, без поля в `Post`

`tenant_id` не хранится в `Post`, передаётся через `context.Context` (`ctx = ctx.WithValue("tenant_id", uuid)`), и репозитории фильтруют по нему. На диске/в БД физический `tenant_id` колонкой/полем frontmatter всё равно нужен.

- **Pros:** доменная модель остаётся «чистой».
- **Cons:** двойная книжка (поле в БД + контекст в коде), легко забыть передать контекст, сложнее тестировать. Стандартный антипаттерн «business data in context».
- **Сложность:** Medium, но риск багов высокий.

### Option D: Использовать generic «metadata map» вместо явных полей

Вместо `Excerpt`, `CoverImage`, `Attachments[]`, `PublishHistory[]` — одно поле `Metadata map[string]any` или `Extras json.RawMessage` в `Post`.

- **Pros:** flexible, легко расширять без миграций.
- **Cons:** теряем типобезопасность, сложно фильтровать/индексировать в SQL, frontmatter становится «мусорным». Противоречит хорошо отделённой доменной модели проекта.
- **Сложность:** Low (на словах), но High в эксплуатации.

---

## Constraints & Risks

### Breaking changes

- `Post` получит **обязательные** новые поля (`TenantID`, `AuthorID`, `CreatedAt`, `UpdatedAt`). По решению пользователя (M1) — «старых» постов нет, миграция данных не нужна. Однако `testdata/posts/*.md` должны быть пересозданы с новыми полями, иначе fsrepo-тесты упадут.
- `IsStatusTransitionValid` поменяет семантику (откаты + новые статусы). Тесты в `core_test.go` нужно обновить.
- Конфиг `Config` получит новые секции. Существующие `.jtpost.yaml` будут продолжать работать (новые поля = дефолты), но `jtpost init` будет писать новый, расширенный шаблон.
- `Defaults.Platforms []string` — поле-артефакт. Удалить целиком (продолжение Этапа 7.3) или оставить?

### Совместимость API

- `jsonPost` в httpapi получит новые поля. Существующие потребители API (Web UI) увидят их — нужно будет обновить шаблон. Web UI на htmx — обновление минимальное (поля не отображаем сразу, всё в F8).

### Производительность

- Новые поля сами по себе не влияют. Однако `PublishHistory []PublishAttempt` — потенциально длинный массив. На FS-репозитории это значит длинный YAML frontmatter. Решение: ограничить хранение последних N записей в frontmatter (например 10), полная история в SQLite/Postgres-режимах.

### Безопасность

- `auth.secret` (для JWT, F4) и `telegram.bot_token` — секреты. Виper уже умеет читать их из env. В F1 только добавляем поля схемы, не реализуем сам auth.

### Зависимости

- F1 не добавляет новых внешних библиотек. `pgx`, `sqlc`, `goose`, `go-git`, OAuth-библиотеки — в последующих фичах.

### Edge cases

- **PostID и TenantID коллизия в FS-режиме:** если два тенанта создадут пост с одинаковым `slug`, файлы конфликтуют. В F1: либо ввести подкаталоги по `tenant_id` в FS, либо ограничиться single-tenant в FS-режиме (multi-tenant только в SQLite/Postgres). Рекомендация: подкаталоги, чтобы паттерн был единым везде.
- **Откаты статусов:** разрешать только определённые. Например `published → archived` ОК, но `published → draft` — потеря данных (`PublishedAt`, `External.TelegramURL`). Решение: явная таблица допустимых переходов (Map<from, []to>), а не вычисление по индексу.
- **`failed` статус:** куда из него можно перейти? Минимум — обратно в `ready` (для повторной попытки). Worker автоматически переводит из `scheduled` в `failed` после исчерпания retry.
- **Generic `JTPOST_` env-переменные с массивами:** `JTPOST_DEFAULTS_PLATFORMS` — массив. Viper парсит через запятую. Новые массивы (`storage.git.remotes`?) должны следовать этой же конвенции.

---

## Recommended Direction

**Option B (инкрементально по слоям) + Option A (`tenant_id` как явное поле в `Post`).**

Внутри F1 идём так:

1. `core` (домен + статусы + ошибки) — расширяем модель, добавляем явный transition-table, новые ошибки.
2. `config` — новая схема со всеми секциями (под F2–F8), `jtpost init` пишет полный пример с TODO-комментариями для будущих секций.
3. `fsrepo` + `frontmatter_parser` — сериализация новых полей; подкаталоги по `tenant_id` (`content/posts/<tenant_id>/<slug>.md`).
4. `httpapi` — `jsonPost` обновлён, хендлеры принимают/возвращают новые поля; CRUD по `tenant_id` через middleware-заглушку (читает из конфига `auth.tenant_default`, реальный auth — F4).
5. `cli` — `jtpost new` принимает `--tenant`/`--author`; `jtpost list --format json` (закрытие TODO); `getService()` legacy выпиливается; `jtpost init` генерирует UUIDv7 для `auth.tenant_default`.
6. Обновление `testdata/posts/*.md` под новые обязательные поля.
7. Тесты конфига (`internal/adapters/config/config_test.go`) — впервые появляются.

Эта последовательность даёт rolling-зелёные тесты и в любом момент `task test` отрабатывает чисто.

---

## Scope Boundaries

### Must-have (v1, эта фича)

- **Доменная модель Post:** добавить обязательные поля `TenantID uuid.UUID`, `AuthorID uuid.UUID`, `CreatedAt time.Time`, `UpdatedAt time.Time`.
- **Доменная модель Post:** добавить опциональные `*string Excerpt`, `*string CoverImage`, `[]Attachment Attachments`, `[]PublishAttempt PublishHistory`, `int Revision`.
- **Новые типы:** `Attachment{Type, URL, Caption}`, `PublishAttempt{At, Target, Status, MessageID, Error}`.
- **Статусы:** добавить `StatusArchived`, `StatusFailed`. Заменить `IsStatusTransitionValid` на explicit `AllowedTransitions map[PostStatus][]PostStatus`.
- **Допустимые переходы (минимум):** `idea→draft`, `draft→ready`, `ready→scheduled`, `ready→published`, `scheduled→published`, `scheduled→failed`, `failed→ready`, `published→archived`, `scheduled→ready` (откат).
- **Новые ошибки:** `ErrInvalidTransition`, `ErrTenantMismatch`, `ErrPublishRetryExhausted`.
- **PostFilter:** добавить `TenantID uuid.UUID` (обязательное), `AuthorID *uuid.UUID`, `SortBy string`, `SortOrder string`, `Limit int`, `Offset int`.
- **Конфигурация:** новые секции `Storage{Type, Git{Enabled, AutoCommit, Remote, AutoPush}, SQLite{DSN}, Postgres{DSN, MaxOpenConns, MaxIdleConns}}`, `Auth{Type, Secret, TenantDefault, OAuth{...}}`, `Worker{Enabled, Interval}`, `Server{Addr, Port, BaseURL}`.
- **`jtpost init`:** создаёт `.jtpost.yaml` с дефолтами всех секций; генерирует UUIDv7 для `auth.tenant_default` (M2).
- **fsrepo:** сериализация новых полей в YAML frontmatter; подкаталоги `content/posts/<tenant_id>/<slug>.md`.
- **httpapi:** `jsonPost` отражает новую модель; хендлеры PATCH/POST принимают новые поля; временный middleware `tenantFromConfig` (без auth) — извлекает `tenant_default` из конфига и кладёт в context; readiness для F4.
- **CLI чистка:** реализовать `jtpost list --format json` (закрытие TODO в `internal/cli/list.go:66`); удалить legacy `getService()` в `internal/cli/root.go:74`.
- **Удаление artefact-поля `Defaults.Platforms`** из `Config` (продолжение Этапа 7.3 ROADMAP).
- **Тесты:** обновить существующие; новый `config_test.go`; обновить `testdata/posts/`.

### Deferred (последующие фичи)

- **F2:** реальная имплементация `storage.type=sqlite|postgres` (сейчас только схема в конфиге).
- **F3:** `storage.git.*` логика (auto-commit, push) — go-git.
- **F4:** реальный auth-middleware вместо `tenantFromConfig`-заглушки; OAuth flow; API-токены; `auth login`.
- **F6:** worker процесс — пишет в `PublishHistory`, делает retry/backoff.
- **F7:** Telegram media — заполняет `Attachments`.
- **F8:** Web UI отображает новые поля (Excerpt, CoverImage и т.д.).

### Needs spike

- **Подкаталоги `content/posts/<tenant_id>/`:** нужна ли поддержка миграции из «плоской» структуры на иерархическую? Пользователь подтвердил M1 (старых данных нет) — спайк не нужен, делаем сразу иерархию.
- **Размер `PublishHistory` в frontmatter:** какой лимит? Нужно решить в фазе Design (предложение: 10 последних записей в FS, полная история — в SQLite/PG в F2).

---

## Assumptions & Open Questions

### Assumptions

- `[ASSUMPTION: «старых» постов в продакшене не существует]` — подтверждено пользователем (M1). Поэтому новые поля делаем обязательными без `omitempty` для критичных, миграцию данных не пишем.
- `[ASSUMPTION: дефолтный tenant генерируется при jtpost init однократно через uuid.NewV7() и сохраняется в auth.tenant_default]` — подтверждено пользователем (M2).
- `[ASSUMPTION: multi-tenant в FS реализуется подкаталогами content/posts/<tenant_id>/<slug>.md]` — снимает коллизию slug между тенантами; единый паттерн с SQLite/Postgres.
- `[ASSUMPTION: PublishHistory в FS-frontmatter ограничивается последними N=10 записями]` — финальное решение в фазе Design.
- `[ASSUMPTION: схема конфига расширяется обратно совместимо]` — старый `.jtpost.yaml` без новых секций продолжает работать через viper-defaults.
- `[ASSUMPTION: поле Defaults.Platforms удаляется как мёртвый код]` — артефакт удалённой Platform-сущности.
- `[ASSUMPTION: middleware tenantFromConfig в F1 — заглушка]` — он только читает `auth.tenant_default` из конфига и кладёт в context; реальная аутентификация — F4.
- `[ASSUMPTION: AuthorID в single-user сценарии = TenantID]` — у F1 нет реальных пользователей; AuthorID можно делать равным TenantID до F4.
- `[ASSUMPTION: Attachment имеет минимальный набор полей: Type (photo|video|document), URL (или path), Caption]` — финализируется в Design.

### Open Questions (для фазы Requirements)

1. **Лимит `PublishHistory` в FS-frontmatter:** 10? 20? Или хранить отдельным файлом `<slug>.history.yaml` рядом с `<slug>.md`?
2. **`Revision int` или `Revision string` (sha):** использовать как монотонно растущий счётчик (просто) или как SHA-1 содержимого/git-commit (точнее, но дороже)?
3. **`Attachment.URL` vs `Attachment.Path`:** хранить URL (после загрузки на CDN/Telegram) или относительный путь к локальному файлу (до загрузки)? Возможно оба поля.
4. **CoverImage:** отдельное поле или просто `Attachments[0]` с `Type=cover`? Рекомендация: отдельное поле для явности.
5. **`jtpost init` и существующий `.jtpost.yaml`:** перезаписывать с подтверждением, мержить, или ошибиться? Сейчас перезаписывает молча.
6. **Имена секций конфига:** `storage.type` vs `storage.driver` vs `storage.kind`? Принятый дефолт по предложению — `type`.
7. **Удаление подкаталогов при смене tenant_id у поста:** возможен ли в принципе такой сценарий? Если да — операция «move», если нет — запрещаем (пост принадлежит тенанту навсегда).
8. **Обновление `cover.out` и coverage gates:** вводить ли в F1 нижнюю планку покрытия (например 80%)? Или это в F10.

---

## Done When (для approve)

Все пункты Quality Checklist:

- [x] Codebase прочитан (cited: `internal/core/post.go`, `core.go`, `errors.go`, `internal/adapters/config/config.go`, `Taskfile.yml`, `.jtpost.example.yaml`).
- [x] Сравнены 4 опции (A/B/C/D), для каждой — pros/cons/complexity.
- [x] Trade-off'ы явные.
- [x] Scope boundaries: Must-have / Deferred / Needs spike — категоризированы.
- [x] Assumptions помечены тэгом `[ASSUMPTION: ...]`.
- [x] Open Questions есть.
- [x] Build Tooling зафиксирован (Task, команды).
- [ ] Артефакт зарегистрирован через `pipeline.sh artifact` — следующий шаг после approve.
