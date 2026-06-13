## relicta promote

Promote a release from one channel to another

### Synopsis

Promote a release from one channel to another.

Promotion creates a new tag on the same commit with the target channel's
prerelease identifier. For example:
  v1.2.0-canary.1 -> v1.2.0-alpha.1 -> v1.2.0-beta.1 -> v1.2.0-rc.1 -> v1.2.0

Available channels (ordered by stability):
  canary  - Bleeding-edge releases
  alpha   - Early development releases
  beta    - Feature-complete but potentially unstable
  next    - Release candidates (maps to -rc.N suffix)
  stable  - Production releases (no prerelease suffix)

Examples:
  # Promote latest canary to alpha
  relicta promote --from canary --to alpha

  # Promote a specific version to stable
  relicta promote --from beta --to stable --version v1.2.0-beta.3

  # Promote from next (rc) to stable
  relicta promote --from next --to stable

  # Dry run to preview the promotion
  relicta promote --from alpha --to beta --dry-run

```
relicta promote [flags]
```

### Options

```
      --from string      source channel (required)
  -h, --help             help for promote
      --to string        target channel (required)
      --version string   specific version to promote (default: latest on source channel)
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

