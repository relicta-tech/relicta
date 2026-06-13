## relicta group release

Coordinated release across repository group

### Synopsis

Execute releases across all repositories in a group.

For coordinated strategy, repositories are released in dependency order.
If a repository fails, coordinated releases stop to prevent inconsistency.
For independent strategy, failures in one repo do not affect others.

```
relicta group release [flags]
```

### Options

```
  -h, --help   help for release
```

### Options inherited from parent commands

```
      --allow-untrusted-plugins   load plugins on best-effort sandbox platforms; review 'relicta plugin sandbox-status' first
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
      --dry-run                   simulate actions without making changes
      --group string              repository group name (required)
      --json                      output results as JSON
      --log string                alias for --log-level
      --log-level string          log level (debug, info, warn, error) (default "info")
      --model string              AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-4, anthropic/claude-sonnet-4, local/mistral)
      --no-color                  disable colored output
      --redact                    redact secrets and API keys from output (auto-enabled in CI mode)
      --repo strings              target specific repos within the group
  -v, --verbose                   enable verbose output
```

### SEE ALSO

* [relicta group](relicta_group.md)	 - Multi-repository governance commands

