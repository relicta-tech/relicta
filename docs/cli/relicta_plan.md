## relicta plan

Analyze changes and plan the next release

### Synopsis

Analyze commits since the last release and suggest a version bump.

This command examines your commit history using conventional commits
to determine what type of release is needed (major, minor, or patch).

Plans also report a warning when the repository has a higher version tag that
is not in the target commit history. This can mean the checkout is stale or is
an intentional maintenance line. Review the branch and tag history before
releasing; the plan uses the newest version tag reachable from the target.
An ordinary plan is not saved while this warning is present unless you pass
`--allow-diverged-version-line`. Use that flag only after confirming the target
branch is the intended maintenance line; the acknowledgment is stored in the
release plan history. Dry-runs remain available without the flag.

```
relicta plan [flags]
```

### Options

```
  -a, --all                    show all commits including non-conventional
      --allow-diverged-version-line acknowledge a higher version tag outside the target history and allow saving this release plan
      --analyze                analyze commit classifications and stop
      --chronos-threads int    max concurrent Chronos ingest requests (overrides config)
      --force                  discard an existing release run for these same commits, including its approval
  -f, --from string            starting reference (default: latest tag)
  -h, --help                   help for plan
      --min-confidence float   minimum confidence to accept classifications
  -m, --minimal                show minimal text output (ignored with --json)
      --no-ai                  disable AI classification
  -r, --review                 review and adjust commit classifications before planning
      --skip-cognitive         skip Mnemos & Chronos cognitive backends
  -t, --to string              ending reference (default "HEAD")
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

