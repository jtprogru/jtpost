## jtpost new

Создание нового поста

### Synopsis

Создаёт новый пост с указанным заголовком и открывает его в редакторе.

```
jtpost new [title] [flags]
```

### Options

```
      --author string    author UUID (по умолчанию из конфига)
      --content string   источник контента: '-' для stdin или путь к файлу (только для --remote)
  -e, --editor string    редактор для открытия файла (по умолчанию $VISUAL или $EDITOR)
  -h, --help             help for new
  -s, --slug string      slug поста (по умолчанию генерируется из заголовка)
  -t, --tag strings      теги поста
      --tenant string    tenant UUID (по умолчанию из конфига)
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

