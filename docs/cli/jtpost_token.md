## jtpost token

Управление Personal Access Tokens

### Synopsis

Команды для создания, просмотра и отзыва PAT (требует storage.type=sqlite|postgres).

### Options

```
  -h, --help   help for token
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
* [jtpost token create](jtpost_token_create.md)	 - Создать новый PAT
* [jtpost token list](jtpost_token_list.md)	 - Список PAT пользователя
* [jtpost token revoke](jtpost_token_revoke.md)	 - Отозвать PAT

