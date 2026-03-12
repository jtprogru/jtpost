# jtpost

CLI-редактор постов для управления контент-пайплайном (Telegram).

[![Go Version](https://img.shields.io/badge/Go-1.25.5+-00ADD8?style=flat&logo=go)](https://go.dev)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/jtprogru/jtpost)](https://goreportcard.com/report/github.com/jtprogru/jtpost)
[![CI](https://github.com/jtprogru/jtpost/actions/workflows/ci.yml/badge.svg)](https://github.com/jtprogru/jtpost/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/jtprogru/jtpost)](https://github.com/jtprogru/jtpost/releases)

**Версия:** 0.2.0 | **Статус:** Активная разработка

---

## Описание

**jtpost** — утилита командной строки для управления жизненным циклом постов: от идеи до публикации в Telegram-канале.

### Возможности

- ✅ Создание постов с frontmatter (YAML + Markdown)
- ✅ Управление статусами: `idea` → `draft` → `ready` → `scheduled` → `published`
- ✅ Публикация в Telegram
- ✅ Фильтрация и поиск постов
- ✅ Планирование публикаций
- ✅ Импорт существующих Markdown-файлов
- ✅ SQLite хранилище для метаданных
- ✅ Миграция между хранилищами (FS ↔ SQLite)
- ✅ HTTP API с Web UI (htmx + Bootstrap)
- ✅ Логирование и middleware

## Установка

### Из исходников

```bash
git clone https://github.com/jtprogru/jtpost.git
cd jtpost
go install ./cmd/jtpost
```

### Через Go Install

```bash
go install github.com/jtprogru/jtpost/cmd/jtpost@latest
```

### Бинарные релизы

Скачайте готовый бинарник со страницы [Releases](https://github.com/jtprogru/jtpost/releases) (версия 0.2.0+).

## Требования

- Go 1.25.5+
- macOS, Linux, Windows

## Быстрый старт

### 1. Инициализация проекта

```bash
jtpost init
```

Создаёт файл `.jtpost.yaml` с конфигурацией по умолчанию.

### 2. Создание нового поста

```bash
jtpost new "Мой первый пост"
```

Создаёт Markdown-файл с frontmatter в директории постов.

### 3. Просмотр списка постов

```bash
jtpost list
jtpost list --status draft
jtpost list --tag golang
```

### 4. Смена статуса

```bash
jtpost status <id> --set ready
```

---

## Примеры использования

### 📝 Полный цикл создания и публикации поста

```bash
# 1. Создать пост с заголовком и тегами
jtpost new "Как оптимизировать Go код" --tag go --tag performance

# 2. Отредактировать содержимое в редакторе
jtpost edit <id>

# 3. Начать работу над постом (idea → draft)
jtpost status <id> --set draft

# 4. Завершить работу (draft → ready)
jtpost status <id> --set ready

# 5. Опубликовать в Telegram
jtpost publish <id> --to telegram

# 6. Пометить как опубликованный
jtpost status <id> --set published
```

### 🔍 Поиск и фильтрация постов

```bash
# Все черновики
jtpost list --status draft

# Посты с тегом "golang"
jtpost list --tag golang

# Комбинация фильтров
jtpost list --status draft --tag go --platform telegram

# Поиск по заголовку
jtpost list --search "оптимизация"

# Вывод в JSON формате
jtpost list --format json
```

### 📅 Планирование контента

```bash
# План публикаций на месяц
jtpost plan

# План на неделю
jtpost plan --days 7

# Получить рекомендацию следующего поста для работы
jtpost next

# Статистика по постам
jtpost stats
```

### 📥 Импорт существующих постов

```bash
# Предпросмотр импорта
jtpost import --dry-run

# Импорт всех Markdown файлов из content/posts/
jtpost import

# Интерактивный режим с подтверждением каждого файла
jtpost import --interactive
```

### 🗄️ Работа с SQLite хранилищем

```bash
# Миграция из файлового хранилища в SQLite
jtpost migrate --to sqlite

# Работа с SQLite через флаг
jtpost list --db .jtpost.db

# Обратная миграция (SQLite → FS)
jtpost migrate --to fs
```

### 🌐 HTTP API и Web UI

```bash
# Запустить сервер на localhost:8080
jtpost serve

# Запустить на другом порту
jtpost serve --port 3000

# Запустить на всех интерфейсах
jtpost serve --addr 0.0.0.0 --port 8080
```

После запуска:

- **Web UI:** <http://localhost:8080>
- **API:** <http://localhost:8080/api/posts>

### 🔧 Примеры скриптов

**Автоматическая публикация по расписанию (cron):**

```bash
# crontab -e
# Публикация запланированных постов каждый час
0 * * * * jtpost list --status scheduled --format json | jq -r '.[].id' | xargs -I {} jtpost publish {} --to telegram
```

**Пакетное создание постов:**

```bash
#!/bin/bash
# create-series.sh
for topic in "basics" "intermediate" "advanced"; do
  jtpost new "Go Guide: $topic" --tag go --tag series
done
```

**Экспорт статистики:**

```bash
# Экспорт статистики в JSON
jtpost stats --format json > stats.json

# Анализ тегов
jtpost list --format json | jq '[.[].tags] | flatten | group_by(.) | map({tag: .[0], count: length})'
```

## Команды CLI

| Команда | Описание |
|---------|----------|
| `jtpost init` | Инициализация проекта (создание `.jtpost.yaml`) |
| `jtpost new <title>` | Создание нового поста |
| `jtpost list` | Список всех постов |
| `jtpost show <id>` | Показать детали поста |
| `jtpost status <id> --set <status>` | Сменить статус поста |
| `jtpost edit <id>` | Редактировать пост в `$EDITOR` |
| `jtpost delete <id>` | Удалить пост |
| `jtpost import` | Импорт постов из `content/posts/` |
| `jtpost migrate` | Миграция из FS в SQLite |
| `jtpost publish <id>` | Опубликовать в Telegram |
| `jtpost plan` | Показать план публикаций |
| `jtpost stats` | Статистика по постам |
| `jtpost next` | Показать следующий пост для публикации |
| `jtpost serve` | Запустить HTTP API сервер |
| `jtpost --help` | Показать справку |

## Формат поста

Посты хранятся в формате Markdown с YAML frontmatter:

```yaml
---
id: "1710234567890-slug-name"
title: "Заголовок поста"
slug: "my-first-post"
status: "draft"
platforms: ["telegram"]
deadline: "2026-02-01"
scheduled_at: "2026-02-03T10:00:00+03:00"
tags: ["golang", "cli"]
external:
  telegram_url: ""
---

Тело поста в формате Markdown...
```

## Конфигурация

Файл `.jtpost.yaml` в корне проекта:

```yaml
# Пути к директориям с постами
posts_dir: content/posts
templates_dir: templates

# SQLite хранилище (опционально)
sqlite:
  dsn: .jtpost.db

# Telegram настройки
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  chat_id: "${TELEGRAM_CHAT_ID}"

# Настройки по умолчанию
defaults:
  status: draft
  platforms: ["telegram"]
```

### Использование SQLite

Для работы с SQLite хранилищем:

```bash
# Миграция из файлового хранилища в SQLite
jtpost migrate

# Работа с SQLite через флаг
jtpost list --db .jtpost.db

# Или через конфигурацию (.jtpost.yaml)
sqlite:
  dsn: .jtpost.db
```

Подробнее: [docs/sqlite.md](docs/sqlite.md)

## Разработка

### Сборка

```bash
task build:bin
# или
go build -o ./dist/jtpost ./cmd/jtpost
```

### Запуск

```bash
task run:cmd
# или
go run ./cmd/jtpost
```

### Тесты

```bash
task test
task test:race
task test:coverage
```

### Линтинг

```bash
task lint
```

## Структура проекта

```
jtpost/
├── cmd/jtpost/           # Точка входа CLI
├── internal/
│   ├── core/             # Доменная модель и интерфейсы
│   ├── adapters/         # Реализации (FS, SQLite, Telegram, HTTP)
│   └── cli/              # Cobra команды
├── templates/            # Шаблоны постов
├── testdata/             # Тестовые данные
├── docs/                 # Документация
├── .jtpost.db            # SQLite БД (опционально)
└── .jtpost.yaml          # Конфигурация проекта
```

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

## Статусы поста

```
idea → draft → ready → scheduled → published
```

- **idea** — черновик идеи, требует проработки
- **draft** — активная работа над постом
- **ready** — готов к публикации
- **scheduled** — запланирован на дату
- **published** — опубликован

## Лицензия

MIT

## Документация

### Основное

- **[ROADMAP.md](./ROADMAP.md)** — план развития проекта (версии 0.2.0, 0.3.0, 0.4.0, 1.0.0)
- **[CHANGELOG.md](./CHANGELOG.md)** — история изменений проекта
- **[CONTRIBUTING.md](./CONTRIBUTING.md)** — руководство для участников проекта
- **[docs/cli.md](./docs/cli.md)** — полное описание CLI команд с примерами
- **[docs/api.md](./docs/api.md)** — документация HTTP API endpoints
- **[docs/architecture.md](./docs/architecture.md)** — архитектура проекта (Hexagonal)
- **[docs/configuration.md](./docs/configuration.md)** — настройка и конфигурация
- **[docs/development.md](./docs/development.md)** — руководство для разработчиков
- **[docs/sqlite.md](./docs/sqlite.md)** — SQLite хранилище и миграция
- **[docs/logging.md](./docs/logging.md)** — логирование и middleware

### Для AI-ассистентов

- **[AGENTS.md](./AGENTS.md)** — руководство для AI-ассистентов
- **[QWEN.md](./QWEN.md)** — контекст проекта для AI

---

## 📚 Дополнительные ресурсы

- **[ROADMAP.md](./ROADMAP.md)** — детальный план развития
- **[Effective Go](https://go.dev/doc/effective_go)** — стиль кода Go
- **[Cobra CLI](https://github.com/spf13/cobra)** — фреймворк для CLI
- **[htmx](https://htmx.org)** — библиотека для Web UI
- **[Telegram Bot API](https://core.telegram.org/bots/api)** — API Telegram
