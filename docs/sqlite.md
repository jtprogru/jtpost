# SQLite хранилище для jtpost

## Обзор

Начиная с версии 0.2.0, jtpost поддерживает SQLite в качестве альтернативного хранилища для постов. Это позволяет:

- **Централизованное хранение** — все посты в одной базе данных
- **Быстрый поиск** — индексы по статусам, slug и платформам
- **Транзакции** — атомарные операции импорта/обновления
- **Миграция** — перенос постов из файлового хранилища в SQLite

## Быстрый старт

### 1. Создание базы данных

```bash
# Инициализируйте проект (если ещё не сделан)
jtpost init

# Выполните миграцию из файлового хранилища в SQLite
jtpost migrate
```

### 2. Использование SQLite в CLI

По умолчанию CLI команды используют файловое хранилище. Для работы с SQLite:

```bash
# Указать путь к БД через флаг
jtpost list --db .jtpost.db

# Или через конфигурацию (.jtpost.yaml)
sqlite:
  dsn: /path/to/jtpost.db
```

## Конфигурация

### Файл `.jtpost.yaml`

```yaml
posts_dir: content/posts
templates_dir: templates

# Настройки SQLite
sqlite:
  dsn: .jtpost.db  # путь к файлу БД

# Настройки Telegram
telegram:
  bot_token: "..."
  chat_id: "..."

# Настройки по умолчанию
defaults:
  status: draft
  platforms:
    - telegram
```

### Переменные окружения

```bash
# Путь к базе данных
export JTPOST_SQLITE_DSN="/path/to/jtpost.db"
```

## Команда migrate

### Описание

Мигрирует посты из файлового хранилища в SQLite.

### Флаги

| Флаг | Короткий | Описание | По умолчанию |
|------|----------|----------|--------------|
| `--db` | `-d` | путь к файлу SQLite БД | `.jtpost.db` |
| `--dry-run` | `-n` | режим предпросмотра | `false` |
| `--overwrite` | `-f` | перезаписать существующую БД | `false` |
| `--from` | `-s` | директория с постами для импорта | `posts_dir` из конфига |

### Примеры

```bash
# Предпросмотр миграции
jtpost migrate --dry-run

# Миграция в БД по умолчанию
jtpost migrate

# Миграция в кастомную БД
jtpost migrate --db /tmp/jtpost.db

# Миграция с перезаписью существующей БД
jtpost migrate --overwrite

# Миграция из конкретной директории
jtpost migrate --from /path/to/old/posts
```

## Схема базы данных

### Таблица `posts`

```sql
CREATE TABLE posts (
    id TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    slug TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL,
    platforms TEXT,              -- JSON массив
    tags TEXT,                   -- JSON массив
    deadline TEXT,               -- RFC3339
    scheduled_at TEXT,           -- RFC3339
    published_at TEXT,           -- RFC3339
    content TEXT NOT NULL,
    telegram_url TEXT,
    created_at TEXT NOT NULL,    -- RFC3339
    updated_at TEXT NOT NULL     -- RFC3339
);

CREATE INDEX idx_posts_status ON posts(status);
CREATE INDEX idx_posts_slug ON posts(slug);
CREATE INDEX idx_posts_platforms ON posts(platforms);
```

### Формат JSON полей

**platforms:**
```json
["telegram"]
```

**tags:**
```json
["go", "cli", "telegram"]
```

## Архитектура

### Интерфейсы

SQLite репозиторий реализует следующие интерфейсы из `internal/core`:

- **`PostRepository`** — базовый интерфейс хранения
- **`MigratableRepository`** — поддержка импорта данных
- **`TransactionalRepository`** — поддержка транзакций

### Преимущества перед файловым хранилищем

| Характеристика | Файловое | SQLite |
|----------------|----------|--------|
| Поиск по статусу | Медленный (scan) | Быстрый (индекс) |
| Поиск по тегам | Медленный (scan) | Быстрый (LIKE) |
| Транзакции | Нет | Есть |
| Конкурентный доступ | Ограничен | Есть |
| Размер | Зависит от файлов | Компактный |
| Бэкап | Копирование директории | Один файл |

## Миграция с других СУБД

### PostgreSQL (планируется)

```bash
jtpost migrate --to postgres --from .jtpost.db
```

### MySQL (планируется)

```bash
jtpost migrate --to mysql --from .jtpost.db
```

## Производительность

### Бенчмарки

```bash
# Поиск по статусу (1000 постов)
# Файловое: ~50ms
# SQLite: ~2ms

# Поиск по тегу
# Файловое: ~100ms
# SQLite: ~5ms
```

## Troubleshooting

### Ошибка: "база данных уже содержит посты"

Используйте флаг `--overwrite` для перезаписи:

```bash
jtpost migrate --overwrite
```

### Ошибка: "неверный формат JSON"

Проверьте формат полей `platforms` и `tags` в базе:

```bash
sqlite3 .jtpost.db "SELECT id, platforms, tags FROM posts LIMIT 5;"
```

### Ошибка: "database is locked"

SQLite поддерживает ограниченную конкурентность. Убедитесь, что:
- Нет других процессов, работающих с БД
- Файл БД не открыт в другом приложении

## См. также

- [CLI команды](cli.md) — полное описание команд
- [Конфигурация](configuration.md) — настройки jtpost
- [ROADMAP](../ROADMAP.md) — план развития проекта
