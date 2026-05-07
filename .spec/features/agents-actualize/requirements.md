# agents-actualize — актуализация AGENTS.md и README

## Goal
Закрыть пункт "Актуализация README.md, AGENTS.md, QWEN.md" из ROADMAP §380.

## Scope
- AGENTS.md — полный rewrite. Старая версия фиксировала состояние на этапе 6;
  отсутствовали B.1-B.5, RBAC, Web UI v2, audit, outbox, OpenAPI, history/diff/revert.
- README.md — обновить блок Возможности и таблицу Статистика проекта (4 storage,
  20+ endpoints, RBAC, audit, worker, history).
- README.en.md — синхронно (уже частично актуализирован при переводе).
- Удалить ссылки на несуществующий QWEN.md.

## Out of scope
- Переписывание detail-документов в `docs/*.md`.
- AGENTS.md тренинг для других моделей (Cursor/Copilot rules, etc.).
