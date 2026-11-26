## relicta deploy audit

Compare what is deployed against what was governed

### Synopsis

Compare deployment records against release records.

Two discrepancies, and they are not equally serious:

  ungoverned deployment  a version is running with no relicta release behind it —
                         something reached an environment without passing through
                         governance
  undeployed release     a governed release never reached production

Exits non-zero when an ungoverned deployment is found, so a pipeline can gate on
it. An undeployed release is reported but does not fail the command: a release
awaiting rollout is a normal state, and failing on it would train people to ignore
the command — at which point it stops reporting the serious case too. Use --strict
to fail on both.

```
relicta deploy audit [flags]
```

### Options

```
  -h, --help     help for audit
      --strict   also fail when a governed release never reached production
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

* [relicta deploy](relicta_deploy.md)	 - Record and inspect deployments of released versions

