## relicta serve

Start the dashboard web server

### Synopsis

Start the self-hosted dashboard web server.

The dashboard provides a web UI for:
  - Release pipeline visualization
  - Governance analytics and risk trends
  - Team performance metrics
  - Approval workflow management
  - Audit trail exploration

The server can be configured via:
  - Command-line flags (--port, --address)
  - Configuration file (dashboard section)
  - Environment variables (RELICTA_DASHBOARD_*)

Examples:
  # Start on default port 8080
  relicta serve

  # Start on custom port
  relicta serve --port 3000

  # Start on specific address
  relicta serve --address localhost:9000

Authentication:
  Authentication mode is configured in .relicta.yaml.
  For production, use API key authentication:

    dashboard:
      enabled: true
      auth:
        mode: api_key
        api_keys:
          - key: ${RELICTA_DASHBOARD_KEY}
            name: "Admin"
            roles: ["admin"]

```
relicta serve [flags]
```

### Options

```
  -a, --address string   Address to listen on (e.g., localhost:8080)
  -k, --api-key string   API key for dashboard authentication (enables API key mode)
  -h, --help             help for serve
  -n, --no-auth          Disable authentication (not recommended for production)
  -p, --port string      Port to listen on (default: 8080)
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

