## relicta init

Initialize a new relicta configuration

### Synopsis

Initialize a new relicta configuration in the current directory.

By default this is zero-config: relicta detects your project from its git
remote and manifests, writes a .relicta.yaml with sensible defaults, and
prints next steps — no prompts. Pass --guided for the 8-step interactive
setup wizard, or --force to overwrite an existing config.

```
relicta init [flags]
```

### Options

```
  -f, --force           overwrite existing config file
      --format string   config file format (yaml, json) (default "yaml")
      --guided          opt in to the 8-step interactive setup wizard
  -h, --help            help for init
  -i, --interactive     run the interactive wizard (deprecated alias for --guided)
      --quick           explicit quick mode (now the default): detect from git + manifests, write defaults, no prompts
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

