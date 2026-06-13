## relicta policy scaffold

Scaffold policy test fixtures

### Synopsis

Generate starter fixtures for policy testing.

The command creates:
  - a single input fixture (policy-input.json by default)
  - a matrix fixture with low/high risk seeds and per-rule candidate scenarios

Use generated fixtures with 'relicta policy test --input/--matrix' and
iterate by refining scenario expectations.

```
relicta policy scaffold [flags]
```

### Options

```
  -d, --dir string               directory containing policy files
  -f, --file string              single policy file to inspect
      --force                    overwrite output files if they already exist
  -h, --help                     help for scaffold
      --input-out string         output path for single-input fixture (.json or .yaml) (default "policy-input.json")
      --matrix-out string        output path for matrix fixture (.json or .yaml) (default "policy-matrix.yaml")
      --max-rule-scenarios int   maximum number of per-rule scenarios to include in matrix (default 8)
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

