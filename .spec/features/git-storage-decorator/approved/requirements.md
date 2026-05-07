# Git-storage Decorator (F3) — Requirements

**Status:** Draft
**Author:** Claude (Opus 4.7) + Mikhail Savin
**Date:** 2026-05-07
**Feature:** git-storage-decorator (F3)
**Branch:** `feature/git-storage-decorator`

## Overview

F3 даёт реальную имплементацию для секции `storage.git.*`, которая в F1 описана только в схеме конфига. Новый пакет `internal/adapters/gitrepo` реализует Decorator-паттерн поверх `core.PostRepository` (фактически — fs-репозитория). После успешного Create/Update/Delete декоратор выполняет `git add` + `git commit` (опц. `git push`) через pure-Go `github.com/go-git/go-git/v5`. Storage factory автоматически оборачивает fs-репо при `cfg.Storage.Type=fs && cfg.Storage.Git.Enabled=true`. Auto-init `git init` при первом `Open()` если posts_dir не git-репо. Doctor расширен git-статусом.

## Glossary

| Term | Definition | Code Artifact |
|------|------------|---------------|
| `GitDecorator` | Обёртка над `core.PostRepository`, выполняющая git-commit после успешных мутаций | `internal/adapters/gitrepo/decorator.go` |
| `CommitTemplate` | Парсенный `text/template.Template` с переменными `.Slug`, `.Title`, `.ID`, `.Status`, `.Operation`, `.Time` | `internal/adapters/gitrepo/template.go` |
| `Operation` | Строка `create` \| `update` \| `delete`, передаётся в commit-template | передаётся в `commitChanges` |
| `gitAuthor` | Идентичность для git-коммитов (env `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL` → fallback `jtpost <bot@jtpost.local>`) | `internal/adapters/gitrepo/author.go` |
| `pushTimeout` | Hardcoded 30s timeout для `git push` операций | `internal/adapters/gitrepo/decorator.go` |
| `gitMutex` | `sync.Mutex` сериализующий git-операции в рамках одного процесса | `internal/adapters/gitrepo/decorator.go` |

## User Stories

- Как **владелец канала**, я хочу включить `storage.git.enabled=true` в конфиге и получать автоматическую git-историю всех изменений постов, чтобы видеть кто/когда что менял.
- Как **владелец канала**, я хочу опциональный `auto_push=true` для бэкапа на удалённый remote (GitHub/GitLab/self-hosted), чтобы не терять данные при поломке диска.
- Как **владелец канала**, я хочу настраиваемый `commit_template`, чтобы коммит-сообщения были осмысленными для моего workflow.
- Как **разработчик-сопровождающий**, я хочу, чтобы git-decorator не блокировал основную операцию: failed push не должен «откатывать» успешно сохранённый пост.
- Как **разработчик**, я хочу запустить `jtpost doctor` и видеть состояние git-репо (clean/dirty/ahead/behind), чтобы быстро диагностировать проблемы с публикацией.

## Requirements

### Group 1 — Decorator контракт

**REQ-1.1** WHEN пакет `internal/adapters/gitrepo` экспортирует функцию `NewGitDecorator(inner core.PostRepository, postsDir string, cfg config.GitStorageConfig) (*GitDecorator, error)`, the system SHALL открыть git-репо в `postsDir` или создать его (`git init`) если он не git-репо.

**REQ-1.2** WHEN `NewGitDecorator` вызывается с невалидным `cfg.CommitTemplate` (не парсится `text/template`), the system SHALL вернуть `errors.Join(core.ErrConfigInvalid, <template-error>)` и не открывать репо.

**REQ-1.3** WHEN `GitDecorator.GetByID/GetBySlug/List/Count` вызывается, the system SHALL делегировать вызов `inner` репозиторию без обращения к git.

**REQ-1.4** WHEN `GitDecorator.Create/Update/Delete` вызывается и `inner` возвращает ошибку, the system SHALL пробросить ошибку без обращения к git (никакого коммита для failed-операции).

**REQ-1.5** WHEN `GitDecorator.Create/Update/Delete` вызывается и `inner` успешно завершается, the system SHALL под `gitMutex` выполнить `wt.Add(<relative_path>)` (для Delete — `wt.Remove`) и `wt.Commit(message, author)` где message получен из `CommitTemplate.Execute`.

**REQ-1.6** WHEN `GitDecorator` запускается и `inner` реализует `core.MigratableRepository`, the system SHALL прокидывать `ImportPosts` и `Count` в inner; после `ImportPosts` SHALL делать ОДИН батч-commit для всей группы постов.

**REQ-1.7** WHEN `GitDecorator` запускается и `inner` реализует `core.TransactionalRepository`, the system SHALL предоставлять `BeginTx` proxy в inner; git-commit делается ПОСЛЕ commit транзакции.

### Group 2 — Auto-init и репо

**REQ-2.1** WHEN `NewGitDecorator` обнаруживает что `postsDir` не существует, the system SHALL создать директорию (mode 0o755) перед `git init`.

**REQ-2.2** WHEN `NewGitDecorator` обнаруживает что `postsDir` существует но не содержит `.git/`, the system SHALL выполнить `git.PlainInit(postsDir, false)` с initial-branch согласно `cfg.Branch` (default `main`).

**REQ-2.3** WHEN `NewGitDecorator` обнаруживает что `postsDir/.git/` уже существует, the system SHALL открыть существующий репо через `git.PlainOpen(postsDir)` и НЕ переключать ветку.

**REQ-2.4** WHEN `NewGitDecorator` обнаруживает что HEAD находится в detached state, the system SHALL логировать warning «git HEAD detached, auto-commit disabled» и продолжать работу без авто-коммитов; mutate-операции возвращают success.

**REQ-2.5** WHEN `NewGitDecorator` обнаруживает stale lockfile `.git/index.lock` старше 60 секунд, the system SHALL удалить его перед открытием репо (защита от crashed previous process).

### Group 3 — Commit-template

**REQ-3.1** WHEN `CommitTemplate.Execute` вызывается с операцией над постом `p`, the system SHALL предоставить переменные `Slug` (`p.Slug`), `Title` (`p.Title`), `ID` (`p.ID.String()`), `Status` (string from `p.Status`), `Operation` (`create|update|delete`), `Time` (`time.Time` ISO 8601 в UTC).

**REQ-3.2** WHEN `cfg.CommitTemplate == ""`, the system SHALL использовать default template `"chore: {{.Operation}} post {{.Slug}}"`.

**REQ-3.3** WHEN `CommitTemplate.Execute` падает в runtime (unlikely при validated template), the system SHALL fall-back на static message `"chore: <operation> post <slug>"` и логировать warning.

### Group 4 — Auto-push

**REQ-4.1** WHEN `cfg.AutoPush == true` и `cfg.Remote == ""`, the system SHALL возвращать `errors.Join(core.ErrConfigInvalid, errors.New("storage.git.auto_push=true requires storage.git.remote"))` из `Config.Validate()`.

**REQ-4.2** WHEN `cfg.AutoPush == true && cfg.Remote != ""`, the system SHALL после каждого commit выполнить `repo.PushContext(ctx, opts)` с `pushTimeout = 30s`.

**REQ-4.3** WHEN `git push` падает (network/auth/conflict), the system SHALL логировать warning с masked URL (без auth-credentials) и НЕ возвращать ошибку из мутирующего метода (Create/Update/Delete возвращают success — пост на диске).

**REQ-4.4** WHEN `cfg.Remote` указывает на URL, отличный от настроенного `git remote origin`, the system SHALL логировать error «remote URL mismatch» и пропустить push (НЕ переписывать `git remote`).

**REQ-4.5** WHEN среда содержит `GIT_HTTPS_TOKEN` env-переменную и push идёт по HTTPS-URL, the system SHALL передать её как basic-auth (username=`token`, password=`<env>`); иначе — пытаться через ssh-agent для SSH-URL.

### Group 5 — Concurrency

**REQ-5.1** WHEN несколько goroutines одного процесса вызывают мутации `GitDecorator`, the system SHALL сериализовать git-операции через `gitMutex` (read/list — параллельны без блокировки).

**REQ-5.2** WHEN inner операция выполнена (под mutex), the system SHALL держать mutex до завершения git-commit И push (если AutoPush). Это серилазет сетевые операции — компромисс ради простоты в F3.

### Group 6 — Storage factory

**REQ-6.1** WHEN `storage.OpenAs(cfg, "fs")` вызывается и `cfg.Storage.Git.Enabled == true`, the system SHALL обернуть результат `fsrepo.NewFileSystemRepository(...)` в `gitrepo.NewGitDecorator(...)` и вернуть как `core.PostRepository`.

**REQ-6.2** WHEN `storage.OpenAs(cfg, "fs")` вызывается и `cfg.Storage.Git.Enabled == false`, the system SHALL возвращать чистый fs-репо без декоратора (текущее поведение F2).

**REQ-6.3** WHEN `storage.OpenAs(cfg, "sqlite"|"postgres")` вызывается, the system SHALL игнорировать `cfg.Storage.Git.*` (git decorator не применяется к SQL-backend).

### Group 7 — Doctor extension

**REQ-7.1** WHEN `jtpost doctor` запускается с `cfg.Storage.Type == "fs"` и `cfg.Storage.Git.Enabled == true`, the system SHALL добавить отдельный check `Git`: открыть репо, выполнить `wt.Status()`, вывести результат clean/dirty + список изменённых файлов (top 5).

**REQ-7.2** WHEN `jtpost doctor` запускается с настроенным `cfg.Storage.Git.Remote`, the system SHALL дополнительно проверить наличие настроенного remote `origin` в `.git/config` и сообщить совпадает ли URL с конфигом.

**REQ-7.3** WHEN `jtpost doctor` обнаруживает в выводе git-status credentials или secrets из URL, the system SHALL маскировать пароль через `maskDSN`-стиль (общая утилита).

### Group 8 — Config validation

**REQ-8.1** WHEN `Config.Validate()` запускается с `Storage.Git.Enabled=true && Storage.Git.AutoPush=true && Storage.Git.Remote==""`, the system SHALL вернуть `errors.Join(core.ErrConfigInvalid, ...)`.

**REQ-8.2** WHEN `Config.Validate()` запускается с `Storage.Git.Enabled=true` и непустым `Storage.Git.CommitTemplate`, the system SHALL парсить шаблон и вернуть `ErrConfigInvalid` если парсер падает.

**REQ-8.3** WHEN `Config.Validate()` запускается с `Storage.Git.Branch == ""` и `Enabled=true`, the system SHALL fall-back на `"main"` (default уже установлен в `NewDefaultConfig`).

### Group 9 — Тесты

**REQ-9.1** WHEN запускается `task test`, the system SHALL пройти tests `internal/adapters/gitrepo/...`: декоратор create→commit, update→commit, delete→commit, list/get pass-through, invalid template fails Open, auto-init на пустом dir, push на bare-remote, mutex-concurrency.

**REQ-9.2** WHEN `repotest.RunContract` запускается через factory `Type=fs && Git.Enabled=true`, the system SHALL пройти все 18 поведенческих subtests (decorator pass-through всё CRUD).

**REQ-9.3** WHEN запускается `task test`, the system SHALL включать тест что `Config.Validate()` отклоняет `AutoPush=true && Remote=""` (REQ-8.1) и невалидный CommitTemplate (REQ-8.2).

## Topological Order

```
Group 8 (Config validation)        — фундамент валидации
       ↓
Group 3 (Commit-template)          — нужен для Group 1
       ↓
Group 1 (Decorator контракт)       — основа
       ↓
Group 2 (Auto-init и репо)         — внутри Group 1 при Open
       ↓
Group 5 (Concurrency)              — добавляется к Group 1
       ↓
Group 4 (Auto-push)                — после Group 1 (под тем же mutex)
       ↓
Group 6 (Storage factory)          — wiring после готовых Groups 1-5
       ↓
Group 7 (Doctor extension)         — потребитель Group 1
       ↓
Group 9 (Тесты)                    — финал
```

## Conflict Priority

**Конфликт 1.** REQ-1.5 (commit после inner success) vs REQ-4.3 (push fail не блокирует операцию).

**Resolution:** В нормальном случае последовательность: inner-write → success → commit → success → push. Если push упал, операция уже успешна (commit локально). Failed push логируется, но не отменяет commit и не возвращает error из CRUD-метода. Это сознательный UX-выбор: пользователь не теряет данные.

**Конфликт 2.** REQ-2.4 (detached HEAD → no commit, return success) vs REQ-1.5 (commit после inner success).

**Resolution:** REQ-2.4 имеет приоритет. Decorator проверяет HEAD-состояние при каждой мутации (быстрая проверка через `repo.Head()`); detached → skip git-commit, log warning, операция success. Пользователь получает данные на диске, но git-history не пополняется до возвращения на ветку.

## Open Design Questions

| Question | Why It Matters | Impacted Requirements |
|----------|---------------|----------------------|
| Где сериализовать `Time` в commit-template — UTC или local TZ? | Влияет на читаемость лога для команды в разных часовых поясах. | REQ-3.1 |
| `wt.Add` использует абсолютный или относительный путь? | go-git API требует относительного к worktree пути; нужен ли helper. | REQ-1.5 |
| Как обработать cleanup репо при `Close()` декоратора? | go-git не требует Close, но pgxpool требует — proxy логика. | REQ-1.1, REQ-6.1 |
| Push-strategy: force, fast-forward only, или с retry? | Влияет на безопасность при concurrent writers. Предложение: fast-forward only, без retry. | REQ-4.2, REQ-4.3 |
| Mutex location: внутри декоратора или через context-key? | Per-decorator mutex проще; cross-process — `.git/index.lock` от go-git автоматически. | REQ-5.1, REQ-5.2 |
| Как тестировать SSH-auth без реальной SSH-инфраструктуры? | Возможно skip integration tests, либо использовать tempdir bare-remote через file-protocol. | REQ-4.5, REQ-9.1 |
| Имя git-default-branch (`main` vs `master`)? | F1 default `main`. Подтвердить. | REQ-2.2, REQ-8.3 |

## Verification Commands

| Action               | Command                          | Source         |
|----------------------|----------------------------------|----------------|
| Test (unit)          | `task test`                      | `Taskfile.yml` |
| Test (race)          | `task test:race`                 | `Taskfile.yml` |
| Test (integration)   | `task test:integration`          | `Taskfile.yml` (без изменений в F3) |
| Build                | `task build`                     | `Taskfile.yml` |
| Lint                 | `task lint`                      | `Taskfile.yml` |
| Generate             | `task generate`                  | `Taskfile.yml` (без изменений) |
| Format               | `task fmt`                       | `Taskfile.yml` |
| Vet                  | `task vet`                       | `Taskfile.yml` |
