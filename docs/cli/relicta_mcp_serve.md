## relicta mcp serve

Start the MCP server

### Synopsis

Start the MCP server for AI agent communication.

The server uses stdio transport by default, allowing integration with
AI clients that support the Model Context Protocol (MCP). You can also
run over HTTP transport for remote/custom integrations. HTTP binds to
loopback by default and has no built-in authentication or TLS; non-loopback
binds require --allow-insecure-http and external network/authentication controls.

Core Tools:
  - relicta_status:   Get current release state
  - relicta_init:     Initialize configuration file
  - relicta_plan:     Analyze commits and plan release
  - relicta_bump:     Calculate and set version
  - relicta_notes:    Generate release notes
  - relicta_evaluate: CGP risk evaluation
  - relicta_approve:  Approve the release
  - relicta_publish:  Execute the release

AI Agent Tools:
  - relicta_blast_radius:     Analyze monorepo change impact
  - relicta_infer_version:    Lightweight version inference
  - relicta_summarize_diff:   Audience-tailored change summaries
  - relicta_validate_release: Pre-flight release validation

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
      --address string        address for HTTP transport, e.g. :8080 or 127.0.0.1:8080
      --allow-insecure-http   allow MCP HTTP to bind non-loopback without built-in authentication; protect it with a trusted proxy and network policy
  -h, --help                  help for serve
      --port string           port for HTTP transport (implies --transport=http)
      --transport string      transport to use: stdio or http (default "stdio")
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

* [relicta mcp](relicta_mcp.md)	 - Model Context Protocol (MCP) server commands

