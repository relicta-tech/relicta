## relicta communicate

Generate audience-specific release narratives

### Synopsis

Generate release communication tailored to different audiences.

Produces audience-specific narratives from the current release changes.
Supports engineering, product, executive, and external audiences.

Examples:
  relicta communicate --audiences all
  relicta communicate --audiences engineering,product --format html
  relicta communicate --audiences executive --output-dir ./release-comms

```
relicta communicate [flags]
```

### Options

```
      --audiences string    comma-separated audiences or 'all' (engineering,product,executive,external) (default "all")
      --format string       output format (markdown, plaintext, html) (default "markdown")
  -h, --help                help for communicate
  -o, --output-dir string   directory to write audience-specific files (default: stdout)
      --product string      product name for branding
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

