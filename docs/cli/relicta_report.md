## relicta report

Generate compliance reports

### Synopsis

Generate compliance and governance reports from release history.

Supported report types:

  dora                    DORA metrics (Deployment Frequency, Lead Time,
                          MTTR, Change Failure Rate)
  soc2                    SOC 2 change management evidence (change log,
                          approval evidence, risk assessments, incidents)
  summary                 General governance summary with risk distribution,
                          approval breakdown, and actor activity
  eu-ai-act-article-12    EU AI Act Article 12 record-keeping log bundle —
                          one entry per governance decision with use period,
                          reference data, input data, verifiers, and audit
                          chain anchors. 6-month retention enforced.
  eu-ai-act-annex-iv      EU AI Act Annex IV technical documentation —
                          eight-section system documentation: general
                          description, detailed elements, monitoring/control,
                          risk management, lifecycle changes, harmonized
                          standards, conformity declaration scaffold, and
                          post-market monitoring. 10-year retention enforced.

Examples:

  relicta report --type dora --period 2026-Q1
  relicta report --type soc2 --period "2026-03-01:2026-03-31" --format json
  relicta report --type summary --period 2026-Q1 -o report.md
  relicta report --type eu-ai-act-article-12 --period 2026-Q1 --format jsonl -o art12.jsonl
  relicta report --type eu-ai-act-article-12 --period 2026-Q1 --format csv -o art12.csv
  relicta report --type eu-ai-act-annex-iv --period 2026-Q1 -o annex-iv.md

```
relicta report [flags]
```

### Options

```
      --format string   output format: markdown, json, jsonl, csv (jsonl/csv require --type eu-ai-act-article-12) (default "markdown")
  -h, --help            help for report
  -o, --output string   write report to file instead of stdout
      --period string   time period (e.g. 2026-Q1 or 2026-03-01:2026-03-31)
      --repo string     repository filter (default: current repository)
      --type string     report type: dora, soc2, summary, eu-ai-act-article-12, eu-ai-act-annex-iv (default "summary")
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

