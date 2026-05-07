# Exploration: Git-storage Decorator (F3)

## Intent

F3 — следующая после F2 фича большой программы доведения jtpost до финала. F1 зафиксировал доменную модель и схему конфига, F2 реализовал три backend (fs/sqlite/postgres) и storage factory. F3 даёт **реальную имплементацию** для секции `storage.git.*`, которая в F1 описана в схеме конфига, но не имеет behaviour в коде.

**Цели F3:**

1. Реализовать decorator-паттерн над `core.PostRepository`: `gitrepo.NewGitDecorator(inner, cfg)`. Внутренний репо — fs (пишет файлы), декоратор после каждой мутации делает `git add` + `git commit` (опционально `git push`).
2. Подключить декоратор в `storage.Open(cfg)`: при `cfg.Storage.Type == "fs"` и `cfg.Storage.Git.Enabled == true` factory оборачивает fs-репо в git-decorator. Для sqlite/postgres — git не применим (бинарный формат, выходит за scope F3).
3. Использовать pure-Go `github.com/go-git/go-git/v5` — без зависимости на CLI `git`.
4. Auto-init: если `posts_dir` не git-репо при `Open()` и `git.enabled=true` → автоматически `git init` (опц. с `git.initial_branch`).
5. Commit-шаблон через `text/template` с переменными `.Slug`, `.Title`, `.ID`, `.Status`, `.Operation` (create/update/delete).
6. Auto-push (если `auto_push=true` и `remote` задан) — синхронно после commit, с таймаутом и масirovaniem ошибок (failed push не должен блокировать локальную запись).
7. Concurrency-safety: декоратор сериализует git-операции через mutex (`.git/index` lock не выдержит параллельных коммитов).
8. Тесты: full unit + интеграционные тесты на bare-tempdir-репозитории через go-git без сети.
9. CLI/UX: `jtpost doctor` сообщает статус git-репо (clean/dirty/ahead/behind) при `storage.git.enabled=true`.

**Чего F3 не делает:**
- Не реализует разрешение конфликтов merge (no concurrent writers from outside jtpost expected).
- Не делает branch-management (создание/удаление веток), кроме автоматического создания initial branch.
- Не реализует git-history просмотр через CLI (`jtpost log` отложен в B-этап).
- Не обёртывает sqlite/postgres — git только над fs.
- Не делает signed commits (GPG) — отложено на потенциальный F11.
- Не интегрирует worker-publisher (F6).

**Триггер:** F2 закрыт (commit `23b756a`), `storage.git.*` уже валидируется в `Validate()` (косвенно — через нет ошибок), но `Storage.Git.Enabled=true` ничего не меняет в runtime.

---

## Investigation

### Что уже есть после F1+F2

**Конфиг (`internal/adapters/config/config.go:64-72`):**
```go
type GitStorageConfig struct {
    Enabled        bool   `yaml:"enabled" mapstructure:"enabled"`
    AutoCommit     bool   `yaml:"auto_commit" mapstructure:"auto_commit"`
    AutoPush       bool   `yaml:"auto_push" mapstructure:"auto_push"`
    Remote         string `yaml:"remote" mapstructure:"remote"`
    Branch         string `yaml:"branch" mapstructure:"branch"`
    CommitTemplate string `yaml:"commit_template" mapstructure:"commit_template"`
}
```
Дефолты (`config.go:127-133`): `AutoCommit=true`, `Branch="main"`, `CommitTemplate="chore: update post {{.Slug}}"`. Env-bind есть для всех 6 полей.

**fs-репозиторий (`internal/adapters/fsrepo/repository.go`):**
- `Create` → `os.WriteFile(<root>/<tenant_short>/<slug>.md, ...)`
- `Update` → `os.WriteFile` (атомарно НЕ через temp+rename, прямой OWriteFile — это выявит F3 в lint).
- `Delete` → `os.Remove`
- Возвращает `*core.Post` без знания о git.

**Storage factory (`internal/adapters/storage/factory.go`):**
- `OpenAs(cfg, "fs")` → `fsrepo.NewFileSystemRepository(cfg.PostsDir)` + `nopCloser{}`.
- НЕ проверяет `cfg.Storage.Git.Enabled` — F3 закроет.

**CLI doctor (`internal/cli/doctor.go:checkStorage`):**
- При `Type=fs` сейчас просто пишет «fs (используется PostsDir)». F3 расширит: если `git.Enabled` → также проверка git-репо.

### Зависимости

- `go-git/go-git/v5` — pure-Go, поддерживает HTTPS+SSH transport, ssh-agent, basic-auth, OAuth-token. Активно развивается, последняя версия `v5.13` (январь 2026).
- Альтернативы: `git2go` (CGO + libgit2 — nope, проект CGO_ENABLED=0); CLI-wrapper через `os/exec git` (нет — нужно убирать ручной CLI-инструмент).
- Размер бинарника: go-git добавит ~6 МБ (приемлемо для CLI-tool).

### Тестовый контекст

- Стиль: native testing + table-driven, как в F2.
- Для F3 — unit-tests с tempdir-репо (go-git позволяет `PlainInit` локально; bare-repo как «fake remote» через `PlainInit(true)`). Никакого Docker не нужно.
- Push-логику можно тестировать: создаём bare-repo в tempdir → передаём как `remote.URL` в clone → проверяем что commit прилетел.

### Архитектурные точки касания

- `internal/adapters/gitrepo/` — новый пакет (decorator).
- `internal/adapters/storage/factory.go` — wiring при `Type=fs && Git.Enabled`.
- `internal/cli/doctor.go` — расширение `checkStorage` (git status check).
- `Taskfile.yml` — без изменений (тесты go-git unit, integration не нужны).
- `go.mod` — `go-git/v5` + transitive.

### Что F3 НЕ затрагивает

- `internal/adapters/sqlite`, `internal/adapters/postgres` — git не оборачивает SQL-backend (бинарный формат, сценарий не имеет смысла).
- `internal/adapters/repotest/contract.go` — уже работает: декоратор-обёртка проходит контракт, потому что Read/List pass-through, а Write/Update/Delete делегируют inner. RunContract для git+fs не требует изменений сьюта.
- `internal/core/*` — без изменений.

---

## Build Tooling

- **Orchestrator:** Task (`Taskfile.yml`).
- **Test:** `task test` (unit + go-git).
- **Test (race):** `task test:race`.
- **Build:** `task build`.
- **Lint:** `task lint`.
- **Generate:** `task generate` (sqlc, без изменений).
- **Source:** `Taskfile.yml`.

CI: GitHub Actions, без изменений (no Docker required for git tests).

---

## Options Considered

### Option A: Decorator над `core.PostRepository` (рекомендуемый)

Новый пакет `internal/adapters/gitrepo` с типом `GitDecorator{inner core.PostRepository; repo *git.Repository; cfg GitConfig; mu sync.Mutex}`. Все методы `core.PostRepository` делегируют inner; после Create/Update/Delete (если success) выполняется `commitChanges(operation, post)`.

- **Pros:** Чистый паттерн (Decorator), повторно используется для будущего `gitsqliterepo` (если когда-нибудь захотим экспортировать SQL-снимки в git). Ноль изменений в `fsrepo`. Интерфейс `core.PostRepository` остаётся single-source.
- **Cons:** Лишний слой type-assertion в factory; mu-lock сериализует git-ops, что может стать узким местом при массовом импорте (но импорт делается реже редкого).
- **Сложность:** Medium.

### Option B: Git-логика встроена в `fsrepo`

Добавить `fsrepo.FileSystemPostRepository.gitCfg *GitConfig` поле; внутри Create/Update/Delete после успеха — git-commit.

- **Pros:** Меньше типов; нет двойной диспатч-логики в factory.
- **Cons:** Нарушает SRP: fsrepo становится «и FS, и git одновременно». Сложнее тестировать FS без git и git без FS отдельно. Conditional-логика во всех мутациях.
- **Сложность:** Low (по строкам), но High по поддерживаемости.

### Option C: External tool — shell-out на CLI `git`

Вместо go-git вызывать `git add . && git commit -m '...'` через `os/exec`.

- **Pros:** Знакомо для devops; не добавляет зависимостей в go.mod.
- **Cons:** Не работает в Docker без git-CLI; проблемы с auth (passphrases для SSH); парсинг stdout/stderr; race-condition если несколько процессов; платформенные различия (Windows). НЕТ.
- **Сложность:** Medium, но reliability low.

### Option D: Async commit через worker-channel

Запустить goroutine, которая получает `commitTask` из канала и делает commit пакетом. Мутации возвращают сразу, commit отстаёт.

- **Pros:** UX быстрее (нет git-overhead на write).
- **Cons:** Атомарность нарушена (CRASH = lost commit), сложная error-propagation, не понятно как dorabotat при `Close()`. Подходит для Web UI (F8), не для CLI.
- **Сложность:** High.

---

## Constraints & Risks

### Атомарность

- FS-write успех + git-commit fail → данные на диске, но не в коммите. Решение: log warning, не fail-операцию (post сохранён). При следующем write попробуем все pending как один коммит, либо игнорируем и оставляем как dirty-tree (пользователь может закоммитить руками).
- FS-write fail → ничего не делаем с git.
- При `auto_push=true` push-fail → log error, операция считается success (commit локально есть, push можно повторить через `jtpost doctor` или ручной `git push`).

### Concurrency

- go-git не concurrency-safe (`.git/index` lock). Все git-ops в decorator под `mu sync.Mutex`. Read-операции (GetByID/List) НЕ блокируются — они идут через inner FS, который в свою очередь стейтлесс.
- Мьютекс per-decorator (per-process). Cross-process safety: `.git/index.lock` файл от go-git предотвратит реальные races между двумя `jtpost`-процессами на одной машине.

### Производительность

- Один post-write добавляет ~30-50ms (open repo, add, commit). Приемлемо для интерактивного CLI.
- ImportPosts (массовый): N постов = N коммитов. Можно опционально коалесцировать в один коммит (`commit_batch` flag), но это deferred (v2).
- Push добавляет latency сети — синхронно в F3, async в F6 (worker).

### Безопасность

- SSH-auth: jtpost читает `~/.ssh/id_rsa` (через ssh-agent). Не хранит пароли.
- HTTPS-auth: через env-переменные `GIT_USERNAME`/`GIT_PASSWORD` или git-credential helper. F3 — sensible defaults: ssh-agent first, HTTPS-token fallback.
- НЕ комментировать сами `secret`/`token` в commit messages.
- `.gitignore` НЕ генерируем (пользователь сам решает что коммитить).

### Зависимости (новые)

- `github.com/go-git/go-git/v5` (~6 MB)
- transitive: `github.com/go-git/gcfg`, `github.com/go-git/go-billy/v5` и т.д.
- Удаляем уязвимости: пинит `go-git/v5 ≥ v5.13.0` (свежий security-patch).

### Edge cases

- **PostsDir не git-репо при `Enabled=true`**: auto-init (`PlainInit`). Лог info: «git repo initialized».
- **PostsDir уже git-репо**: используем как есть. Branch может отличаться от `cfg.Storage.Git.Branch` — не переключаем (это хирургия, оставляем как warning в doctor).
- **Initial commit на пустом репо**: первый Create делает first commit (no parent). go-git с этим справляется через `Worktree.Commit`.
- **Detached HEAD**: пользователь может быть в detached state. Failing-fast: log warning, не делаем auto-commit. Оператор должен переключиться на ветку.
- **Conflict при push**: remote ahead (другой push сделан в межвремье) → push отклонён. Log error: «push rejected, run git pull --rebase`. F3 не разрешает конфликты автоматически.
- **`commit_template` имеет неvalidный шаблон**: ошибка при `Open()` decorator — `errors.Join(core.ErrConfigInvalid, ...)`.
- **`auto_push=true && remote=""`**: ошибка валидации в `Config.Validate()` (новое правило в F3).
- **Tenant изоляция**: git-history будет показывать commits для всех tenants одного `posts_dir`. В multi-tenant scenario это может быть нежелательно. F3 не решает (single-tenant repo per posts_dir типичный сценарий).
- **`.git`-каталог как часть posts_dir**: при tenant-listing в FS-режиме fsrepo уже игнорирует non-`.md` файлы (см. `repository.go:130`), плюс `.git` — каталог, отфильтрован `entry.IsDir()`. ОК.

---

## Recommended Direction

**Option A (Decorator pattern)** — единственный right way:

1. **Новый пакет `internal/adapters/gitrepo`:**
   - `GitDecorator` обёртка над `core.PostRepository`.
   - Конструктор `NewGitDecorator(inner core.PostRepository, postsDir string, cfg config.GitStorageConfig) (*GitDecorator, error)`:
     - Открывает/инициализирует git-репо в `postsDir` (PlainOpen, fallback PlainInit).
     - Парсит `commit_template` (validate at open time).
     - Получает дефолтную identity (env `GIT_AUTHOR_NAME`/`GIT_AUTHOR_EMAIL` → fallback на `jtpost <bot@jtpost.local>`).
   - Все CRUD-методы:
     - Read (Get/List): pass-through inner.
     - Write (Create/Update/Delete): inner-call → если success → `commitChanges(ctx, op, post)` под mutex.
   - `commitChanges`:
     - `wt.Add(<relative_path>)` (для Delete — `wt.Remove`).
     - `wt.Commit(msg, &git.CommitOptions{Author: ...})`.
     - Если `auto_push` → `repo.PushContext(ctx, opts)` с timeout 30s. Failed push → log warning, error не пробрасываем.
   - Реализует `core.MigratableRepository` если inner реализует (proxy).
   - Реализует `core.TransactionalRepository` если inner реализует (proxy без git-транзакций — git commit идёт после inner commit).
2. **Wiring в factory** (`storage/factory.go`):
   - В `OpenAs(cfg, "fs")`: после `fsrepo.NewFileSystemRepository(cfg.PostsDir)`, если `cfg.Storage.Git.Enabled` → обернуть в `gitrepo.NewGitDecorator`.
3. **`Config.Validate()` extension** (`config.go`):
   - При `Storage.Git.Enabled=true && AutoPush=true && Remote==""` → `ErrConfigInvalid`.
   - При `CommitTemplate==""` → fallback на default ("chore: update post {{.Slug}}").
   - Парсить шаблон на валидность при `Validate()` (early failure).
4. **Doctor extension** (`cli/doctor.go`):
   - В `checkStorage` для `Type=fs && Git.Enabled`: открыть репо, выполнить `git status` (clean/dirty), `git rev-list HEAD ^upstream` (ahead count), вернуть человекочитаемый статус.
5. **Тесты:**
   - `gitrepo/decorator_test.go` — unit с tempdir-репо: Create → проверка коммита, Update → второй коммит, Delete → коммит с remove. Ошибочный шаблон → конструктор fails.
   - `gitrepo/push_test.go` — два tempdir (worktree + bare-remote) → push verification.
   - Дополнить `factory_test.go` — кейс `Type=fs && Git.Enabled=true` → возвращает GitDecorator.
   - `repotest.RunContract` гонять через GitDecorator — должен быть GREEN (всё через inner).
6. **CHANGELOG/docs**: F3 секция, breaking changes — нет (git был зашитой схемой, теперь работает).

---

## Scope Boundaries

### Must-have (v1, эта фича)

- `gitrepo.NewGitDecorator` с full F1-CRUD контрактом (proxy + commit hook).
- Auto-init `git init` если posts_dir не репо.
- Commit-template parsing с переменными `.Slug`, `.Title`, `.ID`, `.Status`, `.Operation`.
- Auto-commit на Create/Update/Delete.
- Auto-push (sync, с timeout) при `AutoPush=true`.
- Mutex-based concurrency safety.
- `Config.Validate()` extension: `AutoPush+empty Remote`, `CommitTemplate validation`.
- Storage factory wiring при `Type=fs && Git.Enabled`.
- `jtpost doctor`: git status для fs+git режима.
- Полные unit + push-тесты через go-git in-tempdir.
- `RunContract` GREEN через GitDecorator.

### Deferred (v2 / последующие фичи)

- **F6 worker:** async-push/commit для batch-публикаций.
- **F7 telegram media:** медиафайлы → git LFS (отдельный спайк).
- **F8 web UI:** показ git-history для поста.
- **F11/Maintenance:** signed commits (GPG/SSH-key), `jtpost log` команда.
- **B-этап:** разрешение конфликтов через `git pull --rebase`, conflict-resolution UI.
- Coalesce-batch commits (один коммит на N постов) — performance optimization.

### Needs spike

- **Lock-handling между несколькими jtpost-процессами**: go-git создаёт `.git/index.lock`, но не управляет timeouts. Если процесс crashed с lock — следующий повиснет. Решение в F3: проверка staleness lock-file при Open (если age > 60s, считаем stale, удаляем). Минорный спайк.
- **Performance ImportPosts**: N коммитов или один coalesce. Изначально в F3 — N коммитов (простой), measure → решить про coalesce на основе бенчмарков.

---

## Assumptions & Open Questions

### Assumptions

- `[ASSUMPTION: go-git/v5 покрывает все нужды, без CLI git]` — подтверждаем для CGO_ENABLED=0 совместимости.
- `[ASSUMPTION: git decorator применяется только к fs-backend]` — sqlite/postgres в git не имеет смысла (бинарь). Зафиксировано в F2 explore.
- `[ASSUMPTION: один posts_dir = один git-репо = один tenant в реальном сценарии]` — multi-tenant в одном git-репо допустим, но git-history миксируется. Не блокирует F3.
- `[ASSUMPTION: failed push не fail-ит операцию write]` — UX-приоритет: пользователь не должен терять локально-сохранённый пост из-за сетевой ошибки.
- `[ASSUMPTION: identity берётся из env GIT_AUTHOR_NAME/EMAIL → fallback "jtpost <bot@jtpost.local>"]` — может быть изменено опциональным полем `storage.git.author` в config, но F3 без него.
- `[ASSUMPTION: при отсутствии .git в posts_dir факторий делает auto-init]` — снимает первичный setup для пользователя.
- `[ASSUMPTION: при detached HEAD авто-commit не делается, операция возвращается success без commit + warning в логе]` — не падать на пользовательской ошибке.
- `[ASSUMPTION: commit-template НЕ-валидный фейлит Open() decorator]` — fail-fast.

### Open Questions (для Requirements)

1. **Commit-template variables: только Post-fields или включать context (CurrentTime, User)?** Предложение: минимально — `Slug`, `Title`, `ID`, `Status`, `Operation` (create/update/delete), `Time`. Расширения — в follow-up.
2. **Auth для push: ssh-agent only, или поддержать HTTP-token (env)?** Предложение: ssh-agent + `GIT_HTTPS_TOKEN` env (поддерживается go-git через `BasicAuth`).
3. **Behavior на failed initial-commit (empty tree)?** В go-git первый commit на репо без файлов работает. Но если posts_dir пустой — ничего не коммитим до первого Create. Норма?
4. **Push timeout: 30s default? Опция в конфиге?** Предложение: 30s hardcoded в F3, опция `storage.git.push_timeout` — в follow-up.
5. **Если `auto_push=true && remote != ""` но `git remote` не настроен — auto-add-remote?** Предложение: НЕТ (опасно, может перезаписать существующий). Лог error «remote not configured, run `git remote add origin <url>` или установить storage.git.remote=URL манupally в `.git/config`».
6. **Нужно ли сохранять author identity в config (`storage.git.author_name`, `author_email`)?** Предложение: НЕТ в F3. Stick to env+fallback. Опция — в follow-up.
7. **`jtpost log` команда (показать git-history)?** Deferred.
8. **Pre-commit hooks (e.g. linting)?** Deferred (F11).

---

## Done When (для approve)

Quality Checklist:

- [x] Codebase прочитан (cited: `internal/adapters/config/config.go:64-72,127-133,224-258`, `internal/adapters/fsrepo/repository.go:108-200`, `internal/adapters/storage/factory.go`, F2 explore §«Edge cases» — git decorator decision).
- [x] Сравнены 4 опции (A/B/C/D) с pros/cons/complexity.
- [x] Trade-off'ы явные.
- [x] Scope boundaries: Must-have / Deferred / Needs spike — категоризированы.
- [x] Assumptions помечены тэгом `[ASSUMPTION: ...]`.
- [x] Open Questions есть (8 пунктов).
- [x] Build Tooling зафиксирован (без изменений vs F2).
- [ ] Артефакт зарегистрирован через `pipeline.sh artifact` — следующий шаг.
