## relicta bump

Calculate and apply a version bump

### Synopsis

Calculate the next version based on commits and apply the bump.

This command updates version tags and optionally version files.

```
relicta bump [flags]
```

### Options

```
  -b, --build string        build metadata
      --channel string      release channel (stable, canary, alpha, beta, next)
      --force string        set a specific version (e.g., 2.0.0), bypasses commit analysis
  -h, --help                help for bump
  -l, --level string        bump level (major, minor, patch) - overrides auto-detection
  -p, --prerelease string   prerelease identifier (e.g., alpha, beta, rc.1)
      --version string      alias for --force: set a specific version
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

