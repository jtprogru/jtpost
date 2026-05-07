## jtpost stats

Статистика по постам

### Synopsis

Выводит статистику по постам: количество по статусам, платформам и тегам.

```
jtpost stats [flags]
```

### Options

```
  -f, --format string   формат вывода (table, json) (default "table")
  -h, --help            help for stats
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

