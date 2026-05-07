# post-revert — возврат поста к предыдущей ревизии

## Goal
Закрыть последний defer этапа 9 (Git Repository хранилище): возможность откатить
title/content/tags поста к состоянию из выбранного git-коммита через Web UI.

## User flow
1. Owner открывает `/ui/posts/{id}/history/{hash}` (revision view, уже существует).
2. На странице ревизии — кнопка «Вернуть к этой версии» с browser-confirm.
3. POST `/ui/posts/{id}/history/{hash}/revert` → handler парсит файл из коммита,
   обновляет current post (Title, Content, Tags) и редиректит на
   `/ui/posts/{id}?reverted=1`.

## Scope
- ✅ Revert ТОЛЬКО Title/Content/Tags. Status, ScheduledAt, Deadline, PublishedAt,
  External — остаются текущими (lifecycle поля, не контент).
- ✅ Audit `post.reverted` с metadata `{hash, via=webui}`.
- ✅ Event `post.updated` (тот же, что и обычный save) — dashboard auto-refresh.
- ✅ Browser-confirm на форме (без extra modal).
- ❌ Side-by-side diff с текущей версией — отдельный cut.
- ❌ Cherry-pick отдельных полей — out of scope.

## Edge cases
- HistoryProvider=nil → 503.
- Post не существует / не в текущем tenant → 404.
- Hash не найден / файл отсутствовал в коммите → 404.
- Парсинг frontmatter провалился (старый формат файла) → 422 с понятным сообщением.
- GET на revert-endpoint → 405.

## Non-functional
- CSRF: `/ui/*` skip-prefix (как все формы UI).
- 0 lint issues, race-clean tests.
