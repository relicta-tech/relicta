## relicta server

Start the dashboard API server with deployment mode control

### Synopsis

Start the dashboard server with explicit control over the deployment mode.

This is an enhanced alias for 'relicta serve' that adds deployment mode flags
for separating the frontend from the backend API.

Modes:
  embedded  (default) Serves the embedded frontend alongside the API.
  api       API-only mode — no frontend is served. Set CORS origins
            to allow an external frontend to connect.

Examples:
  # Embedded mode (same as 'relicta serve')
  relicta server

  # API-only mode for standalone frontend deployment
  relicta server --mode api --allowed-origins "http://localhost:5173,https://dashboard.example.com"

  # Using environment variables
  RELICTA_SERVER_MODE=api RELICTA_ALLOWED_ORIGINS="https://dashboard.example.com" relicta server

Authentication:
  All authentication modes from 'relicta serve' are supported.
  See 'relicta serve --help' for details.

```
relicta server [flags]
```

### Options

```
  -a, --address string           Address to listen on (e.g., localhost:8080)
      --allowed-origins string   Comma-separated CORS allowed origins (e.g., http://localhost:5173)
  -k, --api-key string           API key for dashboard authentication
  -h, --help                     help for server
      --mode string              Server mode: embedded (default) or api
  -n, --no-auth                  Disable authentication (not recommended for production)
  -p, --port string              Port to listen on (default: 8080)
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

