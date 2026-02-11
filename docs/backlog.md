# Backlog

## Completed

### Policy Decision Explainability — DONE

Add first-class explainability for policy evaluation so teams can see which rules/conditions drove a decision, with CLI output suitable for CI and human review.

- `--explain` flag on `policy test`
- `--explain-mode all|matched` controls verbosity
- `RuleTrace` + `ConditionTrace` structs capture per-rule/per-condition evaluation
- Output shows field, operator, expected vs actual, matched status

### Matrix Tagging and Sharding — DONE

Extend policy matrix scenarios with tags and add filtering/sharding flags so large policy suites can run in parallel CI jobs with deterministic subsets.

- Tags field on matrix scenarios (`tags: [low-risk, seed]`)
- `--scenario-tag` / `--exclude-scenario-tag` for filtering
- `--shard-index` / `--shard-total` for deterministic FNV-1a sharding
- `--list-scenarios` to preview selected scenarios

### Policy Test CI Artifacts — DONE

Add policy test report exporters (JUnit and compact JSON summary) to integrate matrix outcomes and assertion diffs into CI dashboards and test reporting.

- `--junit-out path` writes JUnit XML with assertion mismatches as failures
- `--summary-out path` writes compact JSON summary (totals, blocked, decisions)
- Both integrate into CI dashboards

### What-If Policy Comparison — DONE

Allow comparing decisions between two policy sets (current vs candidate) across the same matrix to surface governance impact before rollout.

- `--baseline-file` / `--baseline-dir` + `--candidate-file` / `--candidate-dir`
- Per-scenario comparison with strictness ranking
- `--compare-fail-on-stricter` / `--compare-fail-on-looser` threshold flags
- `--compare-max-stricter N` / `--compare-max-looser N` count limits

### Policy Fixture Scaffolding — DONE

Add a command to scaffold policy test fixtures (single input + matrix templates) from existing policies to accelerate adoption and reduce manual setup.

- `relicta policy scaffold` command with `--dir`, `--file`, `--input-out`, `--matrix-out`, `--force`, `--max-rule-scenarios`
- Generates low-risk and high-risk seed scenarios plus per-rule derived scenarios
- Supports JSON and YAML output formats
