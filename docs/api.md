# HTTP API Документация

## Обзор

**jtpost** предоставляет встроенный HTTP сервер с REST API и Web UI для управления постами.

## Запуск сервера

```bash
jtpost serve [флаги]
```

**Флаги:**
- `--addr`, `-a` — адрес для прослушивания (по умолчанию: `localhost`)
- `--port`, `-p` — порт для прослушивания (по умолчанию: `8080`)

**Пример:**
```bash
jtpost serve --addr 0.0.0.0 --port 3000
```

## Web UI

Web UI доступен по адресу `http://localhost:8080` после запуска сервера.

**Возможности:**
- 📊 Список постов с фильтрацией
- 📈 Статистика по постам
- 📌 Рекомендации следующего поста
- ✏️ Редактирование постов
- 📤 Публикация в Telegram

**Технологии:**
- [htmx](https://htmx.org) — динамические обновления без JavaScript-фреймворков
- Встроенный HTML шаблон с CSS стилями

---

## REST API

### Базовый URL

```
http://localhost:8080/api
```

### Формат данных

- **Request/Response:** JSON
- **Кодировка:** UTF-8
- **Content-Type:** `application/json`

---

## Endpoints

### Posts

#### GET /api/posts

Получить список постов с опциональной фильтрацией.

**Query параметры:**
| Параметр | Тип | Описание |
|----------|-----|----------|
| `status` | string[] | Фильтр по статусам (можно несколько) |
| `platform` | string[] | Фильтр по платформам (можно несколько) |
| `tag` | string[] | Фильтр по тегам (можно несколько) |
| `search` | string | Поиск по заголовку/slug |

**Пример запроса:**
```bash
curl "http://localhost:8080/api/posts?status=draft&tag=golang"
```

**Пример ответа:**
```json
[
  {
    "id": "1772824850782694000-moy-pervyy-post",
    "title": "Мой первый пост",
    "slug": "moy-pervyy-post",
    "status": "draft",
    "platforms": ["telegram"],
    "tags": ["golang", "cli"],
    "deadline": "2026-02-01T18:00:00Z",
    "scheduled_at": null,
    "published_at": null,
    "content": "Тело поста...",
    "external": {
      "telegram_url": ""
    }
  }
]
```

---

#### POST /api/posts

Создать новый пост.

**Request body:**
```json
{
  "title": "Новый пост",
  "slug": "novyy-post",
  "platforms": ["telegram"],
  "tags": ["golang"]
}
```

**Поля:**
| Поле | Тип | Обязательное | Описание |
|------|-----|--------------|----------|
| `title` | string | ✅ | Заголовок поста |
| `slug` | string | ❌ | Slug (генерируется автоматически если не указан) |
| `platforms` | string[] | ❌ | Платформы (по умолчанию: `["telegram"]`) |
| `tags` | string[] | ❌ | Теги |

**Пример запроса:**
```bash
curl -X POST http://localhost:8080/api/posts \
  -H "Content-Type: application/json" \
  -d '{"title":"Новый пост","tags":["golang"]}'
```

**Пример ответа (201 Created):**
```json
{
  "id": "1772824850782694000-novyy-post",
  "title": "Новый пост",
  "slug": "novyy-post",
  "status": "idea",
  "platforms": ["telegram"],
  "tags": ["golang"],
  "deadline": null,
  "scheduled_at": null,
  "published_at": null,
  "content": "",
  "external": {
    "telegram_url": ""
  }
}
```

---

#### GET /api/posts/{id}

Получить пост по идентификатору.

**Пример запроса:**
```bash
curl http://localhost:8080/api/posts/1772824850782694000-moy-pervyy-post
```

**Пример ответа:**
```json
{
  "id": "1772824850782694000-moy-pervyy-post",
  "title": "Мой первый пост",
  "slug": "moy-pervyy-post",
  "status": "draft",
  "platforms": ["telegram"],
  "tags": ["golang", "cli"],
  "deadline": "2026-02-01T18:00:00Z",
  "scheduled_at": null,
  "published_at": null,
  "content": "# Заголовок\n\nТело поста...",
  "external": {
    "telegram_url": ""
  }
}
```

**Ответы:**
- `200 OK` — пост найден
- `404 Not Found` — пост не найден

---

#### PATCH /api/posts/{id}

Обновить пост.

**Request body:**
```json
{
  "title": "Обновлённый заголовок",
  "status": "ready",
  "tags": ["golang", "cli", "update"],
  "deadline": "2026-02-10T18:00:00Z",
  "scheduled_at": "2026-02-12T10:00:00Z",
  "content": "# Заголовок\n\nОбновлённое тело поста..."
}
```

**Поля для обновления (все опциональны):**
| Поле | Тип | Описание |
|------|-----|----------|
| `title` | string | Новый заголовок |
| `status` | string | Новый статус (idea/draft/ready/scheduled/published) |
| `tags` | string[] | Новые теги |
| `deadline` | datetime | Новый дедлайн |
| `scheduled_at` | datetime | Время запланированной публикации |
| `content` | string | Тело поста (Markdown) |

**Пример запроса:**
```bash
curl -X PATCH http://localhost:8080/api/posts/1772824850782694000-moy-pervyy-post \
  -H "Content-Type: application/json" \
  -d '{"status":"ready","tags":["golang","cli"]}'
```

**Пример ответа (200 OK):**
```json
{
  "id": "1772824850782694000-moy-pervyy-post",
  "title": "Мой первый пост",
  "slug": "moy-pervyy-post",
  "status": "ready",
  "platforms": ["telegram"],
  "tags": ["golang", "cli"],
  "deadline": null,
  "scheduled_at": null,
  "published_at": null,
  "content": "...",
  "external": {
    "telegram_url": ""
  }
}
```

---

#### DELETE /api/posts/{id}

Удалить пост.

**Пример запроса:**
```bash
curl -X DELETE http://localhost:8080/api/posts/1772824850782694000-moy-pervyy-post
```

**Ответы:**
- `204 No Content` — пост удалён
- `404 Not Found` — пост не найден
- `500 Internal Server Error` — ошибка при удалении

---

#### POST /api/posts/{id}/publish

Опубликовать пост на платформы.

**Request body:**
```json
{
  "platforms": ["telegram"]
}
```

**Требования:**
- Статус поста должен быть `ready` или `scheduled`
- Платформы должны быть настроены в конфигурации

**Пример запроса:**
```bash
curl -X POST http://localhost:8080/api/posts/1772824850782694000-moy-pervyy-post/publish \
  -H "Content-Type: application/json" \
  -d '{"platforms":["telegram"]}'
```

**Пример ответа (200 OK):**
```json
{
  "id": "1772824850782694000-moy-pervyy-post",
  "title": "Мой первый пост",
  "slug": "moy-pervyy-post",
  "status": "published",
  "platforms": ["telegram"],
  "tags": [],
  "deadline": null,
  "scheduled_at": null,
  "published_at": "2026-02-03T10:00:00Z",
  "content": "...",
  "external": {
    "telegram_url": "https://t.me/channelname/123"
  }
}
```

**Ответы:**
- `200 OK` — публикация успешна
- `400 Bad Request` — пост не готов к публикации или платформа не настроена
- `404 Not Found` — пост не найден
- `500 Internal Server Error` — ошибка публикации

---

### Stats

#### GET /api/stats

Получить статистику по постам.

**Пример запроса:**
```bash
curl http://localhost:8080/api/stats
```

**Пример ответа:**
```json
{
  "total": 10,
  "by_status": {
    "idea": 2,
    "draft": 3,
    "ready": 1,
    "scheduled": 1,
    "published": 3
  },
  "by_platform": {
    "telegram": 10
  },
  "by_tag": {
    "cli": 5,
    "go": 8,
    "telegram": 3
  }
}
```

---

### Next

#### GET /api/next

Получить рекомендацию следующего поста для работы.

**Алгоритм рекомендации:**
1. Посты с просроченным дедлайном (наивысший приоритет)
2. Посты с ближайшим дедлайном
3. Посты с ближайшей запланированной публикацией
4. Посты по статусу (ready > draft > idea)

**Пример запроса:**
```bash
curl http://localhost:8080/api/next
```

**Пример ответа:**
```json
{
  "id": "1772824850782694000-obzor-go-125",
  "title": "Обзор Go 1.25",
  "slug": "obzor-go-125",
  "status": "draft",
  "platforms": ["telegram"],
  "tags": ["go", "release"],
  "deadline": "2026-02-10T18:00:00Z",
  "scheduled_at": null,
  "published_at": null,
  "content": "...",
  "external": {
    "telegram_url": ""
  }
}
```

**Ответы:**
- `200 OK` — пост найден
- `404 Not Found` — нет постов для рекомендации (возвращает `{"error": "no posts to recommend"}`)

---

### Plan

#### GET /api/plan

Получить план публикаций на ближайшие дни.

**Query параметры:**
| Параметр | Тип | Описание | По умолчанию |
|----------|-----|----------|--------------|
| `days` | int | Период планирования в днях | 30 |

**Пример запроса:**
```bash
curl "http://localhost:8080/api/plan?days=14"
```

**Пример ответа:**
```json
[
  {
    "id": "1772824850782694000-design-patterns",
    "title": "Design Patterns",
    "slug": "design-patterns",
    "status": "ready",
    "date": "2026-02-05T10:00:00Z",
    "date_type": "schedule"
  },
  {
    "id": "1772824850782694000-performance",
    "title": "Performance Tips",
    "slug": "performance",
    "status": "draft",
    "date": "2026-02-12T18:00:00Z",
    "date_type": "deadline"
  }
]
```

**Поля:**
| Поле | Тип | Описание |
|------|-----|----------|
| `id` | string | Идентификатор поста |
| `title` | string | Заголовок |
| `slug` | string | Slug |
| `status` | string | Статус |
| `date` | datetime | Дата дедлайна или запланированной публикации |
| `date_type` | string | Тип даты: `schedule` или `deadline` |

---

## Коды ответов

| Код | Описание |
|-----|----------|
| `200 OK` | Успешный запрос |
| `201 Created` | Ресурс создан |
| `204 No Content` | Ресурс удалён |
| `400 Bad Request` | Ошибка валидации или неверные данные |
| `404 Not Found` | Ресурс не найден |
| `405 Method Not Allowed` | Метод не поддерживается |
| `500 Internal Server Error` | Внутренняя ошибка сервера |

---

## Форматы данных

### PostStatus

Допустимые значения статуса:
- `idea` — идея
- `draft` — черновик
- `ready` — готов к публикации
- `scheduled` — запланирован
- `published` — опубликован

### Platform

Допустимые значения платформ:
- `telegram` — Telegram канал

### DateTime

Даты и время в формате RFC3339:
```
2026-02-01T18:00:00Z
2026-02-03T10:00:00+03:00
```

---

## Примеры использования

### cURL

```bash
# Создать пост
curl -X POST http://localhost:8080/api/posts \
  -H "Content-Type: application/json" \
  -d '{"title":"Мой пост","tags":["golang"]}'

# Получить список всех постов
curl http://localhost:8080/api/posts

# Получить посты со статусом draft
curl "http://localhost:8080/api/posts?status=draft"

# Обновить пост
curl -X PATCH http://localhost:8080/api/posts/ID \
  -H "Content-Type: application/json" \
  -d '{"status":"ready"}'

# Опубликовать пост
curl -X POST http://localhost:8080/api/posts/ID/publish \
  -H "Content-Type: application/json" \
  -d '{"platforms":["telegram"]}'

# Получить статистику
curl http://localhost:8080/api/stats

# Получить рекомендацию
curl http://localhost:8080/api/next

# Получить план на 14 дней
curl "http://localhost:8080/api/plan?days=14"
```

### JavaScript (Fetch API)

```javascript
// Получить список постов
const posts = await fetch('http://localhost:8080/api/posts')
  .then(r => r.json());

// Создать пост
const newPost = await fetch('http://localhost:8080/api/posts', {
  method: 'POST',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({
    title: 'Новый пост',
    tags: ['golang']
  })
}).then(r => r.json());

// Обновить статус
await fetch(`http://localhost:8080/api/posts/${newPost.id}`, {
  method: 'PATCH',
  headers: {'Content-Type': 'application/json'},
  body: JSON.stringify({status: 'ready'})
});

// Опубликовать
const published = await fetch(
  `http://localhost:8080/api/posts/${newPost.id}/publish`,
  {
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    body: JSON.stringify({platforms: ['telegram']})
  }
).then(r => r.json());

console.log('Опубликовано:', published.external.telegram_url);
```

### Python (requests)

```python
import requests

BASE_URL = 'http://localhost:8080/api'

# Создать пост
response = requests.post(f'{BASE_URL}/posts', json={
    'title': 'Мой пост',
    'tags': ['golang']
})
post = response.json()

# Обновить статус
requests.patch(f'{BASE_URL}/posts/{post["id"]}', json={
    'status': 'ready'
})

# Опубликовать
response = requests.post(f'{BASE_URL}/posts/{post["id"]}/publish', json={
    'platforms': ['telegram']
})
published = response.json()

print(f"Опубликовано: {published['external']['telegram_url']}")

# Получить статистику
stats = requests.get(f'{BASE_URL}/stats').json()
print(f"Всего постов: {stats['total']}")
```

---

## Web UI скриншоты

### Главная страница

- Список постов с фильтрацией
- Статистика по статусам
- Рекомендация следующего поста

### Редактирование поста

- Модальное окно с полями поста
- Редактор Markdown контента
- Настройка дедлайнов и планирования
- Публикация в Telegram

---

## Интеграция с CI/CD

### GitHub Actions пример

```yaml
name: Publish Post

on:
  workflow_dispatch:
    inputs:
      post_id:
        description: 'Post ID to publish'
        required: true

jobs:
  publish:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.25'
      
      - name: Build jtpost
        run: go build -o jtpost ./cmd/jtpost
      
      - name: Publish post
        env:
          TELEGRAM_BOT_TOKEN: ${{ secrets.TELEGRAM_BOT_TOKEN }}
          TELEGRAM_CHAT_ID: ${{ secrets.TELEGRAM_CHAT_ID }}
        run: |
          ./jtpost publish ${{ github.event.inputs.post_id }} --to telegram
```

---

## Troubleshooting

### Ошибка: "post is not ready to publish"

**Решение:** Установите статус `ready` перед публикацией:
```bash
curl -X PATCH http://localhost:8080/api/posts/ID \
  -H "Content-Type: application/json" \
  -d '{"status":"ready"}'
```

### Ошибка: "publisher для платформы telegram не найден"

**Решение:** Проверьте конфигурацию Telegram в `.jtpost.yaml`:
```yaml
telegram:
  bot_token: "your-bot-token"
  chat_id: "@yourchannel"
```

### Ошибка: "config file not found"

**Решение:** Запустите `jtpost init` для создания конфигурации или укажите путь через `--config`:
```bash
jtpost serve --config /path/to/.jtpost.yaml
```
