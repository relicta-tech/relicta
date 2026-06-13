## relicta status

Show the current release status

### Synopsis

Display information about the current release state.

This command shows:
  - Active release run (if any)
  - Current state in the release workflow
  - Version being released
  - Risk assessment (if available)

Examples:
  # Check current release status
  relicta status

  # Output as JSON
  relicta status --json

```
relicta status [flags]
```

### Options

```
  -h, --help   help for status
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

