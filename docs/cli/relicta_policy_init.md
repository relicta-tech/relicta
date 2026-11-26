## relicta policy init

Write a starting policy

### Synopsis

Write one of the example policies included in this binary into your
policy directory, so you have working DSL to edit rather than a blank file.

Governance runs on built-in defaults without any policy file. A policy is how
you encode rules those defaults do not cover — a longer freeze window, stricter
handling of agent-authored changes, a team that reviews its own releases.

The file is written to the first directory relicta searches, so it takes effect
immediately. Use --list to see what is available and --template to choose.

```
relicta policy init [flags]
```

### Options

```
  -d, --dir string        directory to write into (default: the first directory relicta searches)
      --force             overwrite an existing policy file
  -h, --help              help for init
      --list              list the policy templates included in this binary
  -t, --template string   which included policy to write (see --list) (default "starter")
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

* [relicta policy](relicta_policy.md)	 - Manage governance policies

