## relicta publish

Execute the release

### Synopsis

Execute the release by creating tags, updating changelog, and
running configured plugins.

This command performs all the release actions including:
- Creating and pushing git tags
- Updating the changelog file
- Running plugins (GitHub release, npm publish, Slack notification)

```
relicta publish [flags]
```

### Options

```
  -h, --help            help for publish
  -A, --skip-approval   skip approval check
  -G, --skip-plugins    skip running plugins
  -P, --skip-push       skip pushing to remote
  -T, --skip-tag        skip git tag creation
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

