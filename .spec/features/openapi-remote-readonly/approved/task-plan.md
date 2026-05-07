# `--remote` для read-only CLI команд (F5c) — Task Plan

**Test Style Source:** Tier 2 (как F5b).
**Commands:** `task test`, `task build`.

## Coverage Matrix

| REQ | Tasks |
|-----|-------|
| 1.1..1.4 (4 commands) | T-2 |
| 1.5 (local unchanged) | T-2 |
| 2.1..2.2 (helper) | T-1 |
| 3.1 (tests) | T-2 |

## Work Type: Pure feature (additive)

---

## T-1 — Extract `runRemote` helper + refactor list

***_Complexity: standard_*** ***_Requirements: REQ-2.x_***

Subtasks:
- [ ] 1. **CODE** В `internal/cli/remote.go` добавить:
  ```go
  func runRemote(cmd *cobra.Command, fn func(ctx context.Context, cli *apiclient.ClientWithResponses) error) (didRun bool, err error) {
      cli, isRemote, err := newAPIClient(cmd)
      if err != nil { return false, err }
      if !isRemote { return false, nil }
      ctx := cmd.Context()
      if ctx == nil { ctx = context.Background() }
      return true, fn(ctx, cli)
  }
  ```
  Move `contextBackground()` из list_remote.go в remote.go (можно удалить — `context.Background()` inline достаточно).
- [ ] 2. **CODE** Refactor `list_remote.go:runListRemote` принимать ctx параметром (вместо cmd.Context()-в-теле) и быть совместимым с `runRemote(fn)` сигнатурой. Обновить `list.go` использовать `runRemote`.
- [ ] 3. **VERIFY** TestList_RemoteMode_* проходят без изменений.

---

## T-2 — Add `--remote` для show, stats, plan, next

***_Complexity: standard_*** ***_Requirements: REQ-1.x_***

Subtasks:
- [ ] 1. **CODE** Создать `internal/cli/show_remote.go`:
  ```go
  func runShowRemote(ctx context.Context, cli *apiclient.ClientWithResponses, idOrSlug string, out io.Writer) error {
      // Parse uuid; если не uuid — поддержать slug (через GetPostBySlug? endpoint не существует — только /posts/{id}). F5c: только UUID.
      id, err := uuid.Parse(idOrSlug)
      if err != nil { return fmt.Errorf("--remote requires UUID id, got %q", idOrSlug) }
      resp, err := cli.GetPostWithResponse(ctx, id)
      // ... 200 / 401 / 404 / 5xx handling
  }
  ```
- [ ] 2. **CODE** В `show.go` добавить branching: `runRemote(cmd, ...)` → если didRun → return.
- [ ] 3. **CODE** Аналогично `stats_remote.go`, `plan_remote.go`, `next_remote.go`.
- [ ] 4. **CODE** Обновить `stats.go`, `plan.go`, `next.go` с branching.
- [ ] 5. **GREEN** Тесты `*_remote_test.go` через httptest mock-server: success + 401 + (404 для show).
- [ ] 6. **VERIFY** `task test ./internal/cli/...` GREEN.

---

## T-3 — Финал

Subtasks:
- [ ] 1. `task test && task test:race && task build` GREEN.
- [ ] 2. CHANGELOG update.
- [ ] 3. `task generate && git diff --exit-code` clean.

---

## T-4 — GATE
