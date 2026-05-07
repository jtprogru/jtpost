---
id: 01913e00-0000-7000-8000-000000000001
tenant_id: 00000000-0000-7000-8000-000000000001
author_id: 00000000-0000-7000-8000-000000000002
title: Пример поста для Telegram
slug: sample-post
status: draft
tags:
  - go
  - telegram
created_at: 2026-05-07T10:00:00Z
updated_at: 2026-05-07T10:00:00Z
revision: 1
deadline: 2026-05-10T18:00:00Z
---

# Заголовок поста

Это пример поста с frontmatter в формате jtpost. Frontmatter содержит метаданные,
а тело — Markdown, который пойдёт в Telegram (с учётом MarkdownV2-экранирования
при `parse_mode: MarkdownV2`).

## Подзаголовок

- Список можно использовать.
- Ссылки [тоже работают](https://example.com).
- Картинки `![alt](https://example.com/img.png)` будут отправлены через
  `sendPhoto` или `sendMediaGroup` (этап 8).

> Цитаты тоже поддерживаются.

`inline code` и блоки кода:

```go
func main() {
    fmt.Println("hello jtpost")
}
```
