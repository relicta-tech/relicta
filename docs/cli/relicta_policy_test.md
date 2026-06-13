## relicta policy test

Test policies with simulated inputs

### Synopsis

Evaluate loaded policies against simulated release input.

Inputs can be passed via flags, with --input JSON file, or with --matrix
to evaluate multiple scenarios from one file.
Flag values override --input file values when both are provided.

```
relicta policy test [flags]
```

### Options

```
      --actor-id string                        actor identifier (default "human:policy-test")
      --actor-type string                      actor type: human, agent, ci, system (default "human")
      --assert-expected                        for matrix mode: fail when scenario expectations do not match actual decision
      --baseline-dir string                    for compare mode: baseline policy directory
      --baseline-file string                   for compare mode: baseline policy file
      --branch string                          branch name (default "main")
      --breaking int                           breaking change count
      --bump-type string                       suggested bump type: major, minor, patch (default "patch")
      --candidate-dir string                   for compare mode: candidate policy directory
      --candidate-file string                  for compare mode: candidate policy file
      --compare-fail-on-looser                 for compare mode: fail if candidate policy is looser in any scenario
      --compare-fail-on-stricter               for compare mode: fail if candidate policy is stricter in any scenario
      --compare-max-looser int                 for compare mode: maximum allowed looser scenarios (-1 disables) (default -1)
      --compare-max-stricter int               for compare mode: maximum allowed stricter scenarios (-1 disables) (default -1)
      --dependencies int                       dependency change count
  -d, --dir string                             directory containing policy files
      --exclude-scenario stringArray           matrix scenario name to exclude (repeatable)
      --exclude-scenario-pattern stringArray   matrix scenario glob pattern to exclude (repeatable), e.g. 'flaky-*'
      --exclude-scenario-tag stringArray       matrix scenario tag to exclude (repeatable)
      --explain                                include per-rule and per-condition evaluation trace in output
      --explain-mode string                    trace verbosity for --explain: all, matched (default "all")
      --fail-on-blocked                        exit with error if evaluation result is blocked
      --features int                           feature change count
  -f, --file string                            single policy file to test
      --files-changed int                      changed files count
      --fixes int                              fix change count
  -h, --help                                   help for test
      --input string                           input file with test values (.json, .yaml, .yml), or '-' for stdin
      --junit-out string                       for matrix mode: write JUnit XML report to file
      --lines-changed int                      changed lines count
      --list-scenarios                         with --matrix: list scenario names and exit
      --matrix string                          matrix file with multiple scenarios (.json, .yaml, .yml), or '-' for stdin
      --repository string                      repository identifier (default "local/repo")
      --require-approved                       exit with error unless decision is approved
      --risk-score float                       risk score to evaluate (0.0-1.0) (default 0.3)
      --scenario stringArray                   matrix scenario name to run (repeatable)
      --scenario-pattern stringArray           matrix scenario glob pattern to run (repeatable), e.g. 'high-*'
      --scenario-tag stringArray               matrix scenario tag to run (repeatable)
      --security int                           security change count
      --shard-index int                        matrix shard index (1-based, requires --shard-total)
      --shard-total int                        matrix shard count (requires --shard-index)
      --summary                                for matrix mode: include aggregate result summary
      --summary-out string                     for matrix mode: write compact JSON summary report to file
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

* [relicta policy](relicta_policy.md)	 - Manage governance policies

