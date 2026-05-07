# Requirements — F5d

## REQ-1 Functional
- 1.1 `jtpost delete --remote URL --auth TOKEN <UUID>` — DELETE /posts/{id}.
- 1.2 `jtpost publish --remote URL --auth TOKEN <UUID>` — POST /posts/{id}/publish.
- 1.3 Без `--remote` существующее поведение неизменно.
- 1.4 Аргумент должен быть UUID (slug-resolution в remote недоступен — same as F5c).

## REQ-2 Errors
- 2.1 401 → `unauthorized: invalid or missing token`.
- 2.2 403 → `forbidden`.
- 2.3 404 → `post not found`.
- 2.4 409 (publish conflict) → `conflict: <message>`.
- 2.5 5xx → `server error: <status>`.

## REQ-3 Tests
- 3.1 httptest mock-сервер: success + 401 + 404 для каждой команды.
