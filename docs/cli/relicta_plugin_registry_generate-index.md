## relicta plugin registry generate-index

Generate the v2 JSON registry index from registry.yaml

### Synopsis

Generate the published registry index (schema_version 2) from the
legacy registry.yaml source (ADR-008).

The index is a static JSON document with per-plugin version lists and
per-artifact sha256 digests plus Cosign verification metadata. It is what
clients fetch; registry.yaml remains the source it is generated from.

Example:
  relicta plugin registry generate-index --input plugins/registry.yaml --output plugins/index.json

```
relicta plugin registry generate-index [flags]
```

### Options

```
  -h, --help            help for generate-index
      --input string    registry.yaml source path (default "plugins/registry.yaml")
      --output string   index.json output path (default "plugins/index.json")
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

* [relicta plugin registry](relicta_plugin_registry.md)	 - Manage plugin registries

