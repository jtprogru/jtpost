## jtpost migrate db

Управление схемой БД (goose-миграции)

### Synopsis

Применяет/проверяет статус миграций для выбранного backend (sqlite или postgres).

### Options

```
  -h, --help        help for db
      --to string   backend (sqlite|postgres)
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

* [jtpost migrate](jtpost_migrate.md)	 - Миграция данных постов между storage backend
* [jtpost migrate db status](jtpost_migrate_db_status.md)	 - Показать статус миграций
* [jtpost migrate db up](jtpost_migrate_db_up.md)	 - Применить все pending-миграции

