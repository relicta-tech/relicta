## relicta metrics

Start a metrics server for monitoring

### Synopsis

Start an HTTP server exposing Prometheus-compatible metrics.

The metrics server provides visibility into:
  - Release operations (total, successful, failed)
  - Plugin executions and errors
  - Command invocations and latency
  - Active release count

Example:
  # Start metrics server on default port 9090
  relicta metrics

  # Start on custom port
  relicta metrics --port 8080

  # Allow remote scraping only when the network is appropriately restricted
  relicta metrics --host 0.0.0.0 --port 9090

The server binds to 127.0.0.1 by default, so its unauthenticated metrics are only
available on the local machine. To allow remote scraping, set --host to a private
interface or 0.0.0.0 and restrict access with firewall rules or an authenticated
proxy.

```
relicta metrics [flags]
```

### Options

```
  -h, --help          help for metrics
  -H, --host string   Host to bind to (default "127.0.0.1")
  -p, --port int      Port to listen on (default 9090)
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

