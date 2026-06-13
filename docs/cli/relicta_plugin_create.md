## relicta plugin create

Create a new plugin from template

### Synopsis

Create a new Relicta plugin project from a template.

This command scaffolds a complete plugin project with:
- Plugin implementation using the official SDK
- go.mod with correct dependencies
- Example configuration and hooks
- README with usage instructions

Examples:
  # Create a new plugin
  relicta plugin create my-notification

  # Create with specific hooks
  relicta plugin create my-plugin --hooks post-publish,on-success

  # Create in a specific directory
  relicta plugin create my-plugin --output ./plugins

```
relicta plugin create <name> [flags]
```

### Options

```
      --author string   Plugin author name
  -h, --help            help for create
      --hooks strings   Hooks the plugin responds to (default [post-publish])
      --module string   Go module path (default: github.com/yourname/relicta-plugin-<name>)
  -o, --output string   Output directory for the plugin (default ".")
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

