## relicta eval

AI evaluation harness (model regression gate)

### Synopsis

Run the Relicta AI evaluation harness against a corpus of golden cases.

The harness is a non-negotiable gate for any model bump on a governance product.
Without it, prompt drift on a new model silently degrades release-notes /
risk-narrative quality between deployments.

V0 ships with deterministic heuristic scoring (no API keys needed) and a
small embedded golden corpus (release_notes, risk_narrative, communicate).
Real LLM-as-judge scoring is opt-in via build tag in a follow-up.

### Options

```
  -h, --help   help for eval
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
* [relicta eval list](relicta_eval_list.md)	 - List embedded golden cases
* [relicta eval run](relicta_eval_run.md)	 - Run AI evals against the configured AI service

