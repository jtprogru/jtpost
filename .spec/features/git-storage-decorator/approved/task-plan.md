# Git-storage Decorator (F3) — Task Plan

**Test Style Source:** Tier 2
- Reference test files: `internal/adapters/sqlite/repository_test.go`, `internal/adapters/storage/factory_test.go`, `internal/adapters/repotest/contract.go`.
- Key patterns: native `testing` пакет, table-driven через `tt := []struct{...}{...}`, `t.Run` для subtests, `t.TempDir()` для эфемерных директорий, helper-функции локально в `*_test.go`.
- PBT note: PBT-библиотек нет; substitute через targeted unit tests с явным cartesian product (>3 input combinations per property).

**Commands:**

| Action               | Command                  | Source         |
|----------------------|--------------------------|----------------|
| Test (unit)          | `task test`              | `Taskfile.yml` |
| Test (race)          | `task test:race`         | `Taskfile.yml` |
| Build                | `task build`             | `Taskfile.yml` |
| Lint                 | `task lint`              | `Taskfile.yml` |
| Format               | `task fmt`               | `Taskfile.yml` |
| Vet                  | `task vet`               | `Taskfile.yml` |
| Generate (sqlc)      | `task generate`          | `Taskfile.yml` (без изменений) |

---

## Coverage Matrix

| Requirement | Task(s) | Correctness Property |
|-------------|---------|----------------------|
| REQ-1.1     | T-3     | CP-1, CP-4 |
| REQ-1.2     | T-2, T-3 | CP-10 |
| REQ-1.3     | T-3     | CP-3 |
| REQ-1.4     | T-3     | CP-2 |
| REQ-1.5     | T-3     | CP-1 |
| REQ-1.6     | T-3     | CP-16 |
| REQ-1.7     | T-3     | CP-3 |
| REQ-2.1     | T-3     | CP-4 |
| REQ-2.2     | T-3     | CP-4 |
| REQ-2.3     | T-3     | CP-5 |
| REQ-2.4     | T-3     | CP-6 |
| REQ-2.5     | T-3     | CP-7 |
| REQ-3.1     | T-2     | CP-8 |
| REQ-3.2     | T-2     | CP-9 |
| REQ-3.3     | T-2     | CP-8 |
| REQ-4.1     | T-1     | CP-11 |
| REQ-4.2     | T-3, T-5 | CP-12 |
| REQ-4.3     | T-3, T-5 | CP-12 |
| REQ-4.4     | T-3     | CP-12 |
| REQ-4.5     | T-3, T-5 | CP-12 |
| REQ-5.1     | T-3     | CP-13 |
| REQ-5.2     | T-3     | CP-13 |
| REQ-6.1     | T-4     | CP-14 |
| REQ-6.2     | T-4     | CP-14 |
| REQ-6.3     | T-4     | CP-14 |
| REQ-7.1     | T-6     | CP-15 |
| REQ-7.2     | T-6     | CP-15 |
| REQ-7.3     | T-6     | CP-15 |
| REQ-8.1     | T-1     | CP-11 |
| REQ-8.2     | T-1, T-2 | CP-10 |
| REQ-8.3     | T-1     | — |
| REQ-9.1     | T-3, T-5, T-7 | All |
| REQ-9.2     | T-7     | CP-1, CP-3 |
| REQ-9.3     | T-1     | CP-10, CP-11 |

Каждое требование привязано к ≥1 задаче; каждый CP-N — к ≥1 задаче.

---

## Work Type Classification

**Pure feature** — новый пакет `gitrepo`, новые goroutine/file-paths, no migration of existing data. Pre-existing fsrepo и factory остаются без поведенческих изменений когда `Git.Enabled=false`.

Task order: GREEN (test stubs) → CODE (bottom-up) → GREEN (full tests) → GATE.

---

## T-1 — Foundation: Config validation для git-секции

***_Complexity: mechanical_***
***_Requirements: REQ-4.1, REQ-8.1, REQ-8.2, REQ-8.3, REQ-9.3_***
***_Preservation: CP-10, CP-11_***

GOAL: расширить `Config.Validate()` для git-секции. Это даст compile-time + runtime guard для последующих задач.

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/config/config.go` функция `Validate()`:
  - При `c.Storage.Git.Enabled && c.Storage.Git.AutoPush && c.Storage.Git.Remote == ""` → `errors.Join(core.ErrConfigInvalid, errors.New("storage.git.auto_push=true requires storage.git.remote"))`.
  - При `c.Storage.Git.Enabled && c.Storage.Git.CommitTemplate != ""` → попытаться `template.New("commit").Parse(c.Storage.Git.CommitTemplate)`. Если падает — `errors.Join(core.ErrConfigInvalid, ...)`.
  - При `c.Storage.Git.Enabled && c.Storage.Git.Branch == ""` → fallback на `"main"` (мутировать `c.Storage.Git.Branch = "main"`). Default уже стоит в `NewDefaultConfig`, но если пользователь явно задал пустую строку через env — fallback нужен.
- [ ] 2. **GREEN** В `internal/adapters/config/config_test.go` добавить table-driven тест `TestConfig_Validate_GitSection`:
  - 6 кейсов: `{Enabled=false, *}` → ok (git выключен), `{Enabled=true, AutoPush=true, Remote=""}` → fail, `{Enabled=true, AutoPush=true, Remote="git@..."}` → ok, `{Enabled=true, CommitTemplate="{{.Slug"}` → fail (broken syntax), `{Enabled=true, CommitTemplate=""}` → ok (default kicks in), `{Enabled=true, Branch=""}` → mutates to `"main"`.
- [ ] 3. **VERIFY** `go test ./internal/adapters/config/...` GREEN.

NOTE: `text/template` import нужен в config.go — добавить.

---

## T-2 — Commit-template + author identity (gitrepo внутренние утилиты)

***_Complexity: standard_***
***_Requirements: REQ-1.2, REQ-3.1, REQ-3.2, REQ-3.3, REQ-8.2_***
***_Preservation: CP-8, CP-9, CP-10_***

GOAL: подготовить независимые helper-utilities для T-3.

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/gitrepo/template.go`:
  - `const defaultCommitTemplate = "chore: {{.Operation}} post {{.Slug}}"`
  - `type TemplateVars struct { Slug, Title, ID, Status, Operation string; Time time.Time }`
  - `func parseCommitTemplate(s string) (*template.Template, error)` — если `s == ""` → парсить default; иначе парсить `s`. На ошибке — обернуть в `errors.Join(core.ErrConfigInvalid, err)`.
  - `func renderMessage(tmpl *template.Template, op string, post *core.Post) string` — `vars := TemplateVars{Slug: post.Slug, Title: post.Title, ID: post.ID.String(), Status: string(post.Status), Operation: op, Time: time.Now().UTC()}`; рендер в bytes.Buffer; на runtime-ошибке (unlikely для validated template) → fallback `fmt.Sprintf("chore: %s post %s", op, post.Slug)`.
- [ ] 2. **CODE** Создать `internal/adapters/gitrepo/author.go`:
  - `func gitAuthor() *object.Signature`. Импорт: `"github.com/go-git/go-git/v5/plumbing/object"`.
  - Логика: name = env `GIT_AUTHOR_NAME` || `"jtpost"`; email = env `GIT_AUTHOR_EMAIL` || `"bot@jtpost.local"`; When = `time.Now().UTC()`.
- [ ] 3. **GREEN** Создать `internal/adapters/gitrepo/template_test.go`:
  - `TestParseCommitTemplate_Default` — `parseCommitTemplate("")` → шаблон рендерится в `"chore: create post my-slug"` для test-post.
  - `TestParseCommitTemplate_Valid` — кастомный шаблон с 6 переменными, рендеринг возвращает заполненную строку.
  - `TestParseCommitTemplate_Invalid` — `parseCommitTemplate("{{.Slug")` → `errors.Is(err, core.ErrConfigInvalid)`.
  - `TestRenderMessage_AllVars` — table-driven: 5 различных post-fixtures × 3 operations = 15 рендерингов; каждый проверяет что все поля присутствуют.
  - `TestGitAuthor_FromEnv` — `t.Setenv("GIT_AUTHOR_NAME", "alice"); t.Setenv("GIT_AUTHOR_EMAIL", "a@x")` → signature имеет эти значения.
  - `TestGitAuthor_Fallback` — `t.Setenv("GIT_AUTHOR_NAME", ""); t.Setenv("GIT_AUTHOR_EMAIL", "")` → fallback values.
- [ ] 4. **VERIFY** `go test ./internal/adapters/gitrepo/...` GREEN.

NOTE: `go get github.com/go-git/go-git/v5` если ещё не в go.mod (вероятно нет — добавится здесь).

---

## T-3 — GitDecorator: ядро (auto-init, mutex, commit, push)

***_Complexity: complex_***
***_Requirements: REQ-1.1, REQ-1.3..1.7, REQ-2.1..2.5, REQ-4.2..4.5, REQ-5.1, REQ-5.2_***
***_Preservation: CP-1..CP-7, CP-12, CP-13, CP-16_***

GOAL: реализовать `GitDecorator` с полным CRUD контрактом.

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/gitrepo/decorator.go`:
  - `type GitDecorator struct { inner core.PostRepository; repo *git.Repository; postsDir string; cfg config.GitStorageConfig; template *template.Template; mu sync.Mutex; detached bool }`.
  - `NewGitDecorator(inner, postsDir, cfg)`:
    - parseCommitTemplate → если ошибка, return.
    - `os.MkdirAll(postsDir, 0o755)` если не существует.
    - Удалить stale `.git/index.lock` (mtime > 60s).
    - `git.PlainOpen(postsDir)` → если `git.ErrRepositoryNotExists` → `git.PlainInitOptions{Bare: false, InitOptions{DefaultBranch: plumbing.NewBranchReferenceName(cfg.Branch)}}` (используем `git.PlainInitWithOptions` из go-git v5.13+; если API недоступен — `git.PlainInit` + `repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(cfg.Branch)))`).
    - Detect detached HEAD: `head, err := repo.Head()` → если err == plumbing.ErrReferenceNotFound (репо без коммитов) → `detached=false` (норма для пустого репо); иначе если `head.Name() == plumbing.HEAD` (без branch) → `detached=true`, log warning.
- [ ] 2. **CODE** В том же файле — методы CRUD:
  - GetByID/GetBySlug/List/Count → pass-through к `d.inner` без mutex.
  - Create/Update/Delete:
    - Вызвать inner-метод. Если ошибка — return error без git.
    - Если success: `d.mu.Lock(); defer d.mu.Unlock()`.
    - Если `d.detached` → return nil (success, без коммита).
    - `commitChanges(ctx, op, post)`. Ошибка → log warning + return nil (не блокировать операцию).
    - Если `cfg.AutoPush` → `pushChanges(ctx)`. Ошибка → log warning + return nil.
- [ ] 3. **CODE** Helpers:
  - `relativePath(post) string` → `filepath.Join(post.TenantShortID(), post.Slug+".md")`.
  - `commitChanges(ctx, op, post)`:
    - `wt, err := d.repo.Worktree()`. На ошибке return.
    - Для Delete: `wt.Remove(rel)`. Для Create/Update: `wt.Add(rel)`. Если go-git ошибка `path not found` → пропустить (file уже удалён через inner).
    - `msg := renderMessage(d.template, op, post)`.
    - `_, err = wt.Commit(msg, &git.CommitOptions{Author: gitAuthor()})`. Return err.
  - `pushChanges(ctx)`:
    - `ctx, cancel := context.WithTimeout(ctx, 30*time.Second); defer cancel()`.
    - Auth: `auth := resolveAuth(d.cfg.Remote)`.
    - `repo.PushContext(ctx, &git.PushOptions{RemoteName: "origin", Auth: auth, Force: false})`. Return err.
  - `resolveAuth(remote string) transport.AuthMethod`:
    - Если `strings.HasPrefix(remote, "https://")` и есть `os.Getenv("GIT_HTTPS_TOKEN")` → `&http.BasicAuth{Username: "token", Password: token}`.
    - Если SSH-URL (`git@...`) → `ssh.NewSSHAgentAuth("git")` (если ssh-agent доступен; иначе nil — go-git попробует defaults).
    - Иначе nil.
- [ ] 4. **CODE** Реализовать `core.MigratableRepository` proxy если inner реализует:
  - `ImportPosts(ctx, posts) error`: вызов `d.inner.(core.MigratableRepository).ImportPosts(ctx, posts)`. Если success — ОДИН batch-commit: для каждого post в posts → `wt.Add(relativePath(post))`. Затем один `wt.Commit("chore: import N posts", ...)`. Если AutoPush — push.
  - `Count(ctx)` → proxy в inner.
- [ ] 5. **CODE** `Close() error` метод: если inner реализует `io.Closer` — вызвать; иначе nil. Это нужно для совместимости с `storage.Open`-сигнатурой (factory возвращает `io.Closer`).
- [ ] 6. **GREEN** Создать `internal/adapters/gitrepo/decorator_test.go` с table-driven тестами (см. дизайн §2.8). Минимум:
  - newRepo(t) helper — возвращает GitDecorator над in-memory FS-репо в `t.TempDir()`.
  - TestNewGitDecorator_AutoInit, ExistingRepo, DetachedHEAD, StaleLock, FreshLock, InvalidTemplate.
  - TestGitDecorator_Create_AddsCommit, Update_AddsCommit, Delete_AddsCommit (проверять через `repo.Log` итерацию).
  - TestGitDecorator_Create_InnerFail_NoCommit (через mock-inner возвращающий ошибку).
  - TestGitDecorator_Read_PassThrough.
  - TestGitDecorator_Detached_NoCommit.
  - TestGitDecorator_ImportPosts_BatchCommit (3 поста → 1 коммит).
  - TestGitDecorator_Concurrent_NoLockCollision (10 goroutines × Create через `sync.WaitGroup` → 10 коммитов).
- [ ] 7. **VERIFY** `task test ./internal/adapters/gitrepo/... && task test:race ./internal/adapters/gitrepo/...` GREEN.

NOTE: go-git API для `PlainInit` с branch: если `git.PlainInitOptions` не доступен в текущей версии, используй low-level `repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.NewBranchReferenceName(cfg.Branch)))` сразу после PlainInit.
NOTE: Для mock-inner-fail test используй простую struct реализующую `core.PostRepository` с конфигурируемыми ошибками.
DO NOT: трогать config/, fsrepo/, storage/, doctor/ — это T-1, T-4, T-6.

---

## T-4 — Storage factory wiring

***_Complexity: mechanical_***
***_Requirements: REQ-6.1, REQ-6.2, REQ-6.3_***
***_Preservation: CP-14_***

GOAL: интегрировать `gitrepo.NewGitDecorator` в `storage.Open`/`OpenAs`.

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/storage/factory.go`:
  - В `case "fs"` или после `fsrepo.NewFileSystemRepository`: если `cfg.Storage.Git.Enabled` → обернуть в `gitrepo.NewGitDecorator(repo, cfg.PostsDir, cfg.Storage.Git)`. На ошибке → return.
  - Тип возвращаемого `closer`: для git+fs → сам decorator (он реализует `Close()` через REQ T-3 step 5). Для чистого fs — nopCloser{} как раньше.
- [ ] 2. **GREEN** В `internal/adapters/storage/factory_test.go` добавить:
  - `TestOpen_Dispatch_FS_GitEnabled` — `Storage.Type=fs`, `Storage.Git.Enabled=true`, `PostsDir=t.TempDir()` → `repo` имеет тип `*gitrepo.GitDecorator` (через type assertion).
  - `TestOpen_Dispatch_FS_GitDisabled` — текущий case остаётся работать (uvarennее не сломан).
- [ ] 3. **VERIFY** `task test ./internal/adapters/storage/...` GREEN.

---

## T-5 — Push integration tests (через bare-tempdir-remote)

***_Complexity: standard_***
***_Requirements: REQ-4.2, REQ-4.3, REQ-4.5, REQ-9.1_***
***_Preservation: CP-12_***

GOAL: проверить push-логику через локальный bare-репо как fake-remote.

Subtasks:
- [ ] 1. **CODE** Создать `internal/adapters/gitrepo/push_test.go`:
  - Helper `newBareRemote(t) string` — `t.TempDir()` + `git.PlainInit(dir, true)` (bare) → возвращает `file://`-URL (`"file://" + dir`).
  - TestGitDecorator_Push_Success_BareRemote:
    - newBareRemote → URL.
    - newRepo(t) с `cfg.AutoPush=true, cfg.Remote=URL`.
    - До первого Create: `repo.CreateRemote(&config.RemoteConfig{Name: "origin", URLs: []string{URL}})`.
    - Create post → ожидаем коммит локально + push в bare.
    - Проверка: открыть bare-репо, `bare.Log(...)` показывает коммит.
  - TestGitDecorator_Push_Failed_NoOpReturn:
    - cfg с remote = `"file:///nonexistent/path"`.
    - Create post → ожидаем nil error (push fail soft); проверка что post всё равно создан (inner FS).
  - TestGitDecorator_Push_Timeout:
    - Не реализуем настоящий timeout (требует unreachable remote с задержкой). Substitute: проверяем что `pushChanges` использует context.WithTimeout 30s через введение тестового hook (если стоит изобретать инфраструктуру для одного теста — пропустить или mark `t.Skip("requires unreachable remote infra")`). Допустим первые 2 кейса.
- [ ] 2. **VERIFY** `task test ./internal/adapters/gitrepo/...` GREEN с push-тестами.

NOTE: file://-протокол go-git поддерживает нативно для bare-репо.

---

## T-6 — Doctor extension

***_Complexity: standard_***
***_Requirements: REQ-7.1, REQ-7.2, REQ-7.3_***
***_Preservation: CP-15_***

GOAL: расширить `jtpost doctor` диагностикой git-репо.

Subtasks:
- [ ] 1. **CODE** В `internal/cli/doctor.go`:
  - В `checkStorage` для `cfg.Storage.Type == "fs"`: ПОСЛЕ вывода о fs (текущая логика), если `cfg.Storage.Git.Enabled` → добавить вывод нового check. Можно реструктурировать: вернуть `[]checkResult` вместо одного `checkResult` (либо добавить второй check внутри `checkStorage` через выделение в отдельную функцию `checkGitRepo(cfg)`).
  - Лучше: добавить отдельную функцию `checkGitRepo(cfg *config.Config) []checkResult`. В `runDoctor` вызывать её только если `Type=fs && Git.Enabled`.
  - `checkGitRepo`:
    - PlainOpen `cfg.PostsDir`. Ошибка → fail check.
    - `wt.Status()` → если clean → ok; иначе → warn с `len(status)` файлов.
    - Если `cfg.Storage.Git.Remote != ""`:
      - `repo.Remote("origin")` → если не настроен — warn `"remote not configured"`.
      - Если URL не совпадает с `cfg.Storage.Git.Remote` → warn (REQ-4.4).
    - В выводе маскировать креденшелы через `maskDSN` если URL `https://` с `:password@`.
- [ ] 2. **GREEN** В `internal/cli/doctor_test.go` добавить:
  - TestDoctor_Storage_FsGit_Clean — git-enabled репо без изменений → `checkGitRepo` → 1 result level=OK с "clean".
  - TestDoctor_Storage_FsGit_Dirty — после ручного `os.WriteFile` файла, не закоммиченного → status сообщает "dirty".
  - TestDoctor_Storage_FsGit_RemoteMismatch — настроен `git remote add origin <X>`, но cfg.Remote=Y → warn.
- [ ] 3. **VERIFY** `task test ./internal/cli/...` GREEN.

NOTE: maskDSN уже есть в `internal/cli/migrate_db.go` — использовать оттуда.

---

## T-7 — RunContract через GitDecorator + finalization

***_Complexity: standard_***
***_Requirements: REQ-9.1, REQ-9.2_***
***_Preservation: CP-1, CP-3_***

GOAL: запустить общий contract-сьют через GitDecorator и убедиться что декоратор не ломает inner-контракт.

Subtasks:
- [ ] 1. **CODE** В `internal/adapters/gitrepo/decorator_test.go` (или отдельный `contract_test.go`):
  - `TestGitFS_RunContract`:
    ```go
    repotest.RunContract(t, func(t *testing.T) (core.PostRepository, repotest.Capabilities, func()) {
        t.Helper()
        dir := t.TempDir()
        inner, err := fsrepo.NewFileSystemRepository(dir)
        if err != nil { t.Fatal(err) }
        cfg := config.GitStorageConfig{Enabled: true, Branch: "main"}
        dec, err := gitrepo.NewGitDecorator(inner, dir, cfg)
        if err != nil { t.Fatal(err) }
        return dec, repotest.Capabilities{OptimisticLock: false, Transactions: false}, func() {}
    })
    ```
  - Запустить — все 18 subtests должны проходить (read pass-through; write пройдут c доп git-overhead).
- [ ] 2. **VERIFY** `task test ./internal/adapters/gitrepo/...` GREEN.

---

## T-8 — GATE: финальная проверка

***_Complexity: mechanical_***
***_Requirements: ALL_***

CRITICAL: Эта задача — последняя. Не выполнять до завершения T-1 .. T-7.

Instructions:
1. `task fmt && task vet && task lint` — все pass.
2. `task test` — 100% GREEN.
3. `task test:race` — 100% GREEN, no data races.
4. `task build` — собирается.
5. Smoke-test:
   - tempdir → `.jtpost.yaml` с `storage.git.enabled: true`.
   - `./dist/jtpost init --force` → должен создать posts_dir + auto-init git.
   - `./dist/jtpost new --title smoke` → пост создан + git-commit auto-сделан.
   - `cd <posts_dir> && git log --oneline` → виден commit с template-сообщением.
6. Обновить `CHANGELOG.md` секцией F3 (added: gitrepo decorator, auto-commit, auto-push, doctor git-status).
7. Обновить `.jtpost.example.yaml` — добавить пример секции `storage.git` с пояснениями.
8. Если хоть один шаг fail — вернуться к ответственной задаче, не закрывать GATE.
