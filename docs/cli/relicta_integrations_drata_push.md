## relicta integrations drata push

Push evidence to Drata

### Synopsis

Generate compliance evidence and push it to Drata as evidence artifacts.

Examples:

  # Push Article 12 log entries for Q1 2026
  relicta integrations drata push --period 2026-Q1 --type article12 --repo acme/payments

  # Push SOC 2 aggregated evidence
  DRATA_API_TOKEN=secret relicta integrations drata push --period 2026-Q1 --type soc2 --repo acme/payments

  # Dry-run: render JSON payloads without API calls
  relicta integrations drata push --period 2026-Q1 --type article12 --repo acme/payments --dry-run

```
relicta integrations drata push [flags]
```

### Options

```
      --base-url string       Drata API base URL (default: https://api.drata.com/public-api/v1)
      --dry-run               render evidence payloads without pushing to Drata
  -h, --help                  help for push
      --period string         reporting period (e.g. 2026-Q1)
      --repo string           repository identifier (system identifier)
      --token string          Drata API token (defaults to DRATA_API_TOKEN env)
      --type string           evidence type: article12, soc2 (default "article12")
      --workspace-id string   Drata workspace ID (defaults to DRATA_WORKSPACE_ID env)
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

* [relicta integrations drata](relicta_integrations_drata.md)	 - Drata evidence push integration

