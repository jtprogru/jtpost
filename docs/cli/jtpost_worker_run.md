## jtpost worker run

Запустить worker (long-running)

```
jtpost worker run [flags]
```

### Options

```
  -h, --help                help for run
      --interval duration   интервал опроса очереди (default 10s)
      --log-format string   формат логов: text или json (default "text")
      --max-attempts int    максимальное число попыток до permanent fail (default 5)
  -v, --verbose             verbose режим (DEBUG уровень)
```

### Options inherited from parent commands

```
      --auth string        Bearer token (PAT) для --remote (fallback: env JTPOST_AUTH_TOKEN)
  -c, --config string      путь к конфигурационному файлу (default ".jtpost.yaml")
  -D, --posts-dir string   директория с постами (переопределяет конфиг)
      --remote string      URL удалённого jtpost API (включает remote-mode для CLI)
```

### SEE ALSO

* [jtpost worker](jtpost_worker.md)	 - Background worker для outbox-очереди публикаций

