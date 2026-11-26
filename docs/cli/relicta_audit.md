## relicta audit

Show the complete governance record: what was proposed, decided and shipped

### Synopsis

Show one timeline over both governance records.

Relicta keeps two: the CGP protocol records that agents and other clients leave
behind (relicta cgp), and the release audit trail of runs that were planned and
published (relicta history). They describe different things, so they are stored
separately — but a reader asking what governed a particular change needs both.

Entries are joined on the version an authorization granted, so an agent's
proposal appears alongside the release that carried it out. Anything that did
not become a release is still listed, on its own.

Examples:
  # Everything, newest first
  relicta audit

  # What governed one version
  relicta audit --version 1.4.0

  # For a tool
  relicta audit --json

```
relicta audit [flags]
```

### Options

```
  -h, --help             help for audit
  -n, --limit int        maximum entries to show (default 20)
      --version string   show only the record for one version
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

