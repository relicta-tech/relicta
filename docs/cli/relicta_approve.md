## relicta approve

Review and approve the release

### Synopsis

Review the prepared release and approve it for publishing.

This command presents the release summary and allows you to
review and edit the release notes before publishing.

```
relicta approve [flags]
```

### Options

```
      --actor-id string       identity reported by a trusted external approval system (requires --ci, --actor-type, and --reason)
      --actor-type string     actor type reported by a trusted external approval system (human, ci, agent; requires --ci and --actor-id)
  -e, --edit                  edit release notes before approving
  -E, --editor string         editor to use (default: $EDITOR or vim)
  -h, --help                  help for approve
  -i, --interactive           use interactive TUI for approval
      --override-governance   approve despite governance requiring human review; requires --reason and is recorded as an override
      --package string        approve one package of a monorepo, by name or path (each package carries its own decision)
      --reason string         why governance is overridden, or the source for a trusted external approval
  -y, --yes                   automatically approve without prompting
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

* [relicta](relicta.md)	 - The governance layer for software change

