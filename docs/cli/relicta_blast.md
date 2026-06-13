## relicta blast

Analyze blast radius of changes in a monorepo

### Synopsis

Analyze the blast radius of changes between two git references.

This command examines changes in a monorepo and identifies:
- Directly affected packages (where files changed)
- Transitively affected packages (dependencies of changed packages)
- Risk assessment for each affected package
- Suggested release types for each package

Example:
  relicta blast --from v1.0.0 --to HEAD
  relicta blast --from HEAD~10 --verbose
  relicta blast --package-paths "packages/*,services/*"

```
relicta blast [flags]
```

### Options

```
  -e, --exclude strings         paths to exclude from analysis
  -f, --from string             starting reference (default: latest tag)
  -g, --graph                   generate dependency graph
  -h, --help                    help for blast
  -D, --include-docs            include documentation files in analysis
  -T, --include-tests           include test files in analysis
  -p, --package-paths strings   custom package paths (glob patterns)
  -t, --to string               ending reference (default "HEAD")
      --transitive              include transitive dependency impacts (default true)
  -V, --verbose                 show verbose output with file details
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
```

### SEE ALSO

* [relicta](relicta.md)	 - The governance layer for software change

