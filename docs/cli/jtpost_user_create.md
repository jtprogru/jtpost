## jtpost user create

Создать пользователя

```
jtpost user create [flags]
```

### Options

```
      --email string      email пользователя
      --first-owner       создать первого owner (только при пустой users-таблице)
  -h, --help              help for create
      --password string   пароль (≥ 8 символов)
      --role string       роль: owner|editor|author|viewer (default "author")
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

* [jtpost user](jtpost_user.md)	 - Управление пользователями

