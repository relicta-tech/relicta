## relicta history

View release history and CGP metrics

### Synopsis

View historical release data and CGP (Change Governance Protocol) metrics.

This command provides insights into:
  - Past releases and their outcomes
  - Actor reliability scores
  - Risk patterns and trends

Examples:
  # View recent release history
  relicta history

  # View history for a specific repository
  relicta history --repo owner/repo

  # View more history entries
  relicta history --limit 20

  # View risk patterns and trends
  relicta history --risk

  # View metrics for a specific actor
  relicta history --actor human:developer-name

  # Output as JSON
  relicta history --json

```
relicta history [flags]
```

### Options

```
  -h, --help          help for history
  -n, --limit int     Number of entries to show (default 10)
  -r, --repo string   Repository to show history for
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
* [relicta history actor](relicta_history_actor.md)	 - View actor metrics
* [relicta history releases](relicta_history_releases.md)	 - View release history
* [relicta history risk](relicta_history_risk.md)	 - View risk patterns and trends

