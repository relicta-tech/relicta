## relicta integrations vanta push

Push evidence to Vanta

### Synopsis

Generate compliance evidence and push it to Vanta as custom evidence.

Examples:

  # Push Article 12 log entries for Q1 2026 (one evidence record per entry)
  relicta integrations vanta push --period 2026-Q1 --type article12

  # Push SOC 2 aggregated evidence (3 records: change log, approvals, risks)
  VANTA_API_TOKEN=secret relicta integrations vanta push --period 2026-Q1 --type soc2

  # Dry-run: render JSON payloads without API calls
  relicta integrations vanta push --period 2026-Q1 --type article12 --dry-run

```
relicta integrations vanta push [flags]
```

### Options

```
      --base-url string   Vanta API base URL (default: https://api.vanta.com/v1)
      --dry-run           render evidence payloads without pushing to Vanta
  -h, --help              help for push
      --period string     reporting period (e.g. 2026-Q1)
      --repo string       repository identifier (system identifier)
      --token string      Vanta API token (defaults to VANTA_API_TOKEN env)
      --type string       evidence type: article12, soc2 (default "article12")
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

* [relicta integrations vanta](relicta_integrations_vanta.md)	 - Vanta evidence push integration

