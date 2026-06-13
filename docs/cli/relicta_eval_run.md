## relicta eval run

Run AI evals against the configured AI service

### Synopsis

Drive the configured AI service against the golden corpus and score outputs.

Examples:

  # Run all goldens with deterministic scoring (no API needed for judge)
  relicta eval run

  # Run only risk-narrative cases
  relicta eval run --category risk_narrative

  # JSON output for CI ingestion
  relicta eval run --json

  # Stop at first failure (pre-commit use)
  relicta eval run --fail-fast

```
relicta eval run [flags]
```

### Options

```
      --category string    filter by category (release_notes, risk_narrative, communicate, summarize_diff)
      --fail-fast          stop the run on the first failed verdict
      --goldens string     path to a goldens directory; defaults to embedded corpus
  -h, --help               help for run
      --judge string       scoring judge: deterministic (no API calls) | mock (test only) (default "deterministic")
      --timeout duration   per-golden timeout (default 30s)
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

* [relicta eval](relicta_eval.md)	 - AI evaluation harness (model regression gate)

