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
  eu-ai-act-article-12    Release-governance evidence mapped to EU AI Act
                          Article 12 concepts — one entry per governance
                          decision (release records fill gaps) with
                          governance/release timestamps, change metadata,
                          and CGP protocol version. Approver identities and
                          audit-chain hashes are not included, and chain
                          integrity is not checked by this report. It is
                          not an operational log of each AI system use and
                          cannot by itself establish Article 12 logging
                          compliance. It shows a suggested date six months
                          after the report period ends. Articles 19(1) and
                          26(6) set role-specific minimum periods for certain
                          logs, subject to intended purpose and other law;
                          confirm actual duties. Relicta does not enforce
                          storage retention or deletion.
  eu-ai-act-annex-iv      Draft release-governance evidence mapped to EU AI
                          Act Annex IV. It is not complete target-system
                          technical documentation or a conformity assessment;
                          provider-completed details are marked TODO. Article
                          18(1) sets a provider documentation period of ten
                          years from placing on the market or putting into
                          service. Relicta lacks that event date and does not
                          calculate a deadline or enforce storage retention/
                          deletion.

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

