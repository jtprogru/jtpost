# F5d2 — `--remote` для new/edit с передачей контента

## Scope
- `jtpost new --remote URL --auth TOKEN --title T [--slug S] [--tag T...] [--content -|<file>]`
- `jtpost edit --remote URL --auth TOKEN <UUID> [--title T] [--tag T...] [--content -|<file>] [--status STATUS]`

## Existing
- `apiclient.CreatePostWithResponse(ctx, body)` — есть.
- `apiclient.UpdatePostWithResponse(ctx, id, body)` — есть.
- `CreatePostRequest`/`UpdatePostRequest` структуры готовы (Content,Tags,Slug,Title,Status...).
- В local-mode `new` открывает редактор; для remote это бессмысленно — content передаётся явно.

## Decisions
- `--content -` → читать stdin до EOF.
- `--content <path>` → читать файл.
- `--content` пустое в `new` → создаём пост без контента (server применит default).
- Для `edit` — partial update: передаём только указанные поля (в UpdatePostRequest все *T).
- `--status` для edit принимает draft|ready|scheduled|published.
- Editor НЕ запускается в remote-mode (даже если флаг --editor задан → ignored с warning).

## Out of scope
- Аплоад attachments.
- Branching по slug (ID-only, как в F5c/F5d).
