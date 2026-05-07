## jtpost serve

Запустить HTTP сервер

### Synopsis

Запускает встроенный HTTP сервер с REST API и Web UI для управления постами.

```
jtpost serve [flags]
```

### Options

```
  -a, --addr string         адрес для прослушивания (default "localhost")
  -h, --help                help for serve
      --log-format string   формат логов: text (человекочитаемый) или json (структурированный) (default "text")
  -p, --port int            порт для прослушивания (default 8080)
  -v, --verbose             включить подробное логирование (DEBUG режим)
```

### Options inherited from parent commands

```
      --auth string        Bearer token (PAT) для --remote (fallback: env JTPOST_AUTH_TOKEN)
  -c, --config string      путь к конфигурационному файлу (default ".jtpost.yaml")
  -D, --posts-dir string   директория с постами (переопределяет конфиг)
      --remote string      URL удалённого jtpost API (включает remote-mode для CLI)
```

### SEE ALSO

* [jtpost](jtpost.md)	 - CLI-редактор постов для Telegram

