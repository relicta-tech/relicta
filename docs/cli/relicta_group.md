## relicta group

Multi-repository governance commands

### Synopsis

Manage coordinated releases across multiple repositories.

Repository groups are defined in .relicta.yaml under 'repository_groups'.
Each group contains repositories with optional dependency relationships.

Supported strategies:
  - independent: Each repo is released separately
  - coordinated: Repos are released in dependency order

### Options

```
      --group string   repository group name (required)
  -h, --help           help for group
      --repo strings   target specific repos within the group
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
* [relicta group plan](relicta_group_plan.md)	 - Show release plan across repository group
* [relicta group release](relicta_group_release.md)	 - Coordinated release across repository group
* [relicta group status](relicta_group_status.md)	 - Show release state of all repositories in group

