## jtpost migrate

Миграция данных постов между storage backend

### Synopsis

Переносит все посты из source backend в target backend через core.MigratableRepository.

Поддерживаются: fs, sqlite, postgres. Параметры выбираются через --from и --to (обязательны).
Старый формат --db <path> больше не поддерживается; используйте storage.sqlite.dsn в конфиге.

```
jtpost migrate [flags]
```

### Options

```
  -n, --dry-run       режим предпросмотра без миграции
      --from string   source backend (fs|sqlite|postgres)
  -h, --help          help for migrate
  -f, --overwrite     перезаписать target если уже содержит посты
      --to string     target backend (fs|sqlite|postgres)
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
* [jtpost migrate db](jtpost_migrate_db.md)	 - Управление схемой БД (goose-миграции)

