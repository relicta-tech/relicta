## relicta plugin sandbox-status

Show plugin sandbox enforcement posture

### Synopsis

Show the current plugin sandbox enforcement posture for this platform.

Displays whether memory / CPU limits are reliably enforced, whether plugin
signatures are verified before load, and any platform-specific caveats
(e.g. Apple Silicon RLIMIT_AS limitations).

Use this before relying on plugin sandboxing as a security boundary.
Plugin loading on best-effort platforms requires --allow-untrusted-plugins.

```
relicta plugin sandbox-status [flags]
```

### Options

```
  -h, --help   help for sandbox-status
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

* [relicta plugin](relicta_plugin.md)	 - Manage Relicta plugins

