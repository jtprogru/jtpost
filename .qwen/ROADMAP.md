# 🗺️ ROADMAP — План развития проекта jtpost

**Последнее обновление:** 2026-03-12
**Статус:** Активная разработка

---

## 📋 Обзор

**jtpost** — CLI-инструмент для управления контент-пайплайном Telegram-канала.

**Текущий фокус:**
1. ✅ Удалён функционал рекомендаций (endpoint `/api/next`)
2. ✅ Импорт постов из существующих Markdown-файлов
3. ✅ Удаление упоминаний блога (фокус на Telegram)
4. ✅ Поддержка альтернативных хранилищ данных (SQLite реализовано)

---

## 🎯 Этап 1: Импорт постов из `content/posts/`

**Статус:** 🟢 Завершено
**Приоритет:** Высокий
**Оценка:** 1-2 недели

### Цель

Автоматизировать импорт существующих Markdown-файлов с приведением к единому стандарту frontmatter.

### Задачи

- [ ] **Создать парсер frontmatter** (`internal/adapters/fsrepo/frontmatter_parser.go`)
  - [ ] Детектор формата (YAML / TOML / отсутствует)
  - [ ] Парсинг YAML frontmatter
  - [ ] Парсинг TOML frontmatter (опционально)
  - [ ] Обработка файлов без frontmatter
  - [ ] Тесты на различные форматы

- [ ] **Создать команду CLI `post import`** (`internal/cli/import.go`)
  - [ ] Сканирование директории `content/posts/`
  - [ ] Рекурсивный обход поддиректорий
  - [ ] Фильтрация `.md` файлов
  - [ ] Пропуск уже импортированных файлов (с `status` в frontmatter)

- [ ] **Логика нормализации frontmatter**
  - [ ] Если frontmatter есть → обновить до стандарта jtpost
  - [ ] Если frontmatter нет → добавить стандартный шаблон
  - [ ] Генерация `slug` из имени файла
  - [ ] Присвоение статуса `draft` по умолчанию
  - [ ] Сохранение существующих тегов (если есть)

- [ ] **Интерактивный режим и отчётность**
  - [ ] Флаг `--dry-run` для предпросмотра изменений
  - [ ] Флаг `--interactive` для пошагового подтверждения
  - [ ] Флаг `--output` для вывода отчёта в файл
  - [ ] Вывод статистики: импортировано / обновлено / пропущено / ошибок

- [ ] **Документация**
  - [ ] Обновить README.md с описанием команды `post import`
  - [ ] Примеры использования в docs/

### Стандарт frontmatter jtpost

```yaml
---
id: "1710234567890-slug-name"  # Генерируется при импорте
title: "Заголовок поста"
slug: "slug-name"
status: "draft"                 # idea | draft | ready | scheduled | published
platforms:
  - "telegram"
deadline: "2026-02-01"          # Опционально
scheduled_at: "2026-02-03T10:00:00+03:00"  # Опционально
tags: ["golang", "cli"]
external:
  telegram_url: ""              # Заполняется при публикации
---
```

### Команды CLI

```bash
# Предпросмотр импорта
jtpost import --dry-run

# Импорт всех файлов из content/posts/
jtpost import

# Интерактивный режим
jtpost import --interactive

# Импорт с выводом отчёта
jtpost import --output import-report.json
```

---

## 🎯 Этап 2: Удаление упоминаний блога (фокус на Telegram)

**Статус:** 🟢 Завершено
**Приоритет:** Высокий
**Оценка:** 3-5 дней

### Цель

Убрать поддержку блога из UI и API, оставить только Telegram.

### Задачи

- [ ] **Обновить доменные типы** (`internal/core/status.go`)
  - [ ] Удалить константу `PlatformBlog`
  - [ ] Оставить только `PlatformTelegram`
  - [ ] Обновить валидацию платформ

- [ ] **Обновить HTTP API** (`internal/adapters/httpapi/server.go`)
  - [ ] Удалить endpoint `/api/platforms` (или вернуть только `["telegram"]`)
  - [ ] Удалить поле `BlogURL` из `ExternalLinks`
  - [ ] Обновить обработчики ошибок

- [ ] **Обновить Web UI** (`internal/adapters/httpapi/templates/index.html`)
  - [ ] Удалить фильтр "Blog" из dropdown платформ
  - [ ] Удалить чекбокс "Blog" из модальных окон
  - [ ] Удалить колонку "Платформы" из таблицы (или оставить только Telegram)
  - [ ] Обновить заголовки и тексты интерфейса

- [ ] **Обновить CLI команды** (`internal/cli/publish.go`)
  - [ ] Удалить флаг `--to blog`
  - [ ] Удалить логику публикации в блог
  - [ ] Оставить только `--to telegram`

- [ ] **Обновить конфигурацию**
  - [ ] Обновить `.jtpost.example.yaml`
  - [ ] Удалить секции, связанные с блогом
  - [ ] Оставить только Telegram-конфигурацию

- [ ] **Обновить документацию**
  - [ ] README.md
  - [ ] AGENTS.md
  - [ ] QWEN.md

### Итоговая конфигурация

```yaml
# .jtpost.yaml
telegram:
  token: "BOT_TOKEN"
  chat_id: "@channel"
  
storage:
  type: "filesystem"
  path: "content/posts"
```

---

## 🎯 Этап 3: Поддержка альтернативных хранилищ данных

**Статус:** 🟢 Завершено
**Приоритет:** Высокий
**Оценка:** 2-3 недели (SQLite), 2-3 недели каждое доп. хранилище

### Цель

Добавить возможность выбора хранилища метаданных постов: файловая система, SQLite, PostgreSQL, MongoDB, Git.

### Архитектурные изменения

#### 3.1. Расширить интерфейс `PostRepository`

**Файл:** `internal/core/repository.go`

```go
type PostRepository interface {
    // Существующие методы
    Create(ctx context.Context, post *Post) error
    GetByID(ctx context.Context, id PostID) (*Post, error)
    List(ctx context.Context, filter PostFilter) ([]*Post, error)
    Update(ctx context.Context, post *Post) error
    Delete(ctx context.Context, id PostID) error
    
    // Новые методы для миграции
    BeginTx(ctx context.Context) (Transaction, error)
    BulkCreate(ctx context.Context, posts []*Post) error
    MigrateTo(ctx context.Context, to PostRepository) error
}
```

#### 3.2. Интерфейс транзакции

**Файл:** `internal/core/transaction.go` (новый)

```go
type Transaction interface {
    Commit() error
    Rollback() error
    Repository() PostRepository
}
```

### Варианты реализации

#### Вариант A: SQLite (рекомендуется для начала)

**Статус:** 🟢 Завершено
**Приоритет:** Высокий

**Файлы:**
- `internal/adapters/sqlite/repository.go`
- `internal/adapters/sqlite/schema.go`
- `internal/adapters/sqlite/migrations.go`

**Плюсы:**
- Простое развёртывание (один файл)
- Поддержка транзакций
- Не требует отдельный сервер
- Отличная производительность для локального использования

**Схема БД:**

```sql
CREATE TABLE posts (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL,
  slug TEXT NOT NULL UNIQUE,
  status TEXT NOT NULL CHECK(status IN ('idea', 'draft', 'ready', 'scheduled', 'published')),
  platforms TEXT NOT NULL,  -- JSON массив
  tags TEXT NOT NULL,       -- JSON массив
  deadline TIMESTAMP,
  scheduled_at TIMESTAMP,
  published_at TIMESTAMP,
  file_path TEXT NOT NULL,  -- Путь к файлу с контентом
  content_hash TEXT,        -- Хеш для детектирования изменений
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_posts_status ON posts(status);
CREATE INDEX idx_posts_deadline ON posts(deadline);
CREATE INDEX idx_posts_scheduled_at ON posts(scheduled_at);
```

**Зависимости:**
```go
import "modernc.org/sqlite"  // Pure Go SQLite driver
```

**Конфигурация:**
```yaml
storage:
  type: "sqlite"
  path: ".jtpost.db"
```

---

#### Вариант B: PostgreSQL

**Статус:** 🔴 Запланировано  
**Приоритет:** Средний

**Файлы:**
- `internal/adapters/postgres/repository.go`
- `internal/adapters/postgres/schema.go`
- `internal/adapters/postgres/migrations.go`

**Плюсы:**
- Полноценная клиент-серверная БД
- Поддержка распределённой работы
- Мощные возможности (триггеры, представления)

**Минусы:**
- Требует отдельный сервер БД
- Сложнее в развёртывании

**Зависимости:**
```go
import "github.com/jackc/pgx/v5"
```

**Конфигурация:**
```yaml
storage:
  type: "postgres"
  dsn: "postgres://user:pass@host:5432/db?sslmode=disable"
```

---

#### Вариант C: Git Repository (remote)

**Статус:** 🔴 Запланировано  
**Приоритет:** Средний

**Файлы:**
- `internal/adapters/gitrepo/repository.go`
- `internal/adapters/gitrepo/sync.go`

**Плюсы:**
- Версионирование всех изменений
- Синхронизация между устройствами
- Резервное копирование в удалённом репо

**Минусы:**
- Сложность реализации (конфликты слияния)
- Требует настройки SSH/GPG ключей
- Медленнее локальных хранилищ

**Зависимости:**
```go
import "github.com/go-git/go-git/v5"
```

**Конфигурация:**
```yaml
storage:
  type: "git"
  path: ".jtpost"
  remote: "git@github.com:user/jtpost-data.git"
  branch: "main"
  auto_commit: true
  auto_push: true
```

---

#### Вариант D: MongoDB (опционально)

**Статус:** 🔴 Запланировано  
**Приоритет:** Низкий

**Файлы:**
- `internal/adapters/mongodb/repository.go`

**Плюсы:**
- Гибкая схема документов
- JSON-подобное хранение

**Минусы:**
- Избыточно для данной задачи
- Требует отдельный сервер

---

### Команда миграции

**Файл:** `internal/cli/migrate.go`

```bash
# Миграция в SQLite
jtpost migrate --to sqlite

# Миграция в PostgreSQL
jtpost migrate --to postgres --dsn "postgres://..."

# Миграция в Git
jtpost migrate --to git --remote "git@github.com:..."

# Обратная миграция (из SQLite в filesystem)
jtpost migrate --to filesystem

# Проверка целостности после миграции
jtpost migrate --verify
```

**Логика:**
1. Создать новое хранилище
2. Прочитать все посты из текущего хранилища
3. Записать в новое хранилище
4. Верифицировать данные (сравнить количество и хеши)
5. Обновить `.jtpost.yaml` с новым типом хранилища
6. Создать бэкап старого хранилища

---

## 📅 Общий план работ

### Спринт 1: Импорт постов ✅

**Статус:** Завершено

**Неделя 1:**
- ✅ Парсер frontmatter (детектор, YAML, TOML)
- ✅ Тесты на парсер
- ✅ Логика нормализации

**Неделя 2:**
- ✅ Команда `post import`
- ✅ Флаги `--dry-run`, `--interactive`
- ✅ Отчётность и статистика
- ✅ Документация

**Критерии приёмки:**
- ✅ Команда `post import` успешно обрабатывает 100% тестовых файлов
- ✅ Корректное определение формата frontmatter
- ✅ Сохранение существующих данных при нормализации
- ✅ Покрытие тестами > 80%

---

### Спринт 2: Удаление блога ✅

**Статус:** Завершено

**День 1-2:**
- ✅ Обновление доменных типов
- ✅ Обновление HTTP API

**День 3-4:**
- ✅ Обновление Web UI
- ✅ Обновление CLI команд

**День 5:**
- ✅ Обновление документации
- ✅ Тесты и линтинг

**Критерии приёмки:**
- ✅ В коде нет упоминаний `PlatformBlog`
- ✅ В UI нет фильтров/чекбоксов блога
- ✅ Все тесты проходят
- ✅ `golangci-lint` без ошибок

---

### Спринт 3: SQLite ✅

**Статус:** Завершено

**Неделя 1:**
- ✅ Интерфейс `PostRepository` (расширение)
- ✅ Интерфейс `Transaction`
- ✅ Схема БД и миграции

**Неделя 2:**
- ✅ Реализация `SQLiteRepository`
- ✅ Поддержка транзакций
- ✅ Тесты на репозиторий

**Неделя 3:**
- ✅ Команда `post migrate`
- ✅ Интеграция с сервисным слоем
- ✅ Документация

**Критерии приёмки:**
- ✅ Данные сохраняются в `.jtpost.db`
- ✅ Команда `post list` работает с SQLite
- ✅ Миграция из FS в SQLite и обратно
- ✅ Покрытие тестами > 80%

---

### Спринт 4+: Дополнительные хранилища (по желанию)

**PostgreSQL:** 1-2 недели  
**Git Repository:** 2-3 недели  
**MongoDB:** 1 неделя (опционально)

---

## 🔧 Технические улучшения

### Постоянные задачи

- [ ] **Документация:** Актуализация README.md, AGENTS.md, QWEN.md
- [ ] **Тесты:** Покрытие > 80% для всех новых компонентов
- [ ] **Линтинг:** `golangci-lint` без ошибок
- [ ] **Сборка:** Обновление Taskfile.yml новыми задачами

### Задачи по рефакторингу

- [ ] Выделить общую логику из `fsrepo` в утилиты
- [ ] Унифицировать обработку ошибок во всех адаптерах
- [ ] Добавить логирование в критических точках
- [ ] Оптимизировать работу с памятью при bulk-операциях

---

## 📁 Целевая структура проекта

```
jtpost/
├── cmd/jtpost/main.go
├── internal/
│   ├── core/
│   │   ├── post.go
│   │   ├── status.go              # Только PlatformTelegram
│   │   ├── repository.go          # Расширенный интерфейс
│   │   ├── transaction.go         # Новый интерфейс
│   │   ├── service.go
│   │   └── errors.go
│   ├── adapters/
│   │   ├── fsrepo/
│   │   │   ├── repository.go
│   │   │   └── frontmatter_parser.go  # Новый парсер
│   │   ├── sqlite/
│   │   │   ├── repository.go      # Новая реализация
│   │   │   ├── schema.go          # SQL схема
│   │   │   └── migrations.go      # Миграции БД
│   │   ├── postgres/              # Опционально
│   │   │   └── repository.go
│   │   ├── gitrepo/               # Опционально
│   │   │   └── repository.go
│   │   ├── httpapi/
│   │   │   ├── server.go          # Без blog endpoint'ов
│   │   │   └── templates/index.html  # Без blog UI
│   │   └── telegram/
│   │       └── publisher.go
│   └── cli/
│       ├── root.go
│       ├── init.go
│       ├── new.go
│       ├── import.go              # Новая команда
│       ├── migrate.go             # Новая команда
│       ├── list.go
│       ├── show.go
│       ├── status.go
│       ├── edit.go
│       ├── delete.go
│       ├── publish.go             # Только Telegram
│       ├── plan.go
│       ├── stats.go
│       ├── serve.go
│       └── next.go                # Удалено
├── content/
│   └── posts/                     # Исходные MD файлы
├── .jtpost.db                     # SQLite БД (опционально)
├── .jtpost.yaml                   # Конфигурация
└── docs/
    └── import.md                  # Документация по импорту
```

---

## ✅ Метрики успеха

### Импорт постов ✅
- ✅ 100% файлов в `content/posts/` импортированы
- ✅ Все файлы имеют валидный frontmatter
- ✅ Нет потерь данных при импорте

### Telegram-only ✅
- ✅ В коде нет упоминаний блога
- ✅ UI очищен от blog-элементов
- ✅ Публикация работает только в Telegram

### Хранилища ✅
- ✅ SQLite работает стабильно
- ✅ Миграция между хранилищами без потерь
- ✅ Производительность не ухудшилась

### Качество кода
- [ ] Покрытие тестами > 80%
- [ ] `golangci-lint` без ошибок
- [ ] Документация актуальна

---

## 📚 Полезные ссылки

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Database/SQL](https://go.dev/wiki/SQL)
- [modernc.org/sqlite](https://pkg.go.dev/modernc.org/sqlite)
- [Cobra CLI](https://github.com/spf13/cobra)
- [Hexagonal Architecture](https://en.wikipedia.org/wiki/Hexagonal_architecture_(software))

---

## 📝 Changelog

### 2026-03-12 — Версия 0.2.0
- ✅ Завершён Этап 1: Импорт постов из `content/posts/`
- ✅ Завершён Этап 2: Удаление упоминаний блога (фокус на Telegram)
- ✅ Завершён Этап 3: Поддержка SQLite хранилища
  - ✅ Создан `SQLitePostRepository` с миграциями, CRUD, транзакциями
  - ✅ Создана команда CLI `jtpost migrate`
  - ✅ Написано 10 юнит-тестов для SQLite (100% PASS)
  - ✅ Написана документация (`docs/sqlite.md`)
- ✅ Обновлена документация (ROADMAP, README, CLI docs)

### 2026-03-12
- ✅ Удалён endpoint `/api/next` из HTTP API
- ✅ Удалены UI компоненты рекомендаций
- ✅ Создан данный ROADMAP с планом развития

### Предыдущие изменения
- ✅ Инициализация проекта
- ✅ Добавлен HTTP API с Web UI
- ✅ Добавлены CLI команды (list, show, status, edit, delete, publish, plan, stats, serve)
