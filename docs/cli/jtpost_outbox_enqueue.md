## jtpost outbox enqueue

Поставить пост в очередь на публикацию

```
jtpost outbox enqueue <post-id> [flags]
```

### Options

```
  -h, --help   help for enqueue
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

* [jtpost outbox](jtpost_outbox.md)	 - Управление очередью публикаций (outbox)

