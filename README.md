# jtpost

CLI-редактор постов для управления контент-пайплайном (Telegram).

## Описание

**jtpost** — утилита командной строки для управления жизненным циклом постов: от идеи до публикации в Telegram-канале.

### Возможности

- ✅ Создание постов с frontmatter (YAML + Markdown)
- ✅ Управление статусами: `idea` → `draft` → `ready` → `scheduled` → `published`
- ✅ Публикация в Telegram
- ✅ Фильтрация и поиск постов
- ✅ Планирование публикаций

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

Скачайте готовый бинарник со страницы [Releases](https://github.com/jtprogru/jtpost/releases).

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

## Команды CLI

| Команда | Описание |
|---------|----------|
| `jtpost init` | Инициализация проекта (создание `.jtpost.yaml`) |
| `jtpost new <title>` | Создание нового поста |
| `jtpost list` | Список всех постов |
| `jtpost show <id>` | Показать детали поста |
| `jtpost status <id> --set <status>` | Сменить статус поста |
| `jtpost edit <id>` | Редактировать пост в `$EDITOR` |
| `jtpost publish <id> --to <platform>` | Опубликовать на платформу |
| `jtpost plan` | Показать план публикаций |
| `jtpost --help` | Показать справку |

## Формат поста

Посты хранятся в формате Markdown с YAML frontmatter:

```yaml
---
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

# Telegram настройки
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  chat_id: "${TELEGRAM_CHAT_ID}"

# Настройки по умолчанию
defaults:
  status: draft
  platforms: ["telegram"]
```

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
│   ├── adapters/         # Реализации (FS, Telegram, Blog)
│   └── cli/              # Cobra команды
├── templates/            # Шаблоны постов
├── testdata/             # Тестовые данные
└── .jtpost.yaml          # Конфигурация проекта
```

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

## Ссылки

- [ROADMAP.md](./ROADMAP.md) — план развития
- [AGENTS.md](./AGENTS.md) — руководство для разработчиков
- [QWEN.md](./QWEN.md) — контекст для AI-ассистентов
