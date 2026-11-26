## relicta hub sync

Push this repository's governance record to Hub

### Synopsis

Send local release history to a Relicta Hub.

Reads the governance record this repository already keeps — the same store behind `relicta history` —
and pushes it to Hub, which aggregates it into the org-wide release view, DORA metrics and actor
reputation.

Safe to run repeatedly. Event identifiers are derived from the release records rather than
generated, and Hub ignores an identifier it has already stored, so syncing twice converges on the
same rows instead of duplicating them.

Requires a token from `relicta hub login`.

Examples:
  relicta hub sync
  relicta hub sync --dry-run
  relicta hub sync --limit 50

```
relicta hub sync [flags]
```

### Options

```
      --dry-run     Show what would be sent without sending it
  -h, --help        help for sync
      --limit int   Maximum number of release records to send (default 200)
```

### Options inherited from parent commands

```
      --allow-untrusted-plugins   load enabled plugins (unsigned; review 'relicta plugin sandbox-status' first)
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
      --json                      output results as JSON
      --log string                alias for --log-level
      --log-level string          log level (debug, info, warn, error) (default "info")
      --model string              AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-6.2-sol, anthropic/claude-sonnet-4-6, local/mistral)
      --no-color                  disable colored output
      --redact                    redact secrets and API keys from output (auto-enabled in CI mode)
  -v, --verbose                   enable verbose output
```

### SEE ALSO

* [relicta hub](relicta_hub.md)	 - Authenticate against a Relicta Hub

