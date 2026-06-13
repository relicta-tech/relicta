## relicta rollback

Roll back to a previous release version

### Synopsis

Roll back to a previous release version by creating a revert tag.

This command validates that the target version exists as a git tag,
creates a new tag pointing to the same commit as the target version,
and records the rollback event in the audit trail.

Examples:
  relicta rollback --to-version 1.2.3
  relicta rollback --to-tag v1.2.3
  relicta rollback --to-version 1.2.3 --dry-run

```
relicta rollback [flags]
```

### Options

```
      --dry-run             simulate the rollback without making changes
  -h, --help                help for rollback
      --to-tag string       target git tag to roll back to (e.g., v1.2.3)
      --to-version string   target version to roll back to (e.g., 1.2.3)
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

