# «CLI‑редактор постов» для блога/телеграм‑канала

Сделаем инструмент с чётким фокусом: управлять твоим контент‑пайплайном (черновики → готово → опубликовано) для Telegram из терминала.[^1][^2]

## Общая концепция

- CLI‑утилита, работающая поверх git‑репозитория с постами в Markdown (например, того же блога на Hugo).[^3][^4]
- Состояния поста: `idea` → `draft` → `ready` → `scheduled` → `published`, плюс метаданные (платформы, дедлайны, ссылки на публикации).[^2][^1]
- Позже — лёгкий HTTP API, чтобы поверх этого сделать Web UI или интегрировать бота.[^5][^6]

***

## Архитектура (высокоуровнево)

### Основные компоненты

- **core/domain**
    - Тип `Post` (id, title, slug, content, frontmatter, status, tags, platforms, scheduleAt, publishedAt, externalUrls).[^6][^7]
    - Интерфейсы `PostRepository`, `Scheduler`, `Publisher` (Telegram/др.).[^8][^6]
- **infrastructure**
    - `FilesystemPostRepository` — чтение/запись Markdown с frontmatter (YAML/TOML) из директории контента.[^4][^9]
    - `Config` — глобальный конфиг проекта (`.jtpost.yaml`), где описаны пути, шаблоны, параметры Telegram‑бота и т.п.[^10][^8]
    - `TelegramPublisher` — обёртка над Bot API (по токену, chat_id; умеет отправлять текст/HTML/Markdown).[^11][^1]
- **application/services**
    - `PostService` — создание/редактирование/смена статусов/валидация.[^7][^6]
    - `PlanningService` — работа с дедлайнами, выбор «что писать следующим» (например, по дате/тегам).[^2]
    - `SyncService` — связка: один пост → несколько площадок (Telegram), хранение ссылок на опубликованные посты.[^1][^2]
- **interfaces/cli**
    - На базе cobra или urfave/cli: команды `post new`, `post edit`, `post list`, `post status`, `post publish`, `post sync`.[^10][^8]
- **(позже) interfaces/http**
    - Простейший REST API + HTML UI (список постов, форма редактирования), чтобы иногда выходить за пределы терминала.[^5][^6][^7]

***

## Формат хранения постов

- Каталог, например `content/posts/` (для блога) и `content/telegram/` (отдельные форматы, если нужно).[^3][^4]
- Файл `YYYY-MM-DD-slug.md` с frontmatter + тело, совместимый с Hugo, чтобы можно было просто подкладывать в блог.[^13][^4]

Пример frontmatter:

```yaml
title: "SRE и пет-проекты"
slug: "sre-pet-projects"
status: "draft"           # idea/draft/ready/scheduled/published
platforms:
  - "telegram"
deadline: "2026-02-01"
scheduled_at: "2026-02-03T10:00:00+03:00"
tags: ["sre", "career", "golang"]
external:
  blog_url: ""
  telegram_url: ""
```

Такой формат легко дружит и с Hugo, и с любым самописным генератором.[^4][^3]

***

## Roadmap по этапам

### Этап 0: скелет CLI

Цель: рабочий бинарник с одной‑двумя командами и конфигом.

- Инициализация проекта (`go mod`, структура директорий, выбор cli‑фреймворка).[^8][^10]
- Команда `post init`:
    - Создать `.jtpost.yaml` с путями к репозиторию постов и дефолтным шаблоном frontmatter.[^10]
- Команда `post new`:
    - Создать Markdown‑файл по шаблону, сгенерировать slug и дату, открыть в `$EDITOR`.[^4][^10]

Результат: можно быстро заводить идеи/черновики «правильного» формата.

***

### Этап 1: работа с жизненным циклом поста

Цель: держать список постов и переводить их по статусам.

Функции:

- `post list`
    - Фильтры: по статусу, тегам, платформам, deadline < сегодня.[^2]
    - Форматы вывода: таблица, `--json`, `--short`.[^8]
- `post status <id> --set ready`
    - Смена статуса и валидация (например, `ready` требует title и минимум N символов).[^6]
- `post show <id>`
    - Краткий просмотр метаданных + путь к файлу.[^6]

Технически:

- `PostRepository` поверх файловой системы: парсинг frontmatter и поиск по директории.[^9][^4]
- Условное «id» можно сделать как относительный путь или hash от slug+date.[^8]

***

### Этап 2: интеграция с блогом (Hugo/статик)

Цель: связать CLI с твоим блоговым репо.

Функции:

- Поддержка разных «профилей» проекта (несколько блогов/репо в конфиге).[^3][^4]
- `post publish --to blog <id>`
    - Перенос/копирование файла в нужный каталог Hugo, выставление `draft: false`, добавление нужных полей frontmatter.[^12][^4]
    - Опционально: `--git` — сразу добавить `git add` и краткий commit.[^14][^12]

Плюсы:

- Начинаешь реально использовать тул в ежедневном пайплайне блога.[^14][^12]
- Никаких завязок на работу, всё чисто персональный контент.[^3]

***

### Этап 3: Telegram‑интеграция

Цель: уметь отправлять посты в канал и сохранять ссылки.

Функции:

- Настройка в `.jtpost.yaml`: токен бота, chat_id канала.[^11][^1]
- Трансформация Markdown → формат, который нормально ест Telegram (либо MarkdownV2, либо HTML).[^11]
- `post publish --to telegram <id>`:
    - Подготовка текста (опциональное обрезание, шаблон «анонс + ссылка на блог»).[^2]
    - Вызов Telegram Bot API `sendMessage`, сохранение `message_link` в `external.telegram_url`.[^1][^11]

Дополнительно:

- Флаг `--dry-run` для предварительного предпросмотра в терминале.[^1]

***

### Этап 4: планирование и рекомендации

Цель: превратить инструмент в «редактора расписания», а не просто «пушер».

Функции:

- `post plan`:
    - Показать календарь/список на ближайшие N дней: какие посты имеют deadline/scheduled, где есть дырки.[^2]
- `post next`:
    - Выбрать «следующий пост для работы» по простому правилу: ближайший deadline среди `idea/draft`, с учётом тегов/платформ.[^2]
- `post stats`:
    - Количество постов по статусам, распределение по тегам, выполненные/проваленные дедлайны.[^2]

***

### Этап 5: HTTP API + Web UI (по желанию)

Цель: выход за пределы CLI, но на твоих условиях.

Функции:

- Встроенный сервер `post serve`:
    - REST API (`/posts`, `/posts/{id}`, действия смены статуса).[^7][^5]
    - Простенький UI: список постов, фильтры, форма редактирования frontmatter и текста (можно начать с read‑only).[^5][^7]

Архитектурно:

- HTTP слой вызывает те же сервисы, что и CLI, чтобы не плодить второй мир.[^8]
- Фронт может быть на голом HTML/htmx, без тяжёлого SPA.[^7]

***

## Технические акценты для практики Go

- Чёткое разделение `main` vs `internal/app`/`pkg`: `main` только парсит флаги и передаёт управление.[^8]
- Конфиги и окружение: поддержка override через env vars/флаги поверх YAML, как в нормальных сервисах.[^10][^8]
- Тестирование:
    - Юнит‑тесты на `PostService` и парсер frontmatter.[^7]
    - Файловый `PostRepository` через временные директории.[^9]

Если хочешь, можно следующим шагом набросать конкретную структуру пакетов (`/cmd/jtpost`, `/internal/core`, `/internal/adapters/...`) и пример интерфейсов (`PostRepository`, `Publisher`) с сигнатурами.

## Список источников

[^1]: <https://postoplan.contenive.com/scheduled-posting-on-telegram>
[^2]: <https://old.junctionbot.io/top-16-bots-and-services-for-autoposting-on-telegram-to-promote-your-content/>
[^3]: <http://www.testingwithmarie.com/posts/20241126-create-a-static-blog-with-hugo/>
[^4]: <https://www.geeksforgeeks.org/go-language/static-site-generation-with-hugo/>
[^5]: <https://mortenvistisen.com/posts/how-to-create-a-blog-using-golang>
[^6]: <https://github.com/jlelse/GoBlog>
[^7]: <https://blog.jetbrains.com/go/2022/11/08/build-a-blog-with-go-templates/>
[^8]: <https://blog.carlana.net/post/2020/go-cli-how-to-and-advice/>
[^9]: <https://dev.to/envitab/how-to-create-a-static-site-generator-with-go-4jgm>
[^10]: <https://dev.to/rinkiyakedad/creating-a-cli-in-golang-5abl>
[^11]: <https://amarketsaffiliates.com/setting-up-automated-posting-on-telegram/>
[^12]: <https://sambloomquist.com/posts/publishing-hugo-static-site-github-pages/>
[^13]: <https://richardfawcett.net/2025/06/12/static-site-generation-in-2025-hugo>
[^14]: <https://www.mattjh.sh/post/hugo-deployment/>
