## relicta plugin

Manage Relicta plugins

### Synopsis

Manage plugins for Relicta.

Plugins extend Relicta's functionality for version control systems,
package managers, notification services, and more.

Examples:
  # List available plugins
  relicta plugin list --available

  # Install a plugin
  relicta plugin install github

  # Configure a plugin interactively
  relicta plugin configure github

  # Update a plugin
  relicta plugin update github

  # Get plugin information
  relicta plugin info github

### Options

```
  -h, --help   help for plugin
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

* [relicta](relicta.md)	 - The governance layer for software change
* [relicta plugin configure](relicta_plugin_configure.md)	 - Interactively configure a plugin
* [relicta plugin create](relicta_plugin_create.md)	 - Create a new plugin from template
* [relicta plugin dev](relicta_plugin_dev.md)	 - Run a plugin in development mode
* [relicta plugin disable](relicta_plugin_disable.md)	 - Disable a plugin
* [relicta plugin enable](relicta_plugin_enable.md)	 - Enable a plugin
* [relicta plugin info](relicta_plugin_info.md)	 - Show detailed information about a plugin
* [relicta plugin install](relicta_plugin_install.md)	 - Install a plugin (or all required plugins)
* [relicta plugin list](relicta_plugin_list.md)	 - List plugins
* [relicta plugin registry](relicta_plugin_registry.md)	 - Manage plugin registries
* [relicta plugin sandbox-status](relicta_plugin_sandbox-status.md)	 - Show plugin sandbox enforcement posture
* [relicta plugin search](relicta_plugin_search.md)	 - Search for plugins
* [relicta plugin uninstall](relicta_plugin_uninstall.md)	 - Uninstall a plugin
* [relicta plugin update](relicta_plugin_update.md)	 - Update a plugin to the latest version

