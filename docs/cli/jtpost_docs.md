## jtpost docs

Сгенерировать Markdown-справку по всем CLI-командам

### Synopsis

Генерирует Markdown-документацию по всем подкомандам jtpost через cobra/doc.
По умолчанию пишет в ./docs/cli/. Перезаписывает существующие файлы.

```
jtpost docs [flags]
```

### Options

```
  -h, --help         help for docs
  -o, --out string   директория для сгенерированных .md файлов (default "./docs/cli")
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

