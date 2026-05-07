## jtpost show

Показать детали поста

### Synopsis

Выводит подробную информацию о посте по его идентификатору.

```
jtpost show <id> [flags]
```

### Options

```
  -h, --help   help for show
  -j, --json   вывод в формате JSON
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

