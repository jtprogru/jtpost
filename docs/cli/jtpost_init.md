## jtpost init

Инициализация проекта jtpost

### Synopsis

Создаёт файл конфигурации .jtpost.yaml с настройками по умолчанию и генерирует UUIDv7 для tenant_default/author_default.

```
jtpost init [flags]
```

### Options

```
      --force   перезаписать существующий конфиг без подтверждения
  -h, --help    help for init
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

