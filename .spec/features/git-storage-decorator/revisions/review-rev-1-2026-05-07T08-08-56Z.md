# Code Review: git-storage-decorator (F3)

## Verdict: PASS

Все 33 требования реализованы и покрыты тестами; 16 Correctness Properties прослеживаются в коде. Контракт-сьют `repotest.RunContract` подтверждает поведенческую парность GitDecorator с inner FS-адаптером (16 PASS + 2 SKIP по capability). Push-логика проверена через bare-tempdir-remote (file://-протокол). Lint выявил 9 minor-замечаний — ни одного critical или major. По verdict-rules verdict = `PASS`.

## Change Set

| File | Status | Notes |
|------|--------|-------|
| `internal/adapters/gitrepo/decorator.go` | ✅ Planned | GitDecorator + helpers |
| `internal/adapters/gitrepo/template.go` | ✅ Planned | parseCommitTemplate, renderMessage |
| `internal/adapters/gitrepo/author.go` | ✅ Planned | gitAuthor() из env+fallback |
| `internal/adapters/gitrepo/decorator_test.go` | ✅ Planned | 13 unit-тестов |
| `internal/adapters/gitrepo/template_test.go` | ✅ Planned | 6 тестов |
| `internal/adapters/gitrepo/push_test.go` | ✅ Planned | 2 push-теста |
| `internal/adapters/gitrepo/contract_test.go` | ✅ Planned | RunContract proxy |
| `internal/adapters/storage/factory.go` | ✅ Planned | git decorator wiring |
| `internal/adapters/storage/factory_test.go` | ✅ Planned | 2 dispatch теста |
| `internal/adapters/config/config.go` | ✅ Planned | Validate git extension + default template обновлён |
| `internal/adapters/config/config_test.go` | ✅ Planned | TestConfig_Validate_GitSection |
| `internal/cli/doctor.go` | ✅ Planned | checkGitRepo() |
| `internal/cli/doctor_test.go` | ✅ Planned | 3 git-теста |
| `CHANGELOG.md` | ✅ Planned | F3 секция |
| `.jtpost.example.yaml` | ✅ Planned | расширенный комментарий storage.git |
| `go.mod`, `go.sum` | ✅ Planned | go-git/v5 |

Никаких unexpected/missed файлов. План выполнен полностью.

## Requirements Traceability

| Requirement Group | Tests | Code | CPs | Verdict |
|-------------------|-------|------|-----|---------|
| REQ-1.1..1.7 (Decorator контракт) | `Test*Decorator_*`, `TestGitFS_RunContract` | `decorator.go` | CP-1..CP-3, CP-16 | ✅ |
| REQ-2.1..2.5 (Auto-init/repo) | `TestNewGitDecorator_AutoInit/ExistingRepo/DetachedHEAD/StaleLock` | `NewGitDecorator` | CP-4..CP-7 | ✅ |
| REQ-3.1..3.3 (Commit-template) | `TestParseCommitTemplate_*`, `TestRenderMessage_AllVars` | `template.go` | CP-8, CP-9 | ✅ |
| REQ-4.1..4.5 (Auto-push) | `TestGitDecorator_Push_*`, `TestConfig_Validate_GitSection/autopush_no_remote` | `pushChanges`, `resolveAuth` | CP-11, CP-12 | ✅ |
| REQ-5.1..5.2 (Concurrency) | `TestGitDecorator_Concurrent_NoLockCollision` | `mu sync.Mutex` | CP-13 | ✅ |
| REQ-6.1..6.3 (Factory) | `TestOpen_Dispatch_FS_GitEnabled/Disabled_Unchanged` | `storage/factory.go` | CP-14 | ✅ |
| REQ-7.1..7.3 (Doctor) | `TestCheckGitRepo_*` | `doctor.go:checkGitRepo` | CP-15 | ✅ |
| REQ-8.1..8.3 (Config validation) | `TestConfig_Validate_GitSection` (6 кейсов) | `config.go:Validate()` | CP-10, CP-11 | ✅ |
| REQ-9.1..9.3 (Тесты) | сами тесты | — | All CPs | ✅ |

Все 33 REQ покрыты ≥1 тестом, все 16 CP — кодом + тестами.

## Design Conformance

### 3.1 Architectural Boundaries ✅
- `internal/adapters/gitrepo` — новый изолированный пакет.
- `gitrepo` импортирует `core`, `config`, `go-git` — без cross-layer violation.
- Factory wraps без изменения inner — Decorator-pattern чистый.

### 3.2 Data Models ✅
- `GitDecorator` структура соответствует design §2.5.
- `TemplateVars` с 6 переменными — точно как в дизайне.
- `Capabilities{OptimisticLock=false, Transactions=false}` для FS+Git соответствует design §2.6.

### 3.3 API Contracts ✅
- `NewGitDecorator(inner, postsDir, cfg) (*GitDecorator, error)` — точно как design §2.3.
- Все 6 CRUD методов соблюдают `core.PostRepository` контракт.
- `ImportPosts`/`Count` proxy реализован сверх-design (с FS-fallback) — задокументировано в implementation.md.

### 3.4 Error Handling ✅
- 14 сценариев из design §2.7 покрыты:
  - Auto-init на missing dir → `os.MkdirAll` + `PlainInit`.
  - Не git-репо → init.
  - Невалидный template → `errors.Join(ErrConfigInvalid, ...)`.
  - Stale lock → удаление.
  - Detached HEAD → warning + skip commit (REQ-2.4).
  - inner failure → пробросить без git.
  - Push fail → soft-fail (REQ-4.3, ADR-4).
  - Empty repo + initial commit → go-git справляется автоматически.

### 3.5 Correctness Properties ✅
Все 16 CP прослеживаются (см. Traceability таблицу):
- CP-1..16 покрыты конкретными тестами.

### 3.6 Documentation Consistency ✅
- Mermaid в design §2.2 показывает gitrepo зелёным (NEW), factory/doctor жёлтыми (MODIFIED) — соответствует реальному коду.
- Имена компонентов (`GitDecorator`, `parseCommitTemplate`, `gitAuthor`, `commitChanges`, `pushChanges`) совпадают.

## Code Quality

### 4.1 Naming & Clarity ✅
Все идентификаторы соответствуют Go-конвенциям. Helpers (`commitChanges`, `pushChanges`, `relativePath`, `resolveAuth`) — speaking names.

### 4.2 Dead Code & Debug Artifacts ✅
- Ни одного `fmt.Println`/`TODO без тикета`.
- `slog.Warn` использован для soft-fail logging — это intentional, не debug.
- Удалена замечание subagent'ом опечатка `stalLockThreshold`.

### 4.3 Scope Creep ⚠ (minor)
- `GitDecorator.ImportPosts`/`Count` имеют FS-fallback сверх плана — оправдано для прохождения `RunContract` тестов с FS inner. Документировано в implementation.md.
- F1-default `CommitTemplate` обновлён с `"chore: update post {{.Slug}}"` на `"chore: {{.Operation}} post {{.Slug}}"` — minor улучшение F1 (smoke test показал что хардкод "update" неправильный для create/delete операций).

### 4.4 Test Quality ✅
- Все тесты табличные где применимо (parseCommitTemplate_Invalid, RenderMessage_AllVars, Validate_GitSection).
- Concurrent test проверяет реальную race-condition (10 goroutines + WaitGroup), пропускает race detector.
- Push-тесты используют bare-tempdir через file:// — самодостаточные, без сети.

## Security

✅ Безопасных проблем не найдено в изменённых файлах.

- **Input validation:** template parsing валидируется в `parseCommitTemplate` + `Config.Validate()`.
- **Auth secrets:** `GIT_HTTPS_TOKEN` читается из env, не логируется. SSH через ssh-agent — нет credential storage.
- **DSN/URL masking:** `maskDSN` (из cli/migrate_db.go) переиспользован в `doctor.go` для git remote URL вывода.
- **Injection:** template rendering — bytes.Buffer, не shell. Filesystem paths из `post.Slug` идут через `filepath.Join` (без traversal риска внутри tenant subdir).
- **Concurrency:** `sync.Mutex` в decorator + `.git/index.lock` от go-git защищают от race conditions внутри и между процессами.
- **Soft-fail на push:** push не блокирует операцию (commit локально); это документировано как UX-решение (ADR-4) и логируется через `slog.Warn`.

## Verification Evidence

Команды re-run в этой ревью-сессии:

- **Tests** (`go test ./...`):
```
ok  	github.com/jtprogru/jtpost/internal/adapters/config	0.840s
ok  	github.com/jtprogru/jtpost/internal/adapters/fsrepo	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/gitrepo	1.253s
ok  	github.com/jtprogru/jtpost/internal/adapters/httpapi	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/sqlite	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/storage	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegram	(cached)
ok  	github.com/jtprogru/jtpost/internal/adapters/telegramconv	(cached)
ok  	github.com/jtprogru/jtpost/internal/cli	1.274s
ok  	github.com/jtprogru/jtpost/internal/core	(cached)
ok  	github.com/jtprogru/jtpost/internal/logger	(cached)
```

- **Race** (`go test -race ./...`): все 11 пакетов GREEN, no data races (включая 10-goroutine concurrent test в gitrepo).

- **Build** (`task build`):
```
task: [tidy] go mod tidy
task: [build] go mod download
task: [build] CGO_ENABLED=0 go build -o ./dist/jtpost cmd/jtpost/main.go
```

- **Lint** (`task lint`): 9 finding'ов, все minor:
```
* contextcheck: 3 (включая 1 в decorator.Count walks — оправдано: filepath.WalkDir не использует ctx)
* funcorder: 1 (pre-existing F2)
* gochecknoglobals: 1 (test-helper fixedTenantID)
* nilnil: 2 (pre-existing F2)
* staticcheck: 1 (test embedded field selector)
* unparam: 1 (initBareGitRepo возвращает unused dir)
```
Все — minor severity, ни одного critical/major.

## Findings

| ID | Severity | File | Description | Requirement |
|----|----------|------|-------------|-------------|
| F-1 | minor | `internal/adapters/gitrepo/decorator.go:219` | `Count` walks через `filepath.WalkDir` без ctx-проброса (contextcheck). Не баг — WalkDir по локальному FS не имеет lifecycle через ctx. | REQ-1.6 |
| F-2 | minor | `internal/adapters/gitrepo/decorator_test.go:23` | `fixedTenantID` объявлен как global — нарушает gochecknoglobals. Допустимо для test-fixture. | — |
| F-3 | minor | `internal/adapters/gitrepo/decorator_test.go:392` | `decorator.inner.(*fsrepo.FileSystemPostRepository).FileSystemPostRepository` — staticcheck QF1008 (можно убрать embedded field). Стилевое. | — |
| F-4 | minor | `internal/cli/doctor_test.go:173` | `initBareGitRepo` возвращает string который нигде не используется (unparam). | — |
| F-5..F-9 | minor | прочие | Pre-existing F2 finding'и (funcorder, nilnil, contextcheck в doctor.go) — не относятся к F3. | — |

## Recommendations

**Minor (follow-up задачи, не блокируют PASS):**
1. **F-1**: при добавлении ctx-aware WalkDir (Go 1.23+ предлагает iter.Seq2) — обновить `Count`. Сейчас норма.
2. **F-2**: переместить `fixedTenantID` внутрь helper-функции `mkPost` или сделать `t.Helper()` фабрику. Косметическое.
3. **F-3**: упростить test-utility selector. Косметическое.
4. **F-4**: убрать unused return из `initBareGitRepo` (`func initBareGitRepo(t, dir)` без string). Косметическое.

Все — кандидаты в follow-up PR или maintenance-коммит.

## Pipeline state

8/8 задач T-1..T-8 marked complete. Артефакт зарегистрирован.
