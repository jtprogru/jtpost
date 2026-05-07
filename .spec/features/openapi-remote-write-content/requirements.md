# Requirements — F5d2

## REQ-1 new --remote
- 1.1 Argument `<title>` обязателен (как в local).
- 1.2 `--content -` → stdin; `--content <path>` → файл; иначе пустой content.
- 1.3 `--slug`, `--tag` передаются если non-empty.
- 1.4 `--editor` ignored (warning to stderr).
- 1.5 POST /posts. 201/200 → success: вывести id+slug+status. 400→validation, 401→unauthorized, 403→forbidden.

## REQ-2 edit --remote
- 2.1 Argument `<id>` (UUID) обязателен.
- 2.2 Хотя бы один из `--title/--content/--tag/--status` должен быть задан → иначе error "no fields to update".
- 2.3 `--content` источник как в new.
- 2.4 PATCH /posts/{id} (UpdatePostWithResponse). 200→success, 400/401/403/404/409 mapping.

## REQ-3 Tests
- 3.1 httptest mock для new: success + 401 + 400.
- 3.2 httptest mock для edit: success (partial) + 404 + no-fields.
- 3.3 stdin reader test (через bytes.Reader injection).
