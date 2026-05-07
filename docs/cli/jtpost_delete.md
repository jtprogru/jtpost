## jtpost delete

Удалить пост

### Synopsis

Удаляет пост по его идентификатору. Без флага --force запрашивает подтверждение.

```
jtpost delete <id> [flags]
```

### Options

```
  -f, --force   удалить без подтверждения
  -h, --help    help for delete
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

