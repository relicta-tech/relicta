## relicta plugin dev

Run a plugin in development mode

### Synopsis

Run a plugin in development mode with optional file watching.

This command builds and installs a plugin from source for testing.
With --watch, it monitors for file changes and automatically rebuilds.

Examples:
  # Build and install plugin from current directory
  relicta plugin dev

  # Build and install plugin from specific path
  relicta plugin dev ./my-plugin

  # Watch for changes and auto-rebuild
  relicta plugin dev --watch

  # Specify output name
  relicta plugin dev --name my-custom-plugin

```
relicta plugin dev [plugin-path] [flags]
```

### Options

```
  -h, --help          help for dev
  -n, --name string   Plugin name (defaults to directory name)
  -w, --watch         Watch for file changes and auto-rebuild
```

### Options inherited from parent commands

```
      --allow-untrusted-plugins   load plugins on best-effort sandbox platforms; review 'relicta plugin sandbox-status' first
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
      --dry-run                   simulate actions without making changes
      --json                      output results as JSON
      --log string                alias for --log-level
      --log-level string          log level (debug, info, warn, error) (default "info")
      --model string              AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-4, anthropic/claude-sonnet-4, local/mistral)
      --no-color                  disable colored output
      --redact                    redact secrets and API keys from output (auto-enabled in CI mode)
  -v, --verbose                   enable verbose output
```

### SEE ALSO

* [relicta plugin](relicta_plugin.md)	 - Manage Relicta plugins

