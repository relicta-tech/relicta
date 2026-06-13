## relicta history risk

View risk patterns and trends

### Synopsis

View historical risk patterns and trends for the repository.

```
relicta history risk [flags]
```

### Options

```
  -h, --help   help for risk
```

### Options inherited from parent commands

```
      --allow-untrusted-plugins   load plugins on best-effort sandbox platforms; review 'relicta plugin sandbox-status' first
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
      --dry-run                   simulate actions without making changes
      --json                      output results as JSON
  -n, --limit int                 Number of entries to show (default 10)
      --log string                alias for --log-level
      --log-level string          log level (debug, info, warn, error) (default "info")
      --model string              AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-4, anthropic/claude-sonnet-4, local/mistral)
      --no-color                  disable colored output
      --redact                    redact secrets and API keys from output (auto-enabled in CI mode)
  -r, --repo string               Repository to show history for
  -v, --verbose                   enable verbose output
```

### SEE ALSO

* [relicta history](relicta_history.md)	 - View release history and CGP metrics

