## jtpost completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(jtpost completion bash)

To load completions for every new session, execute once:

#### Linux:

	jtpost completion bash > /etc/bash_completion.d/jtpost

#### macOS:

	jtpost completion bash > $(brew --prefix)/etc/bash_completion.d/jtpost

You will need to start a new shell for this setup to take effect.


```
jtpost completion bash
```

### Options

```
  -h, --help              help for bash
      --no-descriptions   disable completion descriptions
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

* [jtpost completion](jtpost_completion.md)	 - Generate the autocompletion script for the specified shell

