## relicta plugin sandbox-status

Show plugin sandbox enforcement posture

### Synopsis

Show the current plugin sandbox enforcement posture for this platform.

Displays memory and CPU enforcement limits, plugin signature status, and
platform-specific caveats (e.g. Apple Silicon RLIMIT_AS limitations). Plugin
signatures are not currently verified on any platform.

Use this before relying on plugin sandboxing as a security boundary.
Loading any enabled plugin requires --allow-untrusted-plugins. The flag records
an operator trust decision; it does not verify the publisher or provide a
complete security sandbox.

```
relicta plugin sandbox-status [flags]
```

### Options

```
  -h, --help   help for sandbox-status
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

* [relicta plugin](relicta_plugin.md)	 - Manage Relicta plugins

