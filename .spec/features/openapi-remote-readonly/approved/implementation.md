# Implementation Report: `--remote` для read-only CLI (F5c)

## Summary

F5c расширяет F5b: extracted `runRemote` helper + `--remote` mode для `show`, `stats`, `plan`, `next` команд. Pattern repetitive — каждая command имеет `*_remote.go` файл с `run*Remote(ctx, cli, out)` функцией; CLI command-RunE добавляет branching через `runRemote(cmd, fn)`.

## Task Execution

- [x] **T-1** Extract `runRemote` helper + refactor list — GREEN
- [x] **T-2** Add `--remote` для show/stats/plan/next — GREEN (8 mock-server tests)
- [x] **T-3** Final + CHANGELOG — done
- [x] **T-4** GATE — fmt+vet+test+build все pass

## Final Verification

- **Tests** (`go test ./...`): 12 пакетов GREEN.
- **Build**: `task build` OK.

## Files Changed

**Created:**
- `internal/cli/show_remote.go`, `stats_remote.go`, `plan_remote.go`, `next_remote.go`
- `internal/cli/show_remote_test.go` (объединяет тесты для show/stats/plan/next)

**Modified:**
- `internal/cli/remote.go` — добавлен `runRemote(cmd, fn) (didRun, err)` helper
- `internal/cli/list.go`, `list_remote.go` — refactored на helper
- `internal/cli/list_remote_test.go` — обновлён под новую сигнатуру
- `internal/cli/show.go`, `stats.go`, `plan.go`, `next.go` — branching local/remote
- `CHANGELOG.md`

## Deviations

- `show --remote` принимает только UUID (slug-lookup через REST требует endpoint /posts/by-slug — отложен).
- JSON-only output (table-mode для apiclient.Post — F5d).
- Filter parameters (limit/offset/sort_by) для remote-mode упрощены: проброс только status/tag/search.

## Pipeline state

4/4 задач T-1..T-4 marked complete.
