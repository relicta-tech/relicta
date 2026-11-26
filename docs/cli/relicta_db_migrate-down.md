## relicta db migrate-down

Roll back the most recently applied migration

```
relicta db migrate-down [flags]
```

### Options

```
  -h, --help   help for migrate-down
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

* [relicta db](relicta_db.md)	 - Manage the database backing relicta's release history

