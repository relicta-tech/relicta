## relicta deploy

Record and inspect deployments of released versions

### Synopsis

Record that a released version reached an environment, and read what did.

A release is a tag being published; a deployment is a change reaching an
environment. Relicta needs both to answer "what is running, and was it governed",
and DORA metrics need deployments rather than releases to mean anything.

Environments must be declared in configuration under 'environments', with exactly
one marked production.

Examples:
  # From a pipeline step, after the deploy succeeded
  relicta deploy record --env production --version 1.2.0

  # A failed deployment is worth recording — change failure rate is computed from these
  relicta deploy record --env production --version 1.2.0 --outcome failed

  # What reached where
  relicta deploy list
  relicta deploy list --env production

### Options

```
  -h, --help   help for deploy
```

### Options inherited from parent commands

```
      --allow-untrusted-plugins   load enabled plugins (unsigned; review 'relicta plugin sandbox-status' first)
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
      --dry-run                   simulate actions without making changes
      --json                      output results as JSON
      --log string                alias for --log-level
      --log-level string          log level (debug, info, warn, error) (default "info")
      --model string              AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-6.2-sol, anthropic/claude-sonnet-4-6, local/mistral)
      --no-color                  disable colored output
      --redact                    redact secrets and API keys from output (auto-enabled in CI mode)
  -v, --verbose                   enable verbose output
```

### SEE ALSO

* [relicta](relicta.md)	 - The governance layer for software change
* [relicta deploy audit](relicta_deploy_audit.md)	 - Compare what is deployed against what was governed
* [relicta deploy list](relicta_deploy_list.md)	 - List recorded deployments, newest first
* [relicta deploy record](relicta_deploy_record.md)	 - Record a deployment of a version to an environment

