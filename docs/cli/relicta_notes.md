## relicta notes

Generate changelog and release notes

### Synopsis

Generate changelog entries and release notes for the current release.

By default, this command creates deterministic release documentation from your
commit history. To use AI-generated prose, enable ai.enabled and configure a
provider in .relicta.yaml. Audience, tone, and language options apply only to
AI-generated content. With MCP, a calling agent can instead supply customer-facing
notes using product context from its conversation. Use --draft-file to persist a
reviewed draft from a version-controlled file, including in CI release workflows.
Review all claims before approval.

```
relicta notes [flags]
```

### Options

```
  -a, --audience string     target audience (developers, users, public, stakeholders)
      --draft-file string   read a customer-facing draft from a file (relative paths use the repository root)
      --emoji               include emojis in output
  -h, --help                help for notes
  -l, --language string     output language (default "English")
  -o, --output string       output file (default: stdout)
  -t, --tone string         AI tone (technical, friendly, professional, marketing)
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

