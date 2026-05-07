# jtpost — examples

Минимальный набор примеров для быстрого старта.

## Файлы

- **`.jtpost.yaml`** — пример конфигурации со sqlite + telegram backend'ом.
  Скопируйте в корень репозитория, заполните `tenant_default` / `author_default`
  своими UUID и установите env-переменные `TELEGRAM_BOT_TOKEN`, `TELEGRAM_CHAT_ID`.
- **`posts/sample-post.md`** — пример поста с полным frontmatter. Демонстрирует
  все основные поля: `title`, `status`, `tags`, `deadline`, `revision`.

## Quickstart

```bash
cp examples/.jtpost.yaml ./.jtpost.yaml
cp examples/posts/sample-post.md ./posts/sample-post.md

# Прогоните доктора, чтобы убедиться что конфиг валиден.
jtpost doctor

# Список постов.
jtpost list

# Просмотр конкретного поста.
jtpost show sample-post
```

## CLI справка

Полная Markdown-справка по всем командам — в [`docs/cli/`](../docs/cli/jtpost.md).
Регенерировать: `jtpost docs`.
