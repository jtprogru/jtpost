## jtpost import

Импортировать посты из существующих Markdown файлов

### Synopsis

Импортирует посты из указанной директории (или content/posts/ по умолчанию).
Сканирует Markdown файлы, парсит frontmatter, нормализует к стандарту jtpost.

Флаги:
  --dry-run        Показать, что будет импортировано, без записи
  --interactive    Запрашивать подтверждение для каждого файла
  --output         Директория для импортированных файлов (по умолчанию postsDir из конфига)
  --update         Обновлять существующие посты вместо пропуска


```
jtpost import [source-dir] [flags]
```

### Options

```
  -n, --dry-run         режим предпросмотра без записи
  -h, --help            help for import
  -i, --interactive     интерактивный режим с подтверждениями
  -o, --output string   выходная директория (по умолчанию postsDir из конфига)
  -u, --update          обновлять существующие посты
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

