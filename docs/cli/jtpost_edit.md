## jtpost edit

Редактировать пост

### Synopsis

Открывает файл поста в редакторе для редактирования.

```
jtpost edit <id> [flags]
```

### Options

```
      --content string   источник контента: '-' для stdin или путь к файлу (только для --remote)
  -e, --editor string    редактор для открытия файла
  -h, --help             help for edit
      --status string    новый статус: draft|ready|scheduled|published (только для --remote)
      --tag strings      новый набор тегов (replace; только для --remote)
      --title string     новый заголовок (только для --remote)
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

