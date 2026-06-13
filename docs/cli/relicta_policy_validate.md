## relicta policy validate

Validate policy files

### Synopsis

Validate policy DSL files for syntax and semantic correctness.

By default, searches for .policy and .cgp files in:
  - .relicta/policies/
  - .github/relicta/policies/
  - policies/

Use --dir to specify a custom directory or --file to validate a single file.

```
relicta policy validate [flags]
```

### Options

```
  -d, --dir string    directory containing policy files
  -f, --file string   specific policy file to validate
  -h, --help          help for validate
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

* [relicta policy](relicta_policy.md)	 - Manage governance policies

