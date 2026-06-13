## relicta plan

Analyze changes and plan the next release

### Synopsis

Analyze commits since the last release and suggest a version bump.

This command examines your commit history using conventional commits
to determine what type of release is needed (major, minor, or patch).

```
relicta plan [flags]
```

### Options

```
  -a, --all                    show all commits including non-conventional
      --analyze                analyze commit classifications and stop
      --chronos-threads int    max concurrent Chronos ingest requests (overrides config)
  -f, --from string            starting reference (default: latest tag)
  -h, --help                   help for plan
      --min-confidence float   minimum confidence to accept classifications
  -m, --minimal                show minimal output
      --no-ai                  disable AI classification
  -r, --review                 review and adjust commit classifications before planning
      --skip-cognitive         skip Mnemos & Chronos cognitive backends
  -t, --to string              ending reference (default "HEAD")
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

