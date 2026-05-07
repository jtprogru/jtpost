# OpenAPI client + `--remote` mode (F5b) — Task Plan

**Test Style Source:** Tier 2.
**Commands:** `task test`, `task build`, `task generate`.

## Coverage Matrix

| REQ | Task | CP |
|-----|------|-----|
| 1.x | T-1 | CP-1, 2, 10, 12 |
| 2.x, 3.x | T-2 | CP-4, 5, 6, 11, 16 |
| 4.x | T-3 | CP-3, 7, 8, 9, 13 |
| 5.x | T-4 | CP-15 |

## Work Type: Pure feature

---

## T-1 — Client codegen toolchain

***_Complexity: standard_*** ***_Requirements: REQ-1.x_*** ***_Preservation: CP-1, 2, 10, 12, 14_***

Subtasks:
- [ ] 1. **CODE** Создать `oapi-codegen-config-client.yaml`:
```yaml
package: apiclient
output: internal/adapters/apiclient/client.gen.go
generate:
  models: false  # types в отдельном пакете oapigen
  client: true
import-mapping:
  api/openapi.yaml: github.com/jtprogru/jtpost/internal/adapters/httpapi/oapigen
output-options:
  skip-prune: true
```
ВАЖНО: import-mapping syntax может варьироваться. Альтернатива — `models: true` тогда client дублирует types в apiclient package. Пробуем сначала reuse-через-import; если не работает — fallback на duplication.

- [ ] 2. **CODE** Создать `mkdir -p internal/adapters/apiclient` + `.gitkeep`.

- [ ] 3. **CODE** В `Taskfile.yml` обновить:
```yaml
  generate:
    cmds:
      - task: generate:sqlc
      - task: generate:openapi:types
      - task: generate:openapi:client

  generate:openapi:types:
    desc: Generate types из api/openapi.yaml
    cmds:
      - "go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen-config.yaml api/openapi.yaml"

  generate:openapi:client:
    desc: Generate HTTP client из api/openapi.yaml
    cmds:
      - "go run github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen -config oapi-codegen-config-client.yaml api/openapi.yaml"

  generate:openapi:
    desc: Alias — types + client
    cmds:
      - task: generate:openapi:types
      - task: generate:openapi:client
```
Удалить старый `generate:openapi` task (replaced).

- [ ] 4. **VERIFY** `task generate:openapi:client` → `internal/adapters/apiclient/client.gen.go` существует, package=apiclient, build clean.

- [ ] 5. **VERIFY** `task generate` clean (sqlc + types + client).

- [ ] 6. **VERIFY** `git diff --exit-code -- internal/adapters/apiclient` после двойного запуска `task generate:openapi:client` — clean.

NOTE про `import-mapping`: если spec referenced как `api/openapi.yaml` — mapping syntax зависит от oapi-codegen v2 docs. Иногда нужно `embedded-spec: false` + manual fix. Если codegen дублирует types — оставляем (apiclient self-contained), просто increased binary size.

---

## T-2 — CLI flags + newAPIClient helper

***_Complexity: standard_*** ***_Requirements: REQ-2.x, REQ-3.x_*** ***_Preservation: CP-4, 5, 6, 11, 16_***

Subtasks:
- [ ] 1. **CODE** В `internal/cli/root.go`:
  - Добавить persistent flags: `--remote URL` (string default "") и `--auth TOKEN` (string default "").
- [ ] 2. **CODE** Создать `internal/cli/remote.go`:
```go
package cli

import (
    "fmt"
    "net/http"
    "net/url"
    "os"
    "strings"

    "github.com/jtprogru/jtpost/internal/adapters/apiclient"
    "github.com/spf13/cobra"
)

// newAPIClient возвращает client для remote mode.
// Returns (nil, false, nil) если --remote не задан → caller использует local mode.
func newAPIClient(cmd *cobra.Command) (*apiclient.ClientWithResponses, bool, error) {
    remote, _ := cmd.Flags().GetString("remote")
    if remote == "" {
        return nil, false, nil
    }
    remote = strings.TrimRight(remote, "/")
    if _, err := url.ParseRequestURI(remote); err != nil {
        return nil, false, fmt.Errorf("invalid --remote URL: %w", err)
    }
    auth, _ := cmd.Flags().GetString("auth")
    if auth == "" {
        auth = os.Getenv("JTPOST_AUTH_TOKEN")
    }
    if auth == "" {
        return nil, false, fmt.Errorf("--auth required when using --remote (or set JTPOST_AUTH_TOKEN env)")
    }
    cli, err := apiclient.NewClientWithResponses(remote, apiclient.WithRequestEditorFn(
        func(_ context.Context, req *http.Request) error {
            req.Header.Set("Authorization", "Bearer "+auth)
            return nil
        },
    ))
    if err != nil {
        return nil, false, fmt.Errorf("apiclient init: %w", err)
    }
    return cli, true, nil
}
```
ВАЖНО: imports `context`. Add it.

- [ ] 3. **GREEN** `internal/cli/remote_test.go`:
  - TestNewAPIClient_NoRemote: --remote пустой → (nil, false, nil).
  - TestNewAPIClient_RemoteWithAuth: --remote=http://x --auth=t → (non-nil, true, nil).
  - TestNewAPIClient_RemoteNoAuth_NoEnv: --remote=http://x, --auth="", env пустой → error.
  - TestNewAPIClient_AuthFromEnv: --auth="" + JTPOST_AUTH_TOKEN=t (через t.Setenv) → success.
  - TestNewAPIClient_InvalidURL: --remote="not-a-url" → error.
  - TestNewAPIClient_TrimsTrailingSlash: --remote=http://x/ → URL без trailing slash.

  Helper для тестов: создать `*cobra.Command` с PersistentFlags `--remote`, `--auth`; установить через `cmd.Flags().Set("remote", "...")`.

- [ ] 4. **VERIFY** `task test ./internal/cli/...` GREEN.

---

## T-3 — `jtpost list --remote` integration

***_Complexity: standard_*** ***_Requirements: REQ-4.x_*** ***_Preservation: CP-3, 7, 8, 9, 13_***

Subtasks:
- [ ] 1. **CODE** В `internal/cli/list.go`:
  - В начале `RunE` сразу после загрузки cfg: `cli, isRemote, err := newAPIClient(cmd)`.
  - Если `err != nil` → return.
  - Если `isRemote`:
    - Построить params `apiclient.GetPostsParams` с status/tag/search/sort_by/sort_order.
    - `resp, err := cli.GetPostsWithResponse(ctx, &params)`. На ошибке HTTP — return.
    - Если `resp.StatusCode() == 401` → return error "unauthorized".
    - Если `resp.StatusCode() != 200` → return error.
    - `posts := *resp.JSON200` — generated returns slice.
    - Вывод через `printPostsJSON(stdout, postsToCorePost(posts))` или адаптер.
  - Иначе — текущий local-mode path (не меняется).
- [ ] 2. **CODE** Helper `postsToCorePost(generated []oapigen.Post) []*core.Post` для compat-вывода (`printTable`/`printPostsJSON` ожидают `[]*core.Post`). Можно через JSON-marshal/unmarshal или manual mapping. Pragmatic: marshal generated → unmarshal в core.Post через JSON tags, которые соответствуют schema.
   - Альтернатива: сделать отдельный `printRemotePostsJSON` для generated types — простой `json.Encoder.Encode(posts)`.
   - Решение: использовать `printRemotePostsJSON` (JSON-вывод напрямую) для F5b proof-of-concept; table-вывод — TODO в follow-up.
- [ ] 3. **GREEN** `internal/cli/list_remote_test.go`:
  - TestList_RemoteMode_Success: httptest.NewServer возвращает JSON `[{...}]`; `jtpost list --remote URL --auth TOKEN` → exit 0 + JSON output.
  - TestList_RemoteMode_401: server returns 401 → command returns error.
  - TestList_LocalMode_Unchanged: без --remote → существующий path работает.
- [ ] 4. **VERIFY** `task test ./internal/cli/...` GREEN.

---

## T-4 — Финал: tests + CHANGELOG + GATE

***_Complexity: mechanical_*** ***_Requirements: REQ-5.x_***

Subtasks:
- [ ] 1. **VERIFY** `task fmt && task vet && task test && task test:race` GREEN.
- [ ] 2. **VERIFY** `task generate && git diff --exit-code -- internal/adapters/{httpapi/oapigen,apiclient}` clean.
- [ ] 3. **VERIFY** `task build` OK.
- [ ] 4. **CODE** Обновить CHANGELOG секцией F5b.
- [ ] 5. **CODE** Обновить .jtpost.example.yaml (упомянуть `--remote` или ничего — env JTPOST_AUTH_TOKEN documented).
- [ ] 6. **VERIFY** Smoke (опц): `jtpost list --remote https://nope --auth fake` → ожидаемая ошибка.
