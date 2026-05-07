## jtpost token create

Создать новый PAT

```
jtpost token create [flags]
```

### Options

```
      --expires-in duration   время до истечения (например 90d=2160h); 0 = без истечения
  -h, --help                  help for create
      --name string           имя токена (для идентификации)
      --user-id string        UUID пользователя
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

* [jtpost token](jtpost_token.md)	 - Управление Personal Access Tokens

