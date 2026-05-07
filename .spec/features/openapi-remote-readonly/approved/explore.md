# Exploration: `--remote` для read-only CLI команд (F5c)

## Intent

F5b дал proof-of-concept `jtpost list --remote URL --auth TOKEN`. F5c расширяет `--remote` на все read-only CLI команды: `show`, `stats`, `plan`, `tags`, `next`. Write-команды (`new`, `edit`, `delete`, `publish`) и server-side ServerInterface codegen — F5d/e.

**Scope F5c:**

1. `jtpost show <id-or-slug> --remote URL --auth TOKEN` — через `cli.GetPostWithResponse(ctx, id)`.
2. `jtpost stats --remote ...` — через `cli.GetStatsWithResponse(ctx)`.
3. `jtpost plan --remote ...` — через `cli.GetPlanWithResponse(ctx)`.
4. `jtpost tags --remote ...` — wait, `tags` нет как separate cmd? Check. (могло быть только `list --tag`).
5. `jtpost next --remote ...` — через `cli.GetNextPostWithResponse(ctx)`.

Если какие-то commands не существуют — пропустить.

**Чего F5c НЕ делает:**
- Не реализует write-commands `--remote`. F5d.
- Не делает server-side ServerInterface. F5e.
- Не делает retry/backoff/caching.
- Не делает interactive auth login через `--remote`.

---

## Investigation

### Existing CLI commands

Существующие cmd в `internal/cli/`:
- `new`, `list`, `show`, `edit`, `delete`, `publish`, `plan`, `next`, `stats`, `import`, `migrate`, `migrate_db`, `migrate_ids`, `serve`, `doctor`, `init`, `status`, `user`, `token`.

Read-only из них: `show`, `list` (✅ done in F5b), `plan`, `next`, `stats`, `status` (status = `git status`-like), `doctor`, `init` (config-write, не read).

`tags` отдельной command нет — это часть `list --tag X`.

F5c targets: `show`, `plan`, `next`, `stats`.

### Existing helper

`internal/cli/remote.go:newAPIClient(cmd)` — переиспользуем.
`internal/cli/list_remote.go:contextBackground()` — extract в shared helper.

### Generated client API

```go
// apiclient.ClientWithResponses methods (relevant subset):
GetPostWithResponse(ctx, id uuid.UUID) // GET /posts/{id}
GetStatsWithResponse(ctx)              // GET /stats
GetPlanWithResponse(ctx, params)       // GET /plan
GetNextPostWithResponse(ctx)           // GET /next
ListTagsWithResponse(ctx)              // GET /tags
```

Точные имена нужно проверить в client.gen.go.

---

## Build Tooling — без изменений.

---

## Options Considered

### Option A: Extract shared helper для remote-mode (recommended)

Создать `runRemote(cmd, fn)` helper, который:
- Builds apiclient.
- Calls user's `fn` с client.
- Wraps standard error-handling (401, 5xx, connection).

Каждая command-cmd в branching local/remote использует helper. DRY.

### Option B: Inline branching в каждой command

Copy-paste `if isRemote { ... }` в каждой command. Repetitive.

---

## Recommended Direction

**Option A** + extract `runRemote(cmd, handler)` helper.

Steps:
1. Refactor `list_remote.go:runListRemote` под общий helper.
2. Extract `contextBackground` в `remote.go`.
3. Создать `show_remote.go`, `stats_remote.go`, `plan_remote.go`, `next_remote.go` через тот же pattern.
4. Tests: один mock-server для каждой команды.
5. CHANGELOG update.

---

## Scope Boundaries

### Must-have (F5c)

- `--remote` для `show`, `stats`, `plan`, `next` (4 commands).
- Shared `runRemote` helper.
- Tests: мок-server для каждой команды.

### Deferred (F5d+)

- `--remote` для `new/edit/delete/publish`.
- Server-side ServerInterface codegen.
- Table-mode для remote-вывода.
- Tags-filter parsing (если бы был отдельная cmd).

---

## Assumptions

- `[ASSUMPTION: existing local-mode для каждой команды НЕ меняется]`
- `[ASSUMPTION: все 4 endpoints generated в client.gen.go]`
- `[ASSUMPTION: JSON-вывод достаточен для F5c proof; table-mode позже]`

---

## Done When

- [x] Codebase читан.
- [x] 2 опции.
- [x] Trade-offs.
- [x] Scope.
- [x] Assumptions.
- [ ] Артефакт зарегистрирован.
