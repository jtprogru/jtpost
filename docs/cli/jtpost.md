## jtpost

CLI-редактор постов для Telegram

### Synopsis

jtpost — утилита для управления жизненным циклом постов: от идеи до публикации в Telegram.

### Options

```
      --auth string        Bearer token (PAT) для --remote (fallback: env JTPOST_AUTH_TOKEN)
  -c, --config string      путь к конфигурационному файлу (default ".jtpost.yaml")
  -h, --help               help for jtpost
  -D, --posts-dir string   директория с постами (переопределяет конфиг)
      --remote string      URL удалённого jtpost API (включает remote-mode для CLI)
  -v, --verbose            подробный вывод
```

### SEE ALSO

* [jtpost completion](jtpost_completion.md)	 - Generate the autocompletion script for the specified shell
* [jtpost delete](jtpost_delete.md)	 - Удалить пост
* [jtpost docs](jtpost_docs.md)	 - Сгенерировать Markdown-справку по всем CLI-командам
* [jtpost doctor](jtpost_doctor.md)	 - Диагностика конфигурации и доступности зависимостей
* [jtpost edit](jtpost_edit.md)	 - Редактировать пост
* [jtpost import](jtpost_import.md)	 - Импортировать посты из существующих Markdown файлов
* [jtpost init](jtpost_init.md)	 - Инициализация проекта jtpost
* [jtpost list](jtpost_list.md)	 - Список постов
* [jtpost migrate](jtpost_migrate.md)	 - Миграция данных постов между storage backend
* [jtpost new](jtpost_new.md)	 - Создание нового поста
* [jtpost next](jtpost_next.md)	 - Рекомендация следующего поста
* [jtpost outbox](jtpost_outbox.md)	 - Управление очередью публикаций (outbox)
* [jtpost plan](jtpost_plan.md)	 - План публикаций
* [jtpost publish](jtpost_publish.md)	 - Опубликовать пост
* [jtpost serve](jtpost_serve.md)	 - Запустить HTTP сервер
* [jtpost show](jtpost_show.md)	 - Показать детали поста
* [jtpost stats](jtpost_stats.md)	 - Статистика по постам
* [jtpost status](jtpost_status.md)	 - Смена статуса поста
* [jtpost token](jtpost_token.md)	 - Управление Personal Access Tokens
* [jtpost user](jtpost_user.md)	 - Управление пользователями
* [jtpost worker](jtpost_worker.md)	 - Background worker для outbox-очереди публикаций

