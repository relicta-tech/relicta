## relicta plugin registry

Manage plugin registries

### Synopsis

Manage plugin registries for discovering and installing plugins.

Relicta supports multiple plugin registries. The official registry is always
enabled and takes precedence. Community registries can be added to discover
third-party plugins.

Examples:
  # List all configured registries
  relicta plugin registry list

  # Add a community registry
  relicta plugin registry add awesome-plugins https://example.com/registry.yaml

  # Remove a registry
  relicta plugin registry remove awesome-plugins

  # Disable a registry temporarily
  relicta plugin registry disable awesome-plugins

### Options

```
  -h, --help   help for registry
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
* [relicta plugin registry add](relicta_plugin_registry_add.md)	 - Add a new registry
* [relicta plugin registry disable](relicta_plugin_registry_disable.md)	 - Disable a registry
* [relicta plugin registry enable](relicta_plugin_registry_enable.md)	 - Enable a disabled registry
* [relicta plugin registry generate-index](relicta_plugin_registry_generate-index.md)	 - Generate the v2 JSON registry index from registry.yaml
* [relicta plugin registry list](relicta_plugin_registry_list.md)	 - List configured registries
* [relicta plugin registry remove](relicta_plugin_registry_remove.md)	 - Remove a registry

