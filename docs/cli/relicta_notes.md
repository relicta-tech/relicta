## relicta notes

Generate changelog and release notes

### Synopsis

Generate changelog entries and release notes for the current release.

This command creates human-readable release documentation from your
commit history, optionally using AI to enhance the content.

```
relicta notes [flags]
```

### Options

```
      --ai                use AI to generate notes (requires an AI provider key: OPENAI_API_KEY, ANTHROPIC_API_KEY, GEMINI_API_KEY, or OLLAMA_HOST)
  -a, --audience string   target audience (developers, users, public, stakeholders)
      --emoji             include emojis in output
  -h, --help              help for notes
  -l, --language string   output language (default "English")
  -o, --output string     output file (default: stdout)
  -t, --tone string       AI tone (technical, friendly, professional, marketing)
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

