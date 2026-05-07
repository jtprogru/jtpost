# OpenAPI client + `--remote` mode (F5b) — Design

**Status:** Draft

## 2.1 Overview

3 части: (1) client codegen toolchain (separate config), (2) CLI flags + helper, (3) `jtpost list --remote` рефакторинг с branching local/remote.

## 2.2 Architecture

```mermaid
graph TB
    Spec["api/openapi.yaml (F5)"]
    ClientConfig["oapi-codegen-config-client.yaml - NEW"]
    ApiClient["internal/adapters/apiclient/client.gen.go - NEW"]
    Oapigen["oapigen.Post (F5)"]

    Spec --> ClientConfig
    ClientConfig --> ApiClient
    ApiClient -.imports.-> Oapigen

    Root["internal/cli/root.go - MODIFIED"]
    Remote["internal/cli/remote.go - NEW"]
    List["internal/cli/list.go - MODIFIED"]

    Root -->|--remote/--auth flags| Remote
    Remote --> ApiClient
    List -->|if --remote| Remote
    List -->|else| LocalRepo[Bundle (local)]

    style ClientConfig fill:#90EE90
    style ApiClient fill:#90EE90
    style Remote fill:#90EE90
    style Root fill:#FFD700
    style List fill:#FFD700
```

## 2.3 Components

### Files Requiring Changes

| File | Status | Description |
|------|--------|-------------|
| `oapi-codegen-config-client.yaml` | NEW | Client config |
| `internal/adapters/apiclient/client.gen.go` | NEW | Generated |
| `Taskfile.yml` | MODIFIED | `:client` task + alias |
| `internal/cli/root.go` | MODIFIED | --remote, --auth flags |
| `internal/cli/remote.go` | NEW | newAPIClient helper |
| `internal/cli/list.go` | MODIFIED | branching local/remote |
| `internal/cli/list_test.go` | MODIFIED | + remote test |
| `internal/cli/remote_test.go` | NEW | helper tests |
| `CHANGELOG.md`, `.jtpost.example.yaml` | MODIFIED | docs |

### Interfaces

```go
// internal/cli/remote.go
package cli

import "github.com/jtprogru/jtpost/internal/adapters/apiclient"

// newAPIClient возвращает (client, true) если --remote задан, иначе (nil, false).
func newAPIClient(cmd *cobra.Command) (*apiclient.ClientWithResponses, bool, error)
```

## 2.4 Key Decisions

### ADR-1: Separate config files

Two configs: `oapi-codegen-config.yaml` (types in oapigen) + `oapi-codegen-config-client.yaml` (client in apiclient with import-mapping to reuse types).

### ADR-2: ClientWithResponses (не raw Client)

Typed responses — convenient для error-handling.

### ADR-3: --remote bool-by-presence vs explicit URL

URL обязателен — нет default value. Пустое = local mode.

### ADR-4: Auth via RequestEditorFn

`apiclient.WithRequestEditorFn(func(ctx, req) error { req.Header.Set("Authorization", "Bearer "+token); return nil })`.

## 2.5 Data Models

Без новых domain types. Generated `apiclient.Client`/`ClientWithResponses` + методы typed-per-operation.

## 2.6 Correctness Properties

```
Property 1: Client codegen produces methods for all spec operations
Category: Equivalence
Statement: For all paths*methods в openapi.yaml, generated client имеет corresponding method (e.g. GetPostsWithResponse).
Validates: REQ-1.5
```

```
Property 2: types reused через import-mapping
Category: Propagation
Statement: Generated client использует `oapigen.Post` (не дублирует тип).
Validates: REQ-1.1
```

```
Property 3: Local mode unchanged
Category: Equivalence
Statement: `jtpost list` без --remote → identical поведение к F5 (local Bundle).
Validates: REQ-4.2
```

```
Property 4: --remote requires --auth
Category: Absence
Statement: --remote без --auth и без JTPOST_AUTH_TOKEN env → exit 1 with error.
Validates: REQ-2.3
```

```
Property 5: Bearer header injection
Category: Propagation
Statement: Все HTTP-requests от newAPIClient содержат `Authorization: Bearer <token>`.
Validates: REQ-3.2
```

```
Property 6: Auth env-fallback
Category: Equivalence
Statement: --auth empty + JTPOST_AUTH_TOKEN=X → token=X.
Validates: REQ-2.2
```

```
Property 7: Remote list returns API data
Category: Round-trip
Statement: jtpost list --remote URL → apiclient.GetPostsWithResponse → printPostsJSON/printTable identical к local-mode для same data.
Validates: REQ-4.1
```

```
Property 8: 401 → exit 1 + error
Category: Absence
Statement: Mock-server returns 401 → CLI exit 1 with "unauthorized" message.
Validates: REQ-4.3
```

```
Property 9: Connection error → exit 1
Category: Absence
Statement: --remote URL points to nonexistent → exit 1 with error.
Validates: REQ-4.4
```

```
Property 10: Generate freshness
Category: Round-trip
Statement: `task generate` → git diff --exit-code -- internal/adapters/apiclient → clean.
Validates: REQ-1.2
```

```
Property 11: Invalid URL → error
Category: Absence
Statement: --remote "not-a-url" → newAPIClient returns error.
Validates: REQ-3.3
```

```
Property 12: Generate task aggregate
Category: Equivalence
Statement: `task generate` runs sqlc + openapi:types + openapi:client.
Validates: REQ-1.4
```

```
Property 13: Filter parameters propagation
Category: Propagation
Statement: jtpost list --remote --status draft → API call with status param.
Validates: REQ-4.1
```

```
Property 14: Build clean
Category: Absence
Statement: task build (с generated client) succeeds.
Validates: REQ-1.5
```

```
Property 15: Tests pass
Category: Equivalence
Statement: existing tests без --remote pass без изменений; new TestList_Remote pass.
Validates: REQ-5.x
```

```
Property 16: --remote URL trim
Category: Equivalence
Statement: --remote "http://x:8080/" trimmed trailing slash before client init.
Validates: REQ-3.2
```

## 2.7 Error Handling

| Scenario | Detection | Action |
|----------|-----------|--------|
| --remote без --auth/env | newAPIClient | error "auth required" |
| Invalid URL | url.Parse fail | error |
| 401 from API | client response StatusCode | exit 1 |
| 5xx | client response StatusCode | exit 1 |
| Connection refused | client error | exit 1 |
| Generated code drift | git diff in CI | flag in task generate check |

## 2.8 Testing Strategy

| Test | Description |
|------|-------------|
| TestNewAPIClient_NoRemote | --remote пустой → returns nil, false, nil |
| TestNewAPIClient_RemoteWithAuth | --remote + --auth → client, true, nil; auth-header injected |
| TestNewAPIClient_RemoteNoAuth | --remote без --auth/env → error |
| TestNewAPIClient_AuthFromEnv | env JTPOST_AUTH_TOKEN → используется |
| TestNewAPIClient_InvalidURL | bad URL → error |
| TestList_LocalMode_Unchanged | без --remote → as before |
| TestList_RemoteMode_Success | mock server returns posts → CLI prints them |
| TestList_RemoteMode_401 | mock returns 401 → exit error |
