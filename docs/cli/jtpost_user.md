## jtpost user

Управление пользователями

### Synopsis

Команды управления учётными записями (требует storage.type=sqlite|postgres).

### Options

```
  -h, --help   help for user
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
* [jtpost user create](jtpost_user_create.md)	 - Создать пользователя
* [jtpost user delete](jtpost_user_delete.md)	 - Удалить пользователя (caskade удалит токены)
* [jtpost user list](jtpost_user_list.md)	 - Список пользователей текущего tenant

