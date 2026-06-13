## relicta verify

Verify governance attestation for a release

### Synopsis

Verify the governance attestation attached to a release.

This command validates the cryptographic signature and governance
constraints of an in-toto attestation generated during publish.

Examples:
  # Verify the attestation for a specific run
  relicta verify --run-id abc123

  # Verify an attestation file directly
  relicta verify --file .relicta/releases/abc123/attestation.intoto.jsonl

  # Verify allowing unsigned attestations
  relicta verify --run-id abc123 --allow-unsigned

  # Verify with governance constraints
  relicta verify --file att.jsonl --max-risk-score 0.5 --min-approvals 2

```
relicta verify [flags]
```

### Options

```
      --allow-unsigned         accept unsigned attestations
      --file string            path to attestation file
  -h, --help                   help for verify
      --max-risk-score float   maximum allowed risk score (0 = no limit)
      --min-approvals int      minimum required approvals (0 = no minimum)
      --public-key string      path to public key PEM file for signature verification
      --run-id string          release run ID to verify
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

