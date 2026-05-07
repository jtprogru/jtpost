## jtpost completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(jtpost completion zsh)

To load completions for every new session, execute once:

#### Linux:

	jtpost completion zsh > "${fpath[1]}/_jtpost"

#### macOS:

	jtpost completion zsh > $(brew --prefix)/share/zsh/site-functions/_jtpost

You will need to start a new shell for this setup to take effect.


```
jtpost completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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

