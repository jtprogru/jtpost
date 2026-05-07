## jtpost publish

Опубликовать пост

### Synopsis

Публикует пост в Telegram.

```
jtpost publish <id> [flags]
```

### Options

```
  -d, --dry-run   режим предпросмотра без публикации
  -h, --help      help for publish
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

