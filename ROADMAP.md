# 🗺️ ROADMAP — План развития проекта jtpost

**Последнее обновление:** 2026-03-12  
**Статус:** Активная разработка  
**Версия:** 0.2.0

---

## 📋 Обзор

**jtpost** — CLI-инструмент для управления контент-пайплайном Telegram-канала.

**Текущее состояние:**
- ✅ 14 CLI команд реализовано
- ✅ HTTP API с Web UI работает
- ✅ SQLite хранилище интегрировано
- ✅ Миграция между хранилищами (FS ↔ SQLite) доступна
- ✅ Импорт постов из Markdown файлов реализован
- ✅ Все тесты проходят (100% PASS)
- ✅ Линтер чист (0 issues)

---

## ✅ Завершённые этапы

### Этап 0: Скелет CLI ✅

**Статус:** 🟢 Завершено  
**Версия:** 0.1.0

**Реализовано:**
- ✅ Инициализация проекта (`jtpost init`)
- ✅ Создание постов (`jtpost new`)
- ✅ Базовая структура Hexagonal Architecture
- ✅ FileSystem репозиторий

---

### Этап 1: Жизненный цикл поста ✅

**Статус:** 🟢 Завершено  
**Версия:** 0.1.0

**Реализовано:**
- ✅ `jtpost list` — список всех постов с фильтрацией
- ✅ `jtpost show <id>` — просмотр деталей поста
- ✅ `jtpost status <id> --set <status>` — смена статуса
- ✅ `jtpost edit <id>` — редактирование в редакторе
- ✅ `jtpost delete <id>` — удаление поста
- ✅ Статусы: `idea` → `draft` → `ready` → `scheduled` → `published`

---

### Этап 2: Интеграция с Telegram ✅

**Статус:** 🟢 Завершено  
**Версия:** 0.2.0

**Реализовано:**
- ✅ `jtpost publish <id> --to telegram` — публикация в Telegram
- ✅ Конвертация Markdown → Telegram HTML/Markdown
- ✅ Сохранение `telegram_url` в frontmatter
- ✅ Telegram адаптер (`internal/adapters/telegram`)
- ✅ Конвертер Markdown (`internal/adapters/telegramconv`)

**Удалено:**
- ✅ Поддержка блога (фокус только на Telegram)
- ✅ PlatformBlog константа
- ✅ Blog publisher

---

### Этап 3: Импорт постов ✅

**Статус:** 🟢 Завершено  
**Версия:** 0.2.0

**Реализовано:**
- ✅ `jtpost import` — импорт из `content/posts/`
- ✅ Парсер YAML frontmatter
- ✅ Нормализация метаданных
- ✅ Флаги `--dry-run`, `--interactive`
- ✅ Отчётность и статистика импорта

---

### Этап 4: Альтернативные хранилища ✅

**Статус:** 🟢 Завершено  
**Версия:** 0.2.0

**Реализовано:**
- ✅ SQLite хранилище (`internal/adapters/sqlite`)
- ✅ `jtpost migrate` — миграция между хранилищами
- ✅ Транзакции и bulk-операции
- ✅ Миграции схемы БД
- ✅ Флаги `--from`, `--to`, `--db`

**Поддерживаемые хранилища:**
- ✅ FileSystem (оригинал)
- ✅ SQLite (через `modernc.org/sqlite`)

---

### Этап 5: Планирование и статистика ✅

**Статус:** 🟢 Завершено  
**Версия:** 0.2.0

**Реализовано:**
- ✅ `jtpost plan` — план публикаций на N дней
- ✅ `jtpost stats` — статистика по постам
- ✅ `jtpost next` — рекомендация следующего поста
- ✅ Фильтрация по дедлайнам и scheduled_at

---

### Этап 6: HTTP API + Web UI ✅

**Статус:** 🟢 Завершено  
**Версия:** 0.2.0

**Реализовано:**
- ✅ `jtpost serve` — встроенный HTTP сервер
- ✅ REST API endpoints:
  - `GET /api/posts` — список постов
  - `GET /api/posts/{id}` — получить пост
  - `PATCH /api/posts/{id}` — обновить пост
  - `DELETE /api/posts/{id}` — удалить пост
  - `POST /api/posts` — создать пост
  - `POST /api/posts/{id}/publish` — опубликовать
  - `GET /api/stats` — статистика
  - `GET /api/plan` — план публикаций
  - `GET /api/next` — рекомендация
- ✅ Web UI на htmx + Bootstrap
- ✅ Логирование запросов (`internal/logger`)
- ✅ Recovery middleware

---

## 🔴 Запланированные этапы

### Этап 7: Улучшение Telegram Publisher (отложено) 🔴

**Статус:** 🔴 Отложено (не актуально)  
**Приоритет:** Низкий  
**Оценка:** 1-2 недели (если потребуется в будущем)

**Возможные улучшения (на будущее):**
- [ ] Поддержка форматирования Telegram (HTML + MarkdownV2) — уже реализовано
- [ ] Авто-добавление кнопок/ссылок к постам
- [ ] Поддержка мультимедиа (фото, видео, документы)
- [ ] Rate limiting для пакетной публикации
- [ ] Обработка ошибок API Telegram
- [ ] Логирование отправленных сообщений

**Примечание:** Текущая реализация Telegram Publisher полностью функциональна для публикации текстовых постов. Дополнительные улучшения не требуются на текущем этапе.

---

### Этап 8: Git Repository хранилище 🔴

**Статус:** 🔴 Запланировано  
**Приоритет:** Средний  
**Оценка:** 2-3 недели

**Задачи:**
- [ ] GitRepository адаптер (`internal/adapters/gitrepo`)
- [ ] Авто-коммиты при изменении постов
- [ ] Синхронизация с удалённым репо
- [ ] Разрешение конфликтов слияния
- [ ] История изменений постов
- [ ] Откат к предыдущим версиям

**Конфигурация:**
```yaml
storage:
  type: "git"
  path: ".jtpost"
  remote: "git@github.com:user/jtpost-data.git"
  branch: "main"
  auto_commit: true
  auto_push: true
  commit_template: "chore: update post {{.Slug}}"
```

---

### Этап 9: PostgreSQL хранилище 🔴

**Статус:** 🔴 Запланировано  
**Приоритет:** Низкий  
**Оценка:** 1-2 недели

**Задачи:**
- [ ] PostgresRepository адаптер (`internal/adapters/postgres`)
- [ ] Миграции схемы БД
- [ ] Connection pooling
- [ ] Поддержка транзакций
- [ ] Команда `jtpost migrate --to postgres`

**Конфигурация:**
```yaml
storage:
  type: "postgres"
  dsn: "postgres://user:pass@host:5432/db?sslmode=disable"
  max_open_conns: 10
  max_idle_conns: 5
```

---

### Этап 10: Улучшение Web UI 🔴

**Статус:** 🔴 Запланировано  
**Приоритет:** Средний  
**Оценка:** 1-2 недели

**Задачи:**
- [ ] Редактор Markdown с предпросмотром
- [ ] Drag-and-drop загрузка изображений
- [ ] Календарь публикаций
- [ ] Тёмная тема
- [ ] PWA (Progressive Web App)
- [ ] WebSocket для real-time обновлений

---

### Этап 11: CI/CD и автоматизация ✅

**Статус:** 🟢 Завершено  
**Приоритет:** Высокий  
**Оценка:** 3-5 дней

**Реализовано:**
- ✅ GitHub Actions workflow (`.github/workflows/ci.yml`)
  - ✅ Запуск тестов на 3 платформах (Ubuntu, macOS, Windows)
  - ✅ Запуск тестов на 2 версиях Go (1.25, 1.26)
  - ✅ Линтинг через golangci-lint-action
  - ✅ Сборка бинарников
  - ✅ Загрузка coverage отчётов на Codecov
  - ✅ Security check через gosec
- ✅ Release workflow (`.github/workflows/release.yml`)
  - ✅ Авто-релизы через GoReleaser
  - ✅ Публикация тегов и создание релизов на GitHub
- ✅ Шаблоны для Issues (bug_report.md, feature_request.md)
- ✅ Шаблон для Pull Request (pull_request_template.md)
- ✅ Руководство для участников (CONTRIBUTING.md)

**Интеграции:**
- 🔴 Codecov — покрытие тестами (требуется настроить токен)
- ✅ GitHub Releases — авто-публикация релизов
- ✅ GitHub Actions — CI/CD пайплайн

---

### Этап 12: Документация и примеры 🔴

**Статус:** 🔴 Запланировано  
**Приоритет:** Средний  
**Оценка:** 1 неделя

**Задачи:**
- [ ] Примеры использования в `examples/`
- [ ] Видео-туториалы
- [ ] API документация (OpenAPI/Swagger)
- [ ] Генерация документации CLI (`jtpost docs`)
- [ ] Перевод документации на английский

---

## 📅 Календарный план

### Март 2026 — Версия 0.2.0 ✅

**Неделя 1-2:**
- ✅ Исправление всех предупреждений golangci-lint (25 → 0)
- ✅ Рефакторинг кода (переименования, modernize)
- ✅ Финальное тестирование и линтинг

**Результат:** Стабильная версия 0.2.0 с чистым кодом

---

### Апрель 2026 — Версия 0.3.0 🔴

**Неделя 1-2:**
- 🔴 Улучшение Telegram Publisher (мультимедиа, кнопки)
- 🔴 Покрытие тестами > 85%

**Неделя 3-4:**
- 🔴 Git Repository хранилище
- 🔴 CI/CD настройка

**Результат:** Версия 0.3.0 с улучшенной публикацией и Git-хранилищем

---

### Май 2026 — Версия 0.4.0 🔴

**Неделя 1-2:**
- 🔴 PostgreSQL хранилище
- 🔴 Улучшение Web UI (календарь, редактор)

**Неделя 3-4:**
- 🔴 Интеграционные тесты
- 🔴 Документация и примеры

**Результат:** Версия 0.4.0 с PostgreSQL и улучшенным UI

---

## 🔧 Технические улучшения

### Постоянные задачи

- [ ] **Документация:** Актуализация README.md, AGENTS.md, QWEN.md
- [ ] **Тесты:** Покрытие > 85% для всех компонентов
- [ ] **Линтинг:** `golangci-lint` без ошибок
- [ ] **Сборка:** Оптимизация времени сборки
- [ ] **Производительность:** Benchmark тесты для критичных участков

### Задачи по рефакторингу

- [ ] Выделить общую логику из адаптеров в утилиты
- [ ] Унифицировать обработку ошибок во всех адаптерах
- [ ] Добавить структурированное логирование
- [ ] Оптимизировать работу с памятью при bulk-операциях
- [ ] Рассмотреть использование interface injection для тестов

---

## 📁 Текущая структура проекта

```
jtpost/
├── cmd/jtpost/main.go              # Точка входа CLI
├── internal/
│   ├── core/                       # Доменная модель
│   │   ├── post.go                 # Тип Post, PostID
│   │   ├── status.go               # PostStatus, Platform
│   │   ├── repository.go           # Интерфейс PostRepository
│   │   ├── service.go              # PostService
│   │   └── errors.go               # Доменные ошибки
│   ├── adapters/                   # Реализации портов
│   │   ├── config/                 # Конфигурация
│   │   ├── fsrepo/                 # FileSystem Repository
│   │   ├── sqlite/                 # SQLite Repository
│   │   ├── httpapi/                # HTTP API + Web UI
│   │   ├── telegram/               # Telegram Publisher
│   │   └── telegramconv/           # Markdown конвертер
│   ├── cli/                        # Cobra команды
│   │   ├── init.go                 # jtpost init
│   │   ├── new.go                  # jtpost new
│   │   ├── list.go                 # jtpost list
│   │   ├── show.go                 # jtpost show
│   │   ├── status.go               # jtpost status
│   │   ├── edit.go                 # jtpost edit
│   │   ├── delete.go               # jtpost delete
│   │   ├── import.go               # jtpost import
│   │   ├── migrate.go              # jtpost migrate
│   │   ├── publish.go              # jtpost publish
│   │   ├── plan.go                 # jtpost plan
│   │   ├── stats.go                # jtpost stats
│   │   ├── next.go                 # jtpost next
│   │   └── serve.go                # jtpost serve
│   └── logger/                     # Логгер
├── templates/                      # Шаблоны постов
├── content/posts/                  # Посты
├── testdata/                       # Тестовые данные
├── docs/                           # Документация
├── .jtpost.example.yaml            # Пример конфига
├── .golangci.yaml                  # Конфиг линтера
├── .goreleaser.yaml                # Конфиг релизов
├── Taskfile.yml                    # Задачи сборки
└── go.mod                          # Зависимости
```

---

## ✅ Метрики успеха

### Качество кода

- ✅ `golangci-lint` без ошибок (0 issues)
- ✅ Все тесты проходят (100% PASS)
- 🔴 Покрытие тестами > 85% (сейчас ~80%)
- 🔴 Benchmark тесты для основных функций

### Функциональность

- ✅ 14 CLI команд работают корректно
- ✅ HTTP API доступно и задокументировано
- ✅ Web UI функционально
- ✅ Миграция между хранилищами работает
- 🔴 Telegram Publisher поддерживает мультимедиа

### Документация

- ✅ README.md актуален
- ✅ ROADMAP.md ведётся
- ✅ AGENTS.md и QWEN.md для AI-ассистентов
- 🔴 Примеры использования в `examples/`
- 🔴 API документация (OpenAPI)

---

## 📊 Статистика проекта

| Метрика | Значение |
|---------|----------|
| **CLI команд** | 14 |
| **HTTP API endpoints** | 9 |
| **Хранилища** | 2 (FS, SQLite) |
| **Платформы** | 1 (Telegram) |
| **Статусы поста** | 5 |
| **Тесты** | 100% PASS |
| **Линтер** | 0 issues |
| **Go версия** | 1.25.5+ |
| **Зависимости** | 20+ |

---

## 📚 Полезные ссылки

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Database/SQL](https://go.dev/wiki/SQL)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- [Cobra CLI](https://github.com/spf13/cobra)
- [Hexagonal Architecture](https://en.wikipedia.org/wiki/Hexagonal_architecture_(software))
- [Telegram Bot API](https://core.telegram.org/bots/api)
- [htmx](https://htmx.org)

---

## 📝 Changelog

### 2026-03-12 — Версия 0.2.0

**Добавлено:**
- ✅ Команда `jtpost import` для импорта Markdown файлов
- ✅ Команда `jtpost migrate` для миграции между хранилищами
- ✅ SQLite хранилище с транзакциями
- ✅ HTTP API endpoint `/api/next` (удалён в 0.2.1)
- ✅ Логгер с уровнями DEBUG/INFO/WARN/ERROR
- ✅ Middleware LoggingMiddleware и RecoveryMiddleware

**Изменено:**
- ✅ Удалён функционал рекомендаций (endpoint `/api/next`)
- ✅ Удалены упоминания блога (фокус на Telegram)
- ✅ Переименован тип `SQLitePostRepository` → `PostRepository`
- ✅ Заменён `interface{}` на `any` во всех файлах

**Исправлено:**
- ✅ Все предупреждения golangci-lint (25 → 0)
- ✅ errcheck, errorlint, noctx, usetesting линтеры

**Документация:**
- ✅ Обновлён ROADMAP.md
- ✅ Обновлены CLI docs (docs/cli.md)
- ✅ Добавлена документация по SQLite (docs/sqlite.md)

### 2026-03-11 — Версия 0.1.0

**Добавлено:**
- ✅ 14 CLI команд (init, new, list, show, status, edit, delete, import, migrate, publish, plan, stats, next, serve)
- ✅ HTTP API с Web UI
- ✅ FileSystem репозиторий
- ✅ Telegram Publisher
- ✅ Markdown конвертер

**Изменено:**
- ✅ Удалена поддержка блога
- ✅ Фокус на Telegram

---

## 🎯 Долгосрочные цели (2026)

### Версия 1.0.0 (конец 2026)

**Цели:**
- [ ] Стабильный API без breaking changes
- [ ] Полное покрытие тестами (>90%)
- [ ] Поддержка 3+ хранилищ (FS, SQLite, Git)
- [ ] Продвинутый Telegram Publisher
- [ ] Web UI с календарём и редактором
- [ ] CI/CD пайплайн
- [ ] Документация на 2 языках (RU/EN)

### Версия 2.0.0 (2027)

**Цели:**
- [ ] Мультипользовательский режим
- [ ] Ролевая модель (admin, editor, viewer)
- [ ] Интеграция с другими платформами (VK, Twitter)
- [ ] Плагины для расширения функционала
- [ ] Desktop приложение (Tauri/Flutter)

---

**Контакты:**  
GitHub: [@jtprogru](https://github.com/jtprogru/jtpost)  
Лицензия: MIT
