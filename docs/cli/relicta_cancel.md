## relicta cancel

Cancel the current release

### Synopsis

Cancel the current in-progress release.

This command cancels a release that is in progress, allowing you to
start fresh with a new release cycle. Use this when:

  • You need to abort a release before publishing
  • The release has issues that require starting over
  • You want to discard the current release plan

After canceling, you can run 'relicta reset' to prepare for a new release,
or simply run 'relicta plan' to start a fresh release cycle.

Note: You cannot cancel a release that is currently being published.
To handle a failed publish, use 'relicta reset' instead.

```
relicta cancel [flags]
```

### Options

```
  -f, --force           force cancel even if in publishing state (not recommended)
  -h, --help            help for cancel
  -r, --reason string   reason for canceling the release
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

