# Task Plan — F5d2

## T-1 Implement
- `internal/cli/new_remote.go`: `runNewRemote` + `readContentSource` helper.
- `internal/cli/edit_remote.go`: `runEditRemote` (partial update, dirty-check).
- В `new.go` добавить флаг `--content`, branching через runRemote.
- В `edit.go` добавить флаги `--title --content --tag --status`, branching.

## T-2 Tests
- `new_remote_test.go` — 4 теста (success, no-title, 400, 401).
- `edit_remote_test.go` — 6 тестов (success, no-fields, bad-uuid, 404, tags-replace, helper).

## T-3 Wrap-up
- task test/build GREEN.
- CHANGELOG.
