## relicta integrations

Third-party integrations (Vanta, Drata)

### Synopsis

Push Relicta governance evidence to external compliance platforms.

Currently supported:

  vanta   Vanta REST API — push release-governance evidence mapped to
          Article 12 concepts and SOC 2 evidence as custom artifacts. This
          Article 12 mapping is not an operational AI-use log. Vanta released
          its remote MCP server and Claude plugin in April 2026; Relicta
          complements Vanta with upstream release-governance evidence.
  drata   Drata REST API — same release-governance evidence shape, different
          transport. Use
          when your org runs Drata for SOC 2 / ISO 27001 / HIPAA / PCI
          continuous monitoring.

### Options

```
  -h, --help   help for integrations
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
* [relicta integrations drata](relicta_integrations_drata.md)	 - Drata evidence push integration
* [relicta integrations vanta](relicta_integrations_vanta.md)	 - Vanta evidence push integration

