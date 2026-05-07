# F5d — `--remote` для write CLI команд (delete, publish)

## Scope
Добавить `--remote` режим для CLI `delete` и `publish` (id-only, trivial).
Команды `new` и `edit` НЕ входят в скоуп F5d — они требуют редизайна editor-flow
для remote-mode (передача контента через --content/stdin/file). Отложено до F5d2.

## Existing
- `internal/cli/remote.go` — runRemote(cmd, fn) helper готов (F5c).
- `apiclient.DeletePostWithResponse(ctx, id)` — есть.
- `apiclient.PublishPostWithResponse(ctx, id)` — есть.
- HTTP API endpoints `DELETE /posts/{id}`, `POST /posts/{id}/publish` работают (F4).

## Out of scope
- `new --remote`, `edit --remote` — редизайн editor flow.
- Backwards/idempotency tokens (нет в API).
