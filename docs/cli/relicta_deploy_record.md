## relicta deploy record

Record a deployment of a version to an environment

```
relicta deploy record [flags]
```

### Options

```
      --duration string     how long the deployment took (e.g. 4m12s)
      --env string          environment the version reached (required)
  -h, --help                help for record
      --outcome string      succeeded, failed or rolled_back (default "succeeded")
      --provenance string   what observed this: reported, inferred or manual (default "reported")
      --ref string          reference back to the deploying system (CI run URL, rollout ID)
      --release-id string   relicta release run this deployment came from
      --version string      version that was deployed (required)
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

* [relicta deploy](relicta_deploy.md)	 - Record and inspect deployments of released versions

