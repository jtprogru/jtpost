## jtpost token revoke

Отозвать PAT

```
jtpost token revoke <token-id> [flags]
```

### Options

```
  -h, --help   help for revoke
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

