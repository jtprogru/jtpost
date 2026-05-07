# Implementation Report: OpenAPI client + `--remote` mode (F5b)

## Summary

F5b реализована: generated HTTP-client (`apiclient.ClientWithResponses` со всеми 13 operations) + CLI глобальные флаги `--remote/--auth` + helper `newAPIClient` + proof-of-concept `jtpost list --remote URL --auth TOKEN` через apiclient. Локальный режим без `--remote` сохраняется без изменений.

## Task Execution

- [x] **T-1** Client codegen toolchain — GREEN (operationId `login` → `authLogin` чтобы избежать collision с schema `LoginResponse`)
- [x] **T-2** CLI flags + newAPIClient helper — GREEN (6 unit tests)
- [x] **T-3** `jtpost list --remote` integration — GREEN (3 tests с httptest mock-server)
- [x] **T-4** GATE — fmt+vet+test+race+generate+build все pass

## Final Verification

- **Tests** (`go test ./...`): 12 пакетов GREEN.
- **Race**: 12 пакетов GREEN.
- **Generate**: clean (sqlc + types + client).
- **Build**: `task build` OK.

## Files Changed

**Created:**
- `oapi-codegen-config-client.yaml`
- `internal/adapters/apiclient/client.gen.go` (~2000 lines generated)
- `internal/cli/remote.go`, `remote_test.go`
- `internal/cli/list_remote.go`, `list_remote_test.go`

**Modified:**
- `Taskfile.yml` — generate aggregate с 3 sub-tasks
- `internal/cli/root.go` — `--remote`, `--auth` global flags
- `internal/cli/list.go` — branching local/remote
- `api/openapi.yaml` — operationId `login` → `authLogin` (collision fix)
- `internal/adapters/httpapi/oapigen/types.gen.go` — regenerated
- `CHANGELOG.md`
- `go.mod`, `go.sum` — `oapi-codegen/runtime`

## Deviations

- **operationId rename**: `login` → `authLogin` для избежания collision с `LoginResponse` schema в client codegen. Не влияет на API public contract (operationId — internal к OpenAPI tooling).
- **JSON-only output для remote**: `runListRemote` использует `json.Encode` напрямую вместо `printPostsJSON`/`printTable` (которые ожидают `[]*core.Post`). Conversion `apiclient.Post → core.Post` отложен в F5c (table-output можно реализовать через generated types напрямую).
- **`contextBackground()` helper**: defensive fallback если `cmd.Context()` nil (cobra в not-executed-from-root scenarios).

## Pipeline state

4/4 задач T-1..T-4 marked complete.
