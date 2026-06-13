## relicta plugin install

Install a plugin (or all required plugins)

### Synopsis

Install a plugin from the registry.

Downloads the plugin binary for your platform and makes it available
for use. Plugins must be enabled after installation with 'plugin enable'.

With no arguments, resolves and installs every plugin pinned in
plugin_security.required (ADR-008), skipping requirements that are
already satisfied:

  plugin_security:
    required:
      - github@^2.0
      - slack

```
relicta plugin install [name] [flags]
```

### Options

```
  -h, --help   help for install
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

