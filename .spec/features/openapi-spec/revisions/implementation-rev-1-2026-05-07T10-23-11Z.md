# Implementation Report: OpenAPI 3.1 Spec + types codegen (F5)

## Summary

F5 (start of B.3) реализована: формальная OpenAPI 3.1 спецификация всех 13 public endpoints + generated Go types через `oapi-codegen` v2 (types-only mode) + `/api/v1/` aliases (backward-compat) + refactor LoginHandler. Server-side handlers и CLI client отложены в F5b/c.

## Task Execution

- [x] **T-1** OpenAPI spec (subagent) — GREEN (~600 строк yaml; 38 generated top-level types)
- [x] **T-2** oapi-codegen toolchain + Taskfile — GREEN (`tools.go`, `oapi-codegen-config.yaml`, `task generate:openapi`)
- [x] **T-3** Refactor LoginHandler использует oapigen.LoginRequest/LoginResponse — GREEN
- [x] **T-4** `/api/v1/` aliases через `bothPrefixes` helper — GREEN (`TestAPIV1_LoginAliasWorks` PASS для legacy + v1)
- [x] **T-5** Tests + CHANGELOG — GREEN
- [x] **T-6** GATE — fmt+test+race+generate+build все pass

## Final Verification

- **Tests** (`go test ./...`): 12 пакетов GREEN.
- **Race**: 12 пакетов GREEN.
- **Generate**: `task generate` clean (sqlc + oapi-codegen).
- **Build**: `task build` OK.

## Files Changed

**Created:**
- `api/openapi.yaml` (~600 строк, 13 paths, 38 schemas)
- `tools.go` (build-tag tools)
- `oapi-codegen-config.yaml`
- `internal/adapters/httpapi/oapigen/types.gen.go` (generated, committed)
- `internal/adapters/httpapi/api_v1_test.go`

**Modified:**
- `Taskfile.yml` — `generate` aggregate, `generate:sqlc`, `generate:openapi`
- `internal/adapters/httpapi/auth_handlers.go` — LoginHandler use oapigen types
- `internal/adapters/httpapi/auth_handlers_test.go` — обновлены под oapigen types
- `internal/adapters/httpapi/server.go` — `bothPrefixes`/`bothPrefixesFunc` helpers, все routes регистрируются под legacy + `/api/v1/` префиксами
- `CHANGELOG.md`
- `go.mod`, `go.sum` — `oapi-codegen/v2`, `oapi-codegen/runtime`

## Deviations

- **OpenAPI 3.1 partial support**: oapi-codegen warning (v2.7.0) о partial support OpenAPI 3.1. Codegen работает, types корректные. Documented.
- **`Stats`/`PlanItem`/`TagsResponse`** schema форма выровнена с фактическими handler outputs (`map[PostStatus]int`, `jsonPlannedPost` shape, `{tags: []string}` wrapper).
- **`CreatePostRequest`/`UpdatePostRequest`** — partial DTOs, отдельно от `Post` (handlers принимают partial body).
- **`bothPrefixes` helper** — простая path-double registration, не нужен router-mw.

## Pipeline state

6/6 задач T-1..T-6 marked complete.
