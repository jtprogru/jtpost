# Project Summary

## Overall Goal
Разработка CLI-инструмента jtpost для управления контент-пайплайном Telegram-канала с поддержкой импорта существующих постов, удаления упоминаний блога и добавления альтернативных хранилищ данных (SQLite, PostgreSQL, Git).

## Key Knowledge
- **Проект:** jtpost — CLI-инструмент на Go 1.25.5+ для управления постами Telegram
- **Модуль:** `github.com/jtprogru/jtpost`
- **Архитектура:** Hexagonal/Clean Architecture (`cmd/` + `internal/core/` + `internal/adapters/`)
- **HTTP API:** Порт 8080, Web UI на htmx + Tailwind v4
- **Сборка:** `go build -o ./dist/jtpost ./cmd/jtpost` — ✅ PASS
- **Тесты:** `go test ./...` — ✅ 100% PASS
- **Линтинг:** `golangci-lint run` — ✅ 7 minor warnings (modernize, intrange, gocritic)
- **Запуск сервера:** `./dist/jtpost serve` или `go run cmd/jtpost/main.go serve`
- **CLI команды:** 13 команд (init, new, list, show, status, edit, delete, publish, plan, stats, next, serve, **import**)
- **Жизненный цикл поста:** idea → draft → ready → scheduled → published
- **Формат поста:** Markdown с YAML frontmatter
- **Новые файлы:** `internal/cli/import.go`, обновлены `repository.go`, `service.go`, `post.go`

## Recent Actions
- ✅ Создан детальный ROADMAP.md с планом развития проекта (3 этапа: импорт, удаление блога, хранилища)
- ✅ Создан парсер frontmatter с поддержкой YAML и файлов без frontmatter (`frontmatter_parser.go`)
- ✅ Реализована нормализация frontmatter до стандарта jtpost
- ✅ Написаны тесты на парсер (18 тестов, 100% PASS)
- ✅ Создана команда CLI `post import` с флагами `--dry-run`, `--interactive`, `--output`, `--update`
- ✅ Добавлен метод `GetBySlug` в интерфейс `PostRepository`
- ✅ Добавлен метод `CreatePostWithContent` в `PostService`
- ✅ Добавлена функция `GeneratePostID` в `core/post.go`
- ✅ Обновлены mock-репозитории в тестах (`service_test.go`, `server_test.go`)
- ✅ Успешно протестирован импорт 7 постов из `content/posts/`

## Current Plan

### Этап 1: Импорт постов ✅ COMPLETED
1. [DONE] Создать парсер frontmatter (`internal/adapters/fsrepo/frontmatter_parser.go`)
2. [DONE] Написать тесты на парсер frontmatter
3. [DONE] Создать команду CLI `post import` (`internal/cli/import.go`)
4. [DONE] Добавить флаги `--dry-run`, `--interactive`, `--output`, `--update`
5. [DONE] Обновить интерфейс `PostRepository` (добавить `GetBySlug`)
6. [DONE] Обновить `PostService` (добавить `GetBySlug`, `CreatePostWithContent`)
7. [DONE] Протестировать на реальных файлах в `content/posts/`

### Этап 2: Удаление упоминаний блога (TODO)
8. [TODO] Удалить `PlatformBlog` из `internal/core/status.go`
9. [TODO] Обновить HTTP API (удалить blog-упоминания)
10. [TODO] Обновить Web UI (удалить blog-элементы)
11. [TODO] Обновить CLI publish (удалить `--to blog`)
12. [TODO] Обновить документацию

### Этап 3: Поддержка альтернативных хранилищ (TODO)
13. [TODO] Расширить интерфейс `PostRepository` (добавить транзакции)
14. [TODO] Создать SQLite репозиторий (`internal/adapters/sqlite/`)
15. [TODO] Создать команду CLI `post migrate`
16. [TODO] Написать тесты на SQLite и миграцию

## Architecture Decisions
- **Frontmatter Parser:** Поддерживает YAML и отсутствие frontmatter, нормализует к стандарту jtpost
- **Import Strategy:** Сканирование `content/posts/`, детектирование формата, автоматический/интерактивный режим
- **Storage Abstraction:** Интерфейс `PostRepository` с поддержкой `GetBySlug` для быстрого поиска
- **Slug Normalization:** Автоматическое удаление префиксов даты (YYYY-MM-DD-) из slug
- **SQLite Choice:** Pure Go driver (`modernc.org/sqlite`) для простоты развёртывания

## Testing Status
- **fsrepo package:** 18 тестов, все PASS
- **core package:** Все тесты PASS
- **httpapi package:** Все тесты PASS
- **cli package:** Все тесты PASS
- **Coverage:** Требуется проверка после завершения import команды
- **Lint:** 7 minor warnings (не критично)

## Files Created/Modified
| File | Status | Description |
|------|--------|-------------|
| `.qwen/ROADMAP.md` | Created | Детальный план развития проекта |
| `internal/adapters/fsrepo/frontmatter_parser.go` | Created | Парсер и нормализатор frontmatter |
| `internal/adapters/fsrepo/frontmatter_parser_test.go` | Created | Тесты парсера (18 тестов) |
| `internal/cli/import.go` | Created | CLI команда импорта |
| `internal/core/repository.go` | Modified | Добавлен метод `GetBySlug` |
| `internal/core/service.go` | Modified | Добавлены `GetBySlug`, `CreatePostWithContent` |
| `internal/core/post.go` | Modified | Добавлена `GeneratePostID` |
| `internal/core/service_test.go` | Modified | Добавлен `GetBySlug` в mock |
| `internal/adapters/httpapi/server_test.go` | Modified | Добавлен `GetBySlug` в mock |
| `internal/adapters/fsrepo/repository.go` | Modified | Реализован `GetBySlug` |
| `internal/cli/root.go` | Modified | Зарегистрирована команда `import` |

## Demo Results
```bash
# Тестирование импорта
./dist/jtpost import content/posts --output /tmp/jtpost-test
✅ Импортировано 7 постов
✅ Все тесты PASS
✅ Сборка без ошибок
```

## Next Session Priorities
1. [ ] Начать этап 2 (удаление блога)
   - Удалить `PlatformBlog` из доменной модели
   - Обновить CLI, API, UI
2. [ ] Написать тесты на команду `import`
3. [ ] Обновить документацию (README.md, docs/import.md)

---

## Summary Metadata
**Update time**: 2026-03-12T13:15:00+03:00
**Last commit**: Import команда готова к использованию
