## relicta release

Run the complete release workflow

### Synopsis

Run the complete release workflow in one command.

This is equivalent to running:
  relicta plan → bump → notes → approve → publish

By default, the command runs interactively and prompts for approval.
Use --yes to auto-approve for CI/CD pipelines.

Examples:
  # Interactive release (prompts for approval)
  relicta release

  # Auto-approve for CI/CD
  relicta release --yes

  # Dry run to preview changes
  relicta release --dry-run

  # Force a specific version
  relicta release --force v2.0.0

  # Clear any stale release state before starting
  relicta release --clean

```
relicta release [flags]
```

### Options

```
      --channel string   release channel (stable, canary, alpha, beta, next)
  -x, --clean            clear any active release state before starting
  -f, --force string     force a specific version (e.g., v2.0.0)
  -h, --help             help for release
      --skip-push        skip pushing to remote
  -y, --yes              auto-approve the release without prompting
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

