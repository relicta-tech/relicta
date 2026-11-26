## relicta cgp

Inspect Change Governance Protocol records

### Synopsis

Inspect the CGP records left by AI agents and other protocol clients.

The cgp_propose, cgp_authorize and cgp_status MCP tools record a governance
handshake for each proposed change. These commands read those records, so a
person can audit what was proposed and what was decided without going through
an agent.

Records are kept indefinitely. Nothing prunes .relicta/cgp/, and that is the
decision rather than an omission: these are the evidence that a change was
governed, and an audit trail that expires cannot answer a question asked after
it expired. Removing records is therefore a deliberate act — delete the files —
and 'relicta cgp list' reports how many are held so the growth is visible
instead of silent.

Examples:
  # What has been proposed in this repository?
  relicta cgp list

  # The full handshake for one proposal
  relicta cgp status prop_d419e938-a1b

### Options

```
  -h, --help   help for cgp
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
* [relicta cgp list](relicta_cgp_list.md)	 - List recorded CGP proposals, newest first
* [relicta cgp status](relicta_cgp_status.md)	 - Show the recorded governance state of a proposal

