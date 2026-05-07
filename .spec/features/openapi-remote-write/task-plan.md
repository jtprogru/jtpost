# Task Plan — F5d

## T-1 Implement
- `internal/cli/delete_remote.go`: `runDeleteRemote(ctx, cli, idStr, out)`.
- `internal/cli/publish_remote.go`: `runPublishRemote(ctx, cli, idStr, out)`.
- В `delete.go` и `publish.go` — branching через `runRemote`.

## T-2 Tests
- `delete_remote_test.go`, `publish_remote_test.go` (httptest, 200/401/404).

## T-3 Wrap-up
- `task test && task build` GREEN.
- CHANGELOG секция F5d.
