## relicta group plan

Show release plan across repository group

### Synopsis

Analyze all repositories in a group and show which ones
have unreleased changes and what versions they would receive.

For coordinated groups, the plan shows dependency-ordered release sequence. With --repo,
coordinated groups include every transitive dependency of the selected repositories; independent
groups include only the selected repositories. Unknown repository names are errors.

```
relicta group plan [flags]
```

### Options

```
  -h, --help   help for plan
```

### Options inherited from parent commands

```
      --allow-untrusted-plugins   load enabled plugins (unsigned; review 'relicta plugin sandbox-status' first)
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
      --dry-run                   simulate actions without making changes
      --group string              repository group name (required)
      --json                      output results as JSON
      --log string                alias for --log-level
      --log-level string          log level (debug, info, warn, error) (default "info")
      --model string              AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-6.2-sol, anthropic/claude-sonnet-4-6, local/mistral)
      --no-color                  disable colored output
      --redact                    redact secrets and API keys from output (auto-enabled in CI mode)
      --repo strings              target specific repos within the group
  -v, --verbose                   enable verbose output
```

### SEE ALSO

* [relicta group](relicta_group.md)	 - Multi-repository governance commands

