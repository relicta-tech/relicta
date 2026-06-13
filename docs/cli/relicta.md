## relicta

The governance layer for software change

### Synopsis

Relicta is the governance layer for software change.

As AI agents and CI systems generate more code, deciding what should ship
becomes the hardest problem. Relicta governs change — before it reaches
production.

The Change Governance Protocol (CGP):
  • Risk assessment — Analyze blast radius and impact of every change
  • Audit trails — Complete history of approvals and decisions
  • Approval workflows — Gate releases with configurable policies
  • AI-powered insights — Intelligent release notes and risk analysis

Today, it's a production-ready CLI for semantic versioning, changelogs,
and release automation. Tomorrow, it's the decision layer for risk-aware
releases in an AI-driven world.

Get started with 'relicta init' to set up your project.

### Options

```
      --allow-untrusted-plugins   load plugins on best-effort sandbox platforms; review 'relicta plugin sandbox-status' first
      --ci                        CI/CD mode: auto-approve, JSON output, non-interactive
  -c, --config string             config file (default: .relicta.yaml)
      --dry-run                   simulate actions without making changes
  -h, --help                      help for relicta
      --json                      output results as JSON
      --log string                alias for --log-level
      --log-level string          log level (debug, info, warn, error) (default "info")
      --model string              AI model to use (format: provider/model, e.g., ollama/llama3.2, openai/gpt-4, anthropic/claude-sonnet-4, local/mistral)
      --no-color                  disable colored output
      --redact                    redact secrets and API keys from output (auto-enabled in CI mode)
  -v, --verbose                   enable verbose output
```

### SEE ALSO

* [relicta analytics](relicta_analytics.md)	 - Show governance analytics (risk trends, decisions, team metrics)
* [relicta approve](relicta_approve.md)	 - Review and approve the release
* [relicta blast](relicta_blast.md)	 - Analyze blast radius of changes in a monorepo
* [relicta bump](relicta_bump.md)	 - Calculate and apply a version bump
* [relicta cancel](relicta_cancel.md)	 - Cancel the current release
* [relicta clean](relicta_clean.md)	 - Clean up old release runs
* [relicta communicate](relicta_communicate.md)	 - Generate audience-specific release narratives
* [relicta demo](relicta_demo.md)	 - Manage the local Docker demo environment
* [relicta eval](relicta_eval.md)	 - AI evaluation harness (model regression gate)
* [relicta evaluate](relicta_evaluate.md)	 - Evaluate release risk and governance decision
* [relicta group](relicta_group.md)	 - Multi-repository governance commands
* [relicta health](relicta_health.md)	 - Check the health of relicta and its dependencies
* [relicta history](relicta_history.md)	 - View release history and CGP metrics
* [relicta init](relicta_init.md)	 - Initialize a new relicta configuration
* [relicta integrations](relicta_integrations.md)	 - Third-party integrations (Vanta, Drata)
* [relicta mcp](relicta_mcp.md)	 - Model Context Protocol (MCP) server commands
* [relicta metrics](relicta_metrics.md)	 - Start a metrics server for monitoring
* [relicta notes](relicta_notes.md)	 - Generate changelog and release notes
* [relicta plan](relicta_plan.md)	 - Analyze changes and plan the next release
* [relicta plugin](relicta_plugin.md)	 - Manage Relicta plugins
* [relicta policy](relicta_policy.md)	 - Manage governance policies
* [relicta promote](relicta_promote.md)	 - Promote a release from one channel to another
* [relicta publish](relicta_publish.md)	 - Execute the release
* [relicta release](relicta_release.md)	 - Run the complete release workflow
* [relicta report](relicta_report.md)	 - Generate compliance reports
* [relicta reset](relicta_reset.md)	 - Reset a failed or canceled release
* [relicta rollback](relicta_rollback.md)	 - Roll back to a previous release version
* [relicta serve](relicta_serve.md)	 - Start the dashboard web server
* [relicta server](relicta_server.md)	 - Start the dashboard API server with deployment mode control
* [relicta status](relicta_status.md)	 - Show the current release status
* [relicta verify](relicta_verify.md)	 - Verify governance attestation for a release
* [relicta version](relicta_version.md)	 - Print version information

