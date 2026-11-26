## relicta hub login

Obtain a Hub token by approving this device in a browser

### Synopsis

Authorize this machine against a Relicta Hub.

Prints a short code and a URL. Open the URL, confirm the code matches, and approve — this
command waits and stores the token it receives.

The password is entered in the browser, never here: this process sees only a one-time code and
the token it is exchanged for. That is the point of the device authorization grant (RFC 8628),
and it is what makes the flow usable on a machine with no browser of its own, such as a CI
runner or a server over SSH.

Examples:
  relicta hub login
  relicta hub login --hub https://hub.example.com
  RELICTA_HUB_URL=https://hub.example.com relicta hub login

```
relicta hub login [flags]
```

### Options

```
  -h, --help         help for login
      --hub string   Hub base URL (defaults to $RELICTA_HUB_URL)
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

* [relicta hub](relicta_hub.md)	 - Authenticate against a Relicta Hub

