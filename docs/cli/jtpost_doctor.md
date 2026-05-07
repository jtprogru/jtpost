## jtpost doctor

Диагностика конфигурации и доступности зависимостей

### Synopsis

Проверяет, готов ли jtpost к работе:
  • найден ли конфиг и валиден ли YAML;
  • доступна ли директория постов на чтение/запись;
  • можно ли использовать SQLite-базу;
  • отвечает ли Telegram Bot API на токен из конфига;
  • задана ли переменная VISUAL/EDITOR.

Возвращает код 0, если все критичные проверки пройдены.

```
jtpost doctor [flags]
```

### Options

```
  -h, --help   help for doctor
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

