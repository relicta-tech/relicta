## relicta reset

Reset a failed or canceled release

### Synopsis

Reset a release to allow starting fresh.

This command resets a release that has failed or been canceled,
clearing the error state and preparing for a new release attempt.

Use this when:
  • A publish operation failed and you want to retry
  • You canceled a release and want to start over
  • The release state is stuck and needs to be cleared

After resetting, run 'relicta plan' to start a new release cycle.

```
relicta reset [flags]
```

### Options

```
  -f, --force   force reset even if release is in progress
  -h, --help    help for reset
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

