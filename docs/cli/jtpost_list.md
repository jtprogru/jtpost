## jtpost list

Список постов

### Synopsis

Выводит список постов с возможностью фильтрации по статусу и тегам.

```
jtpost list [flags]
```

### Options

```
  -f, --format string    формат вывода (table, json) (default "table")
  -h, --help             help for list
      --no-id            скрыть колонку ID
  -q, --search string    поиск по заголовку/slug
  -s, --status strings   фильтр по статусам
  -t, --tag strings      фильтр по тегам
```

### Options inherited from parent commands

```
      --auth string        Bearer token (PAT) для --remote (fallback: env JTPOST_AUTH_TOKEN)
  -c, --config string      путь к конфигурационному файлу (default ".jtpost.yaml")
  -D, --posts-dir string   директория с постами (переопределяет конфиг)
      --remote string      URL удалённого jtpost API (включает remote-mode для CLI)
  -v, --verbose            подробный вывод
```

### SEE ALSO

* [jtpost](jtpost.md)	 - CLI-редактор постов для Telegram

