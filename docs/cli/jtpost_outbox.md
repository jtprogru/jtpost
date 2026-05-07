## jtpost outbox

Управление очередью публикаций (outbox)

### Options

```
  -h, --help   help for outbox
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
* [jtpost outbox enqueue](jtpost_outbox_enqueue.md)	 - Поставить пост в очередь на публикацию
* [jtpost outbox list](jtpost_outbox_list.md)	 - Показать записи outbox

