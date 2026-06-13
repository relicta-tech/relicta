## relicta mcp serve

Start the MCP server

### Synopsis

Start the MCP server for AI agent communication.

The server uses stdio transport by default, allowing integration with
AI clients that support the Model Context Protocol (MCP). You can also
run over HTTP transport for remote/custom integrations.

Core Tools:
  - relicta.status:   Get current release state
  - relicta.init:     Initialize configuration file
  - relicta.plan:     Analyze commits and plan release
  - relicta.bump:     Calculate and set version
  - relicta.notes:    Generate release notes
  - relicta.evaluate: CGP risk evaluation
  - relicta.approve:  Approve the release
  - relicta.publish:  Execute the release

AI Agent Tools:
  - relicta.blast_radius:     Analyze monorepo change impact
  - relicta.infer_version:    Lightweight version inference
  - relicta.summarize_diff:   Audience-tailored change summaries
  - relicta.validate_release: Pre-flight release validation

Resources available:
  - relicta://state:       Current release state
  - relicta://config:      Configuration settings
  - relicta://commits:     Recent commits
  - relicta://changelog:   Generated changelog
  - relicta://risk-report: CGP risk assessment

```
relicta mcp serve [flags]
```

### Options

```
      --address string     address for HTTP transport, e.g. :8080 or 127.0.0.1:8080
  -h, --help               help for serve
      --port string        port for HTTP transport (implies --transport=http)
      --transport string   transport to use: stdio or http (default "stdio")
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

* [relicta mcp](relicta_mcp.md)	 - Model Context Protocol (MCP) server commands

