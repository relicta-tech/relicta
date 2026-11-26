## relicta policy

Manage governance policies

### Synopsis

Manage CGP (Change Governance Protocol) policies.

Policies define rules for evaluating release changes, determining
risk levels, and requiring approvals based on configurable conditions.

Examples:
  # Write a starting policy (governance is on by default; this customizes it)
  relicta policy init

  # See the policy templates included in the binary
  relicta policy init --list

  # Validate all policies in the default directory
  relicta policy validate

  # Validate policies in a specific directory
  relicta policy validate --dir .relicta/policies

  # Validate a specific policy file
  relicta policy validate --file security.policy

  # List all loaded policies
  relicta policy list

  # Test policies with simulated input
  relicta policy test --risk-score 0.85 --bump-type major

### Options

```
  -h, --help   help for policy
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
* [relicta policy fields](relicta_policy_fields.md)	 - List the fields a policy condition can test
* [relicta policy init](relicta_policy_init.md)	 - Write a starting policy
* [relicta policy list](relicta_policy_list.md)	 - List loaded policies
* [relicta policy scaffold](relicta_policy_scaffold.md)	 - Scaffold policy test fixtures
* [relicta policy templates](relicta_policy_templates.md)	 - List the built-in policy templates
* [relicta policy test](relicta_policy_test.md)	 - Test policies with simulated inputs
* [relicta policy validate](relicta_policy_validate.md)	 - Validate policy files

