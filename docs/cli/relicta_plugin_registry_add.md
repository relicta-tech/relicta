## relicta plugin registry add

Add a new registry

### Synopsis

Add a new plugin registry.

The registry should be a YAML file with the same format as the official registry.
Priority determines the order (higher = checked first). Default priority is 100.

Example:
  relicta plugin registry add community https://example.com/plugins/registry.yaml
  relicta plugin registry add company https://internal.example.com/registry.yaml 500

```
relicta plugin registry add <name> <url> [priority] [flags]
```

### Options

```
  -h, --help   help for add
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

* [relicta plugin registry](relicta_plugin_registry.md)	 - Manage plugin registries

