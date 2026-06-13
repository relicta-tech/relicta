## relicta clean

Clean up old release runs

### Synopsis

Clean up old or stale release runs from the .relicta/releases directory.

This command helps manage disk space and reduce clutter by removing
old release runs that are no longer needed. By default, it keeps the
last 10 runs and removes older ones.

Examples:
  relicta clean                     # Keep last 10 runs, remove others
  relicta clean --keep 5            # Keep last 5 runs
  relicta clean --older-than 30d    # Remove runs older than 30 days
  relicta clean --all               # Remove all except the latest
  relicta clean --dry-run           # Show what would be deleted

The command will never delete an active (in-progress) release run.
Use 'relicta cancel' to cancel an active release first.

```
relicta clean [flags]
```

### Options

```
  -a, --all                 remove all release runs except the latest
      --dry-run             show what would be deleted without deleting
  -h, --help                help for clean
  -k, --keep int            keep the last N release runs (default: 10) (default 10)
  -o, --older-than string   remove runs older than duration (e.g., 7d, 30d)
```

### Options inherited from parent commands

```
      --allow-untrusted-plugins   load plugins on best-effort sandbox platforms; review 'relicta plugin sandbox-status' first
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
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

