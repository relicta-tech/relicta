## relicta health

Check the health of relicta and its dependencies

### Synopsis

Perform health checks on relicta and its dependencies.

This command verifies:
  - Git availability and repository status
  - Configuration validity
  - Plugin connectivity
  - AI service availability (if enabled)

Exit codes:
  0 - All checks passed (healthy)
  1 - Some non-critical checks failed (degraded)
  2 - Critical checks failed (unhealthy)

```
relicta health [flags]
```

### Options

```
  -h, --help   help for health
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

