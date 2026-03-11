# Конфигурация jtpost

## Обзор

Конфигурация **jtpost** хранится в файле `.jtpost.yaml` в корне проекта и содержит настройки для работы с постами и интеграции с внешними сервисами.

## Быстрый старт

### Инициализация

Создайте конфигурационный файл командой:

```bash
jtpost init
```

Это создаст файл `.jtpost.yaml` с настройками по умолчанию:

```yaml
posts_dir: content/posts
templates_dir: templates

telegram: {}

defaults:
  status: idea
  platforms:
    - telegram
```

## Структура конфигурации

### Корневые поля

| Поле | Тип | Описание | По умолчанию |
|------|-----|----------|--------------|
| `posts_dir` | string | Директория для хранения постов | `content/posts` |
| `templates_dir` | string | Директория для шаблонов | `templates` |
| `telegram` | object | Настройки Telegram | `{}` |
| `defaults` | object | Настройки по умолчанию | См. ниже |

### Telegram настройки

| Поле | Тип | Описание | Обязательное |
|------|-----|----------|--------------|
| `bot_token` | string | Токен Telegram бота | ✅ Для публикации |
| `chat_id` | string | ID канала/чата | ✅ Для публикации |

**Пример:**
```yaml
telegram:
  bot_token: "1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi"
  chat_id: "@mychannel"
```

или для приватного канала:
```yaml
telegram:
  bot_token: "1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi"
  chat_id: "-1001234567890"
```

### Настройки по умолчанию

| Поле | Тип | Описание | По умолчанию |
|------|-----|----------|--------------|
| `status` | string | Статус новых постов | `idea` |
| `platforms` | string[] | Платформы по умолчанию | `["telegram"]` |
| `deadline` | datetime | Дедлайн по умолчанию | `null` |

**Пример:**
```yaml
defaults:
  status: draft
  platforms:
    - telegram
```

## Переменные окружения

Конфигурация поддерживает подстановку переменных окружения для безопасного хранения чувствительных данных.

### Синтаксис

```yaml
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  chat_id: "${TELEGRAM_CHAT_ID}"
```

### Поддерживаемые переменные

| Переменная | Описание |
|------------|----------|
| `TELEGRAM_BOT_TOKEN` | Токен Telegram бота |
| `TELEGRAM_CHAT_ID` | ID канала/чата Telegram |
| `VISUAL` | Предпочитаемый визуальный редактор |
| `EDITOR` | Текстовый редактор по умолчанию |

### Пример использования

**.jtpost.yaml:**
```yaml
posts_dir: content/posts
templates_dir: templates

telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  chat_id: "${TELEGRAM_CHAT_ID}"

defaults:
  status: idea
  platforms:
    - telegram
```

**.env (локальный файл, не коммитить в git):**
```bash
TELEGRAM_BOT_TOKEN=1234567890:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi
TELEGRAM_CHAT_ID=-1001234567890
```

**Загрузка в shell:**
```bash
# Bash/Zsh
export $(cat .env | xargs)

# Или использовать с командой
TELEGRAM_BOT_TOKEN=xxx TELEGRAM_CHAT_ID=yyy jtpost publish <id> --to telegram
```

## Полная конфигурация

```yaml
# Директория с постами
posts_dir: content/posts

# Директория с шаблонами
templates_dir: templates

# Настройки Telegram
telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  chat_id: "${TELEGRAM_CHAT_ID}"

# Настройки по умолчанию
defaults:
  status: idea
  platforms:
    - telegram
```

## Примеры конфигураций

### Минимальная (только блог)

```yaml
posts_dir: content/posts
templates_dir: templates

defaults:
  status: draft
  platforms:
    - telegram
```

### Полная (с Telegram)

```yaml
posts_dir: content/posts
templates_dir: templates

telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  chat_id: "@mychannel"

defaults:
  status: idea
  platforms:
    - telegram
```

### Несколько платформ (будущее расширение)

```yaml
posts_dir: content/posts
templates_dir: templates

telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"
  chat_id: "@mychannel"

defaults:
  status: draft
  platforms:
    - telegram
```

## Получение Telegram токенов

### Bot Token

1. Откройте [@BotFather](https://t.me/BotFather) в Telegram
2. Отправьте команду `/newbot`
3. Следуйте инструкциям для создания бота
4. Скопируйте полученный токен (выглядит как `1234567890:ABC...`)

### Chat ID

**Для публичного канала:**
- Используйте имя канала с `@`: `@mychannel`

**Для приватного канала:**
1. Добавьте бота в канал как администратора
2. Отправьте сообщение в канале
3. Используйте API для получения ID:
   ```bash
   curl "https://api.telegram.org/bot<BOT_TOKEN>/getUpdates"
   ```
4. Найдите `"chat":{"id":-1001234567890,...}` в ответе
5. Используйте полный ID (с `-100` в начале)

## Валидация конфигурации

При запуске команд конфигурация автоматически проверяется на валидность.

### Обязательные поля

- `posts_dir` — должен быть указан и директория должна существовать

### Опциональные поля

- `telegram.bot_token` и `telegram.chat_id` — требуются только для публикации в Telegram

### Ошибки конфигурации

| Ошибка | Причина | Решение |
|--------|---------|---------|
| `config file not found` | Файл `.jtpost.yaml` не найден | Запустите `jtpost init` |
| `config file is invalid` | Ошибка парсинга YAML | Проверьте синтаксис YAML |
| `posts directory not found` | Директория постов не существует | Создайте директорию или исправьте путь |

## Переопределение конфигурации

### Через CLI флаги

Некоторые настройки можно переопределить через флаги командной строки:

```bash
# Использовать другой конфиг
jtpost --config /path/to/other.jtpost.yaml list

# Переопределить директорию постов
jtpost --posts-dir /tmp/posts list

# Комбинация
jtpost -c custom.yaml -D /tmp/posts list
```

### Через переменные окружения

```bash
# Переопределить токен
export TELEGRAM_BOT_TOKEN=new_token
jtpost publish <id> --to telegram
```

## Файл .jtpost.example.yaml

Для проектов с несколькими разработчиками рекомендуется создать файл `.jtpost.example.yaml` с примером конфигурации (без чувствительных данных):

```yaml
# .jtpost.example.yaml
posts_dir: content/posts
templates_dir: templates

telegram:
  bot_token: "${TELEGRAM_BOT_TOKEN}"  # Замените на ваш токен
  chat_id: "${TELEGRAM_CHAT_ID}"      # Замените на ваш chat_id

defaults:
  status: idea
  platforms:
    - telegram
```

Добавьте этот файл в репозиторий, а `.jtpost.yaml` добавьте в `.gitignore`:

```gitignore
# .gitignore
.jtpost.yaml
```

## Интеграция с CI/CD

### GitHub Actions

```yaml
# .github/workflows/publish.yml
name: Publish Post

on:
  workflow_dispatch:
    inputs:
      post_id:
        description: 'Post ID'
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
      
      - name: Build
        run: go build -o jtpost ./cmd/jtpost
      
      - name: Publish
        env:
          TELEGRAM_BOT_TOKEN: ${{ secrets.TELEGRAM_BOT_TOKEN }}
          TELEGRAM_CHAT_ID: ${{ secrets.TELEGRAM_CHAT_ID }}
        run: |
          ./jtpost publish ${{ github.event.inputs.post_id }} --to telegram
```

### Настройка секретов

В настройках репозитория GitHub добавьте секреты:
- `TELEGRAM_BOT_TOKEN`
- `TELEGRAM_CHAT_ID`

## Миграция конфигурации

### Версия 0.1.x → 0.2.x

В версии 0.2.x изменилась структура секции Telegram:

**Старый формат:**
```yaml
platforms:
  telegram:
    bot_token: "..."
    chat_id: "..."
```

**Новый формат:**
```yaml
telegram:
  bot_token: "..."
  chat_id: "..."
```

## Диагностика

### Проверка конфигурации

```bash
# Проверить загрузку конфигурации
jtpost list --verbose

# Проверить подключение к Telegram
jtpost publish <id> --to telegram --dry-run
```

### Логирование

Включите подробный вывод для отладки:

```bash
jtpost --verbose list
jtpost -v serve
```

## FAQ

### Как использовать разные конфигурации для dev/prod?

Создайте несколько файлов конфигурации:

```bash
.jtpost.dev.yaml   # Локальная разработка
.jtpost.prod.yaml  # Продакшен
```

Переключайтесь через флаг:

```bash
jtpost -c .jtpost.dev.yaml serve
jtpost -c .jtpost.prod.yaml publish <id> --to telegram
```

### Как изменить директорию постов?

Измените поле `posts_dir` в конфигурации:

```yaml
posts_dir: /absolute/path/to/posts
# или
posts_dir: ../my-blog/posts
```

### Можно ли использовать относительные пути?

Да, относительные пути разрешаются относительно текущего рабочего каталога:

```yaml
posts_dir: content/posts  # ./content/posts
templates_dir: templates  # ./templates
```

### Как экспортировать конфигурацию?

```bash
# Показать текущую конфигурацию (без секретов)
jtpost config show

# Экспорт в JSON
jtpost config show --format json
```
