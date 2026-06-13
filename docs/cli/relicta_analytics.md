## relicta analytics

Show governance analytics (risk trends, decisions, team metrics)

### Synopsis

Surface the governance analytics captured during plan/approve/publish:
risk-score trends over time, the distribution of policy decisions, and
per-actor approval/release metrics.

Analytics are captured automatically when governance is enabled; this
command aggregates the stored events.

Examples:
  relicta analytics                       # all views, weekly buckets
  relicta analytics --view risk -g day    # daily risk trend only
  relicta analytics --json                # machine-readable output

```
relicta analytics [flags]
```

### Options

```
  -g, --granularity string   time bucket for trends: day | week | month (default "week")
  -h, --help                 help for analytics
  -w, --view string          which view to show: risk | decisions | team | all (default "all")
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

