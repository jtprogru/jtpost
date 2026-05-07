## jtpost migrate db status

Показать статус миграций

```
jtpost migrate db status [flags]
```

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
      --auth string        Bearer token (PAT) для --remote (fallback: env JTPOST_AUTH_TOKEN)
  -c, --config string      путь к конфигурационному файлу (default ".jtpost.yaml")
  -D, --posts-dir string   директория с постами (переопределяет конфиг)
      --remote string      URL удалённого jtpost API (включает remote-mode для CLI)
      --to string          backend (sqlite|postgres)
  -v, --verbose            подробный вывод
```

### SEE ALSO

* [jtpost migrate db](jtpost_migrate_db.md)	 - Управление схемой БД (goose-миграции)

