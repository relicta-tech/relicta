## relicta demo

Manage the local Docker demo environment

### Synopsis

Manage the local Docker demo environment used for product showcases.

Examples:
  relicta demo
  relicta demo --reset
  relicta demo --down
  relicta demo --file docker-compose.demo.yml

```
relicta demo [flags]
```

### Options

```
      --down          tear down demo environment
  -f, --file string   path to docker compose demo file (default "docker-compose.demo.yml")
  -h, --help          help for demo
      --reset         reset demo data by running 'down -v' before startup
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

